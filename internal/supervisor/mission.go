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
	Plan                *parser.ExecutionPlan
}
