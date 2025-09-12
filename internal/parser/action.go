package parser

type Action struct {
	ID      string         `json:"id"`
	Action  string         `json:"action"`
	Payload map[string]any `json:"payload"`
}

type ExecutionStage struct {
	Stage   int      `json:"stage"`
	Actions []Action `json:"actions"`
}

type ExecutionPlan struct {
	Plan []ExecutionStage `json:"plan"`
}

type ActionDefinition struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	PayloadSchema struct {
		Required []string `json:"required"`
	} `json:"payload_schema"`
	OutputSchema struct {
		Keys []string `json:"keys"`
	} `json:"output_schema"`
}

type ActionRegistry struct {
	Actions    []ActionDefinition
	actionsMap map[string]ActionDefinition
}

type ConversationTurn struct {
	UserGoal       string `json:"user_goal"`
	AssistantPlan  string `json:"assistant_plan"`
	ExecutionError string `json:"execution_error,omitempty"`
}

type GoalIntent struct {
	RequiresConfirmation bool `json:"requires_confirmation"`
}
