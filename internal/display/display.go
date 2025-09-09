package display

import (
	"a-a/internal/parser"
	"fmt"
	"strings"
)

const maxPayloadValueLength = 100

func FormatPlan(plan *parser.ExecutionPlan) string {
	var sb strings.Builder
	sb.WriteString("Proposed execution plan:\n")
	sb.WriteString("--------------------------------------------------\n")

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
	sb.WriteString("--------------------------------------------------")
	return sb.String()
}

func formatValueForDisplay(value any) string {
	s := fmt.Sprintf("%v", value)
	s = strings.ReplaceAll(s, "\n", "\\n")

	if len(s) > maxPayloadValueLength {
		return s[:maxPayloadValueLength] + "..."
	}
	return s
}
