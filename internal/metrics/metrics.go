package metrics

import "time"

type ActionMetrics struct {
	ID         string    `json:"id"`
	Action     string    `json:"action"`
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	DurationMs int64     `json:"duration_ms"`
	Success    bool      `json:"success"`
	Err        string    `json:"err,omitempty"`
}

type StageMetrics struct {
	Stage      int             `json:"stage"`
	Start      time.Time       `json:"start"`
	End        time.Time       `json:"end"`
	DurationMs int64           `json:"duration_ms"`
	Actions    []ActionMetrics `json:"actions"`
}

type MissionMetrics struct {
	MissionID  string         `json:"mission_id"`
	Start      time.Time      `json:"start"`
	End        time.Time      `json:"end"`
	DurationMs int64          `json:"duration_ms"`
	Succeeded  bool           `json:"succeeded"`
	Stages     []StageMetrics `json:"stages"`
}

// Compute derived fields for a stage.
func (s *StageMetrics) Finalize() {
	s.DurationMs = s.End.Sub(s.Start).Milliseconds()
}
