package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"gcal-auto-add/internal/calendar"
	"gcal-auto-add/internal/gemini"
)

func main() {
	fmt.Println("========================================")
	fmt.Println(" Google Calendar 自動予定登録ツール")
	fmt.Println("========================================")
	fmt.Println("予定テキストを貼り付けてください。")
	fmt.Println("入力終了は Ctrl+D (Mac/Linux) です。")
	fmt.Println("----------------------------------------")

	input := readStdin()
	if strings.TrimSpace(input) == "" {
		fmt.Println("入力がありません。終了します。")
		os.Exit(0)
	}

	fmt.Printf("\n--- 入力テキスト (%d文字) ---\n%s\n---\n\n", len(input), input)

	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		geminiAPIKey = os.Getenv("GEMINI_API")
	}
	if geminiAPIKey == "" {
		fmt.Fprintln(os.Stderr, "エラー: GEMINI_API_KEY または GEMINI_API 環境変数が設定されていません")
		fmt.Fprintln(os.Stderr, "  export GEMINI_API_KEY='your-api-key'")
		os.Exit(1)
	}

	fmt.Println("Geminiで解析中...")
	events, err := gemini.ParseEvents(geminiAPIKey, input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gemini解析エラー: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("解析完了: %d件のイベントを検出\n\n", len(events))

	calService, err := calendar.NewService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Google Calendar認証エラー: %v\n", err)
		os.Exit(1)
	}

	successCount := 0
	failCount := 0
	skipCount := 0

	for i, event := range events {
		fmt.Printf("[%d/%d] %s\n", i+1, len(events), event.Title)

		if event.Confidence < 0.7 {
			fmt.Printf("  → スキップ (信頼度低: %.2f)\n", event.Confidence)
			fmt.Printf("    元テキスト: %s\n", event.SourceText)
			skipCount++
			continue
		}

		if err := calService.RegisterEvent(event); err != nil {
			fmt.Fprintf(os.Stderr, "  → 登録失敗: %v\n", err)
			failCount++
			continue
		}

		if event.IsAllDay {
			fmt.Printf("  → 登録成功 [終日] %s\n", event.Start)
		} else {
			fmt.Printf("  → 登録成功 %s 〜 %s\n", event.Start, event.End)
		}
		successCount++
	}

	fmt.Println("\n========================================")
	fmt.Printf(" 登録成功:   %d件\n", successCount)
	fmt.Printf(" スキップ:   %d件 (信頼度低)\n", skipCount)
	fmt.Printf(" 登録失敗:   %d件\n", failCount)
	fmt.Println("========================================")
}

func readStdin() string {
	reader := bufio.NewReader(os.Stdin)
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			lines = append(lines, strings.TrimRight(line, "\n\r"))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "入力読み取りエラー: %v\n", err)
			os.Exit(1)
		}
	}
	return strings.Join(lines, "\n")
}
