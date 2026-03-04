package gemini

import (
	"strings"
	"testing"
	"time"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // "" means no JSON expected
	}{
		{
			name: "json fenced block",
			input: "以下の通りです。\n```json\n[{\"title\":\"会議\"}]\n```\n",
			want:  `[{"title":"会議"}]`,
		},
		{
			name: "plain fenced block with array",
			input: "```\n[{\"title\":\"MTG\"}]\n```",
			want:  `[{"title":"MTG"}]`,
		},
		{
			name: "raw JSON array in text",
			input: `結果: [{"title":"ランチ"}] でした。`,
			want:  `[{"title":"ランチ"}]`,
		},
		{
			name:  "no JSON",
			input: "予定はありません。",
			want:  "",
		},
		{
			name: "json fenced block takes priority over raw array",
			input: "dummy [x] text\n```json\n[{\"title\":\"正解\"}]\n```\n",
			want:  `[{"title":"正解"}]`,
		},
		{
			name: "empty json array",
			input: "```json\n[]\n```",
			want:  `[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	text := "4月10日 13:00 会議"

	prompt := buildPrompt(text, now)

	checks := []struct {
		desc    string
		contain string
	}{
		{"contains input text", text},
		{"contains current year", "2026"},
		{"contains next year for rollover", "2027"},
		{"contains today's date header", "今日の日付"},
		{"contains output format instruction", "JSON配列のみ"},
		{"contains confidence rule", "confidence"},
		{"contains recurrence rule", "RRULE:FREQ=WEEKLY"},
		{"contains is_all_day field", "is_all_day"},
	}

	for _, c := range checks {
		t.Run(c.desc, func(t *testing.T) {
			if !strings.Contains(prompt, c.contain) {
				t.Errorf("buildPrompt() does not contain %q", c.contain)
			}
		})
	}
}
