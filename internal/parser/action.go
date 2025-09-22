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

type PlanMeta struct {
	PlanType    string `json:"plan_type,omitempty"`    // "exploration", "extraction", ...
	Replan      bool   `json:"replan,omitempty"`       // true if this plan is a replan
	HandoffPath string `json:"handoff_path,omitempty"` // path to handoff context file if any
}

type ExecutionPlan struct {
	Meta PlanMeta         `json:"meta,omitempty"`
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
	DefaultTimeoutMs int `json:"default_timeout_ms,omitempty"`
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
	RequiresConfirmation bool     `json:"requires_confirmation"` // true if user confirmation is needed before executing
	RunManualPlans       bool     `json:"run_manual_plans"`      // true if user wants to execute plans from a JSON file
	ManualPlansPath      string   `json:"manual_plans_path"`     // path to the JSON file
	ManualPlanNames      []string `json:"manual_plan_names"`     // names to run (ordered). If empty -> run all
	Cancel               bool     `json:"cancel"`                // true if user asks to stop/abort/kill/cancel
	TargetMissionID      string   `json:"target_mission_id"`     // mission/plan ID if provided
	TargetIsPrevious     bool     `json:"target_is_previous"`    // true for "previous / last / most recent"
}

func GetActionDefinition(actionName string) (ActionDefinition, bool) {
	if registry == nil {
		return ActionDefinition{}, false
	}
	return registry.GetDefinition(actionName)
}
