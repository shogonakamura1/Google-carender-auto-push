package calendar

import (
	"testing"

	"gcal-auto-add/internal/models"
)

func TestBuildDescription(t *testing.T) {
	tests := []struct {
		name  string
		event models.GeminiEvent
		want  string
	}{
		{
			name: "description only",
			event: models.GeminiEvent{
				Description: "定例ミーティング",
			},
			want: "定例ミーティング",
		},
		{
			name: "urls only",
			event: models.GeminiEvent{
				URLs: []string{"https://meet.example.com/abc"},
			},
			want: "https://meet.example.com/abc",
		},
		{
			name: "description and urls",
			event: models.GeminiEvent{
				Description: "スプリントレビュー",
				URLs:        []string{"https://zoom.us/j/123", "https://docs.example.com/notes"},
			},
			want: "スプリントレビュー\nhttps://zoom.us/j/123\nhttps://docs.example.com/notes",
		},
		{
			name:  "empty event",
			event: models.GeminiEvent{},
			want:  "",
		},
		{
			name: "multiple urls no description",
			event: models.GeminiEvent{
				URLs: []string{"https://a.com", "https://b.com"},
			},
			want: "https://a.com\nhttps://b.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDescription(tt.event)
			if got != tt.want {
				t.Errorf("buildDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}
