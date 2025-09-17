package supervisor

import "a-a/internal/metrics"

type MissionResult struct {
	MissionID    string                  `json:"mission_id"`
	OriginalGoal string                  `json:"original_goal"`
	FinalPlan    string                  `json:"final_plan"`
	Error        string                  `json:"error,omitempty"`
	Metrics      *metrics.MissionMetrics `json:"metrics,omitempty"`
}

// Global channel for all mission results.
var ResultChannel = make(chan MissionResult, 100)
