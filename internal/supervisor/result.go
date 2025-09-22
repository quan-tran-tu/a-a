package supervisor

import "a-a/internal/metrics"

type MissionResult struct {
	MissionID    string                  `json:"mission_id"`
	OriginalGoal string                  `json:"original_goal"`
	FinalPlan    string                  `json:"final_plan"`
	Error        string                  `json:"error,omitempty"`
	Metrics      *metrics.MissionMetrics `json:"metrics,omitempty"`
}

type PlanPreview struct {
	MissionID string `json:"mission_id"`
	PlanJSON  string `json:"plan_json"`
}

type PlanApproval struct {
	MissionID string `json:"mission_id"`
	Approved  bool   `json:"approved"`
}

var PlanPreviewChannel = make(chan PlanPreview, 16)
var PlanApprovalChannel = make(chan PlanApproval, 16)

// Global channel for all mission results.
var ResultChannel = make(chan MissionResult, 100)
