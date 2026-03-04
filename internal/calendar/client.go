package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gcal-auto-add/internal/models"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googlecalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Service はGoogle Calendarサービスのラッパー
type Service struct {
	svc *googlecalendar.Service
}

// NewService はOAuth2認証を行いGoogle Calendarサービスを返す
func NewService() (*Service, error) {
	credFile := os.Getenv("GOOGLE_CREDENTIALS_FILE")
	if credFile == "" {
		credFile = "credentials.json"
	}

	b, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf(
			"credentials.jsonが読み込めません: %w\n"+
				"Google Cloud Console > APIとサービス > 認証情報 から\n"+
				"OAuthクライアントID（デスクトップアプリ）をダウンロードして\n"+
				"credentials.json として保存してください", err)
	}

	config, err := google.ConfigFromJSON(b, googlecalendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("OAuth2設定エラー: %w", err)
	}

	tokenFile := tokenFilePath()
	token, err := loadToken(tokenFile)
	if err != nil {
		fmt.Println("初回認証が必要です。ブラウザでGoogleにログインしてください。")
		token, err = getTokenFromWeb(config)
		if err != nil {
			return nil, fmt.Errorf("OAuth2認証エラー: %w", err)
		}
		if saveErr := saveToken(tokenFile, token); saveErr != nil {
			fmt.Printf("警告: トークンの保存に失敗しました: %v\n", saveErr)
		} else {
			fmt.Printf("認証トークンを保存しました: %s\n", tokenFile)
		}
	}

	ctx := context.Background()
	tokenSource := config.TokenSource(ctx, token)
	httpClient := oauth2.NewClient(ctx, tokenSource)

	svc, err := googlecalendar.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("Google Calendarサービス作成エラー: %w", err)
	}

	return &Service{svc: svc}, nil
}

// RegisterEvent はGoogle CalendarへイベントをAPI経由で登録する
func (s *Service) RegisterEvent(event models.GeminiEvent) error {
	calEvent := &googlecalendar.Event{
		Summary:     event.Title,
		Location:    event.Location,
		Description: buildDescription(event),
	}

	if event.IsAllDay {
		calEvent.Start = &googlecalendar.EventDateTime{
			Date: event.Start,
		}
		calEvent.End = &googlecalendar.EventDateTime{
			Date: event.End,
		}
	} else {
		calEvent.Start = &googlecalendar.EventDateTime{
			DateTime: event.Start,
			TimeZone: "Asia/Tokyo",
		}
		calEvent.End = &googlecalendar.EventDateTime{
			DateTime: event.End,
			TimeZone: "Asia/Tokyo",
		}
	}

	if event.Recurrence != "" {
		calEvent.Recurrence = []string{event.Recurrence}
	}

	_, err := s.svc.Events.Insert("primary", calEvent).Do()
	if err != nil {
		return fmt.Errorf("イベント登録エラー: %w", err)
	}
	return nil
}

func buildDescription(event models.GeminiEvent) string {
	var parts []string
	if event.Description != "" {
		parts = append(parts, event.Description)
	}
	for _, u := range event.URLs {
		parts = append(parts, u)
	}
	return strings.Join(parts, "\n")
}

func tokenFilePath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "gcal-auto-add")
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, "token.json")
}

func loadToken(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok oauth2.Token
	if err := json.NewDecoder(f).Decode(&tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func saveToken(file string, token *oauth2.Token) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	config.RedirectURL = "http://localhost:8080/callback"
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	fmt.Printf("\n以下のURLをブラウザで開いてください:\n%s\n\n", authURL)
	openBrowser(authURL)

	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "認証コードが取得できませんでした", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "<html><body><h1>認証完了！</h1><p>このタブを閉じてCLIに戻ってください。</p></body></html>")
		codeCh <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "コールバックサーバーエラー: %v\n", err)
		}
	}()

	fmt.Println("Googleログインを完了させてください...")

	select {
	case code := <-codeCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)

		token, err := config.Exchange(context.Background(), code)
		if err != nil {
			return nil, fmt.Errorf("認証コード交換エラー: %w", err)
		}
		fmt.Println("認証成功！")
		return token, nil

	case <-time.After(5 * time.Minute):
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
		return nil, fmt.Errorf("認証タイムアウト（5分経過）")
	}
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return
	}
	exec.Command(cmd, args...).Start()
}
