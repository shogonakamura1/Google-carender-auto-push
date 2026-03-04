package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gcal-auto-add/internal/models"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// ParseEvents はGemini APIを使って予定テキストを解析し、イベント一覧を返す
func ParseEvents(apiKey, text string) ([]models.GeminiEvent, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("Geminiクライアント作成エラー: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-3.1-flash-lite-preview")
	model.SetTemperature(0)

	prompt := buildPrompt(text, time.Now())

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("Gemini APIエラー: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("Geminiからの応答が空です")
	}

	var responseText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if t, ok := part.(genai.Text); ok {
			responseText += string(t)
		}
	}

	jsonStr := extractJSON(responseText)
	if jsonStr == "" {
		return nil, fmt.Errorf("レスポンスからJSONが抽出できません:\n%s", responseText)
	}

	var events []models.GeminiEvent
	if err := json.Unmarshal([]byte(jsonStr), &events); err != nil {
		return nil, fmt.Errorf("JSON解析エラー: %w\nレスポンス: %s", err, responseText)
	}

	return events, nil
}

func buildPrompt(text string, now time.Time) string {
	return fmt.Sprintf(`あなたは予定テキストをGoogle Calendarイベントに変換するアシスタントです。
以下の予定テキストを解析し、イベントのJSON配列のみを返してください。説明文や補足は不要です。

## 今日の日付
%s

## 解析ルール

### 年の補完
- 年が明示されていない場合、最初に出現する日付に現在年（%d年）を適用する
- 月が基準より小さく戻る場合は翌年（%d年）と判断する
- 例: 4月→5月→10月→12月→1月 の順に現れる場合、1月は翌年

### 時刻の補完
- 開始時刻のみの場合、終了時刻 = 開始時刻 + 1時間
- 例: 13:00開始 → 終了は14:00

### 終日イベント判定
- 時刻情報が存在しない場合は終日イベント（is_all_day: true）とする
- 日付範囲（4/10-4/12、4/10〜4/12など）は終日イベントとして登録する

### 終日イベントの日付範囲
- Google Calendarでは終日イベントの終了日は「翌日」を指定する
- 例: 4月10日の終日イベント → start: "2026-04-10", end: "2026-04-11"
- 例: 4/10〜4/12の範囲 → start: "2026-04-10", end: "2026-04-13"（4/12の翌日）

### 繰り返しイベント
- 「毎週」「各週」「every week」が含まれる場合、recurrenceに "RRULE:FREQ=WEEKLY" を設定する
- それ以外は繰り返しなし（recurrenceフィールドは省略）

### URL処理
- テキスト内のURLはurls配列に全て追加する
- URLはdescriptionにも含める

### インデント処理
- インデントは日付グループ・イベントグループを示す
- インデントを考慮して同日の複数イベントを正しく解析する

### 信頼度（confidence）
- 確信を持って予定と判断できる: 0.9以上
- 予定らしいが曖昧: 0.7〜0.89
- 確信が持てない（予定でない可能性がある）: 0.7未満

## 出力形式（JSON配列のみ）

` + "```json" + `
[
  {
    "title": "イベントタイトル",
    "start": "2026-04-10T13:00:00",
    "end": "2026-04-10T14:00:00",
    "is_all_day": false,
    "location": "場所（なければ空文字）",
    "description": "説明（なければ空文字）",
    "urls": ["https://example.com"],
    "source_text": "元のテキスト",
    "confidence": 0.95,
    "recurrence": "RRULE:FREQ=WEEKLY"
  }
]
` + "```" + `

終日イベントの場合はstartとendをdate形式（"YYYY-MM-DD"）で指定する。
時刻ありイベントの場合はstartとendをdatetime形式（"YYYY-MM-DDTHH:MM:SS"）で指定する。

## 予定テキスト

%s`, now.Format("2006年01月02日（Monday）"), now.Year(), now.Year()+1, text)
}

func extractJSON(text string) string {
	// ```json ... ``` ブロックから抽出
	if idx := strings.Index(text, "```json"); idx != -1 {
		start := idx + 7
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	// ``` ... ``` ブロックから抽出
	if idx := strings.Index(text, "```"); idx != -1 {
		start := idx + 3
		end := strings.Index(text[start:], "```")
		if end != -1 {
			candidate := strings.TrimSpace(text[start : start+end])
			if strings.HasPrefix(candidate, "[") {
				return candidate
			}
		}
	}
	// JSON配列を直接探す
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start != -1 && end != -1 && end > start {
		return strings.TrimSpace(text[start : end+1])
	}
	return ""
}
