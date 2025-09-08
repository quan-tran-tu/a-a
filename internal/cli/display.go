package cli

import (
	"fmt"
	"strings"

	"a-a/internal/parser"
)

func PrettyPrintPlan(plan *parser.ExecutionPlan) {
	var sb strings.Builder
	sb.WriteString("EXECUTION PLAN:\n")
	for _, stage := range plan.Plan {
		sb.WriteString(fmt.Sprintf("Stage %d:\n", stage.Stage))
		for _, action := range stage.Actions {
			sb.WriteString(fmt.Sprintf("  - Action: %s (ID: %s)\n", action.Action, action.ID))
			if len(action.Payload) > 0 {
				sb.WriteString("    Payload:\n")
				for key, val := range action.Payload {
					displayValue := formatValueForDisplay(val)
					sb.WriteString(fmt.Sprintf("      %s: %s\n", key, displayValue))
				}
			}
		}
	}
	sb.WriteString("--------------------------------------------------\n")

	fmt.Print(sb.String())
}

func formatValueForDisplay(value any) string {
	s := fmt.Sprintf("%v", value)
	s = strings.ReplaceAll(s, "\n", "\\n") // Keep display on one line

	return s
}
