package models

// GeminiEvent はGeminiが解析した予定イベントを表す
type GeminiEvent struct {
	Title       string   `json:"title"`
	Start       string   `json:"start"`      // "2026-04-10" (終日) or "2026-04-10T13:00:00" (時刻あり)
	End         string   `json:"end"`        // "2026-04-11" (終日、翌日) or "2026-04-10T14:00:00"
	IsAllDay    bool     `json:"is_all_day"`
	Location    string   `json:"location"`
	Description string   `json:"description"`
	URLs        []string `json:"urls"`
	SourceText  string   `json:"source_text"`
	Confidence  float64  `json:"confidence"`
	Recurrence  string   `json:"recurrence,omitempty"` // e.g. "RRULE:FREQ=WEEKLY"
}
