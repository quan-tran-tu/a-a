package supervisor

import "a-a/internal/parser"

const (
	StatusRunning   = "RUNNING"
	StatusSucceeded = "SUCCEEDED"
	StatusFailed    = "FAILED"
)

type Mission struct {
	ID                  string
	OriginalGoal        string
	State               string
	CurrentAttempt      int
	MaxRetries          int
	ConversationHistory []parser.ConversationTurn
}

type ConfirmationRequest struct {
	MissionID string
	Plan      *parser.ExecutionPlan
	Response  chan bool
}

var ConfirmationChannel = make(chan ConfirmationRequest)
