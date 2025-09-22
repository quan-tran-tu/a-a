package supervisor

import (
	"sync"

	"a-a/internal/parser"
)

const (
	StatusRunning   = "RUNNING"
	StatusSucceeded = "SUCCEEDED"
	StatusFailed    = "FAILED"
	StatusCancelled = "CANCELLED"
)

type Mission struct {
	ID                  string
	OriginalGoal        string
	State               string
	CurrentAttempt      int
	MaxRetries          int
	ConversationHistory []parser.ConversationTurn
	Plan                *parser.ExecutionPlan
	RequireConfirm      bool
	ScratchDir          string
	Evidence            string
	Results             map[string]map[string]any
	ResultsMu           sync.Mutex
	LastStage           int
}
