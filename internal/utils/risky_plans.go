package utils

import "a-a/internal/parser"

// Define the risky actions once
var riskyActions = map[string]struct{}{
	"system.execute_shell": {},
	"system.delete_folder": {},
	"system.shutdown":      {},
}

func IsPlanRisky(plan *parser.ExecutionPlan) bool {
	for _, stage := range plan.Plan {
		for _, action := range stage.Actions {
			if _, exists := riskyActions[action.Action]; exists {
				return true
			}
		}
	}
	return false
}
