package display

import (
	"fmt"
	"strings"

	"a-a/internal/parser"
	"a-a/internal/supervisor"
)

const maxPayloadValueLength = 100

func FormatPlansCatalog(file string, plans []parser.NamedPlan) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d mission(s) in %s:\n", len(plans), file))
	for i, p := range plans {
		stages := len(p.Plan.Plan)
		actions := 0
		for _, s := range p.Plan.Plan {
			actions += len(s.Actions)
		}
		risky := supervisor.IsPlanRisky(p.Plan)
		sb.WriteString(fmt.Sprintf("  %2d. %s  (stages=%d, actions=%d, risky=%v)\n",
			i+1, p.Name, stages, actions, risky))
	}
	return sb.String()
}

// stdout plan (truncated)
func FormatPlan(plan *parser.ExecutionPlan) string {
	return formatPlanInternal(plan, maxPayloadValueLength)
}

// full plan (no truncation) — used for logs
func FormatPlanFull(plan *parser.ExecutionPlan) string {
	return formatPlanInternal(plan, -1) // -1 => no limit
}

func formatPlanInternal(plan *parser.ExecutionPlan, limit int) string {
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
					displayValue := formatValueForDisplay(val, limit)
					sb.WriteString(fmt.Sprintf("      %s: %s\n", key, displayValue))
				}
			}
		}
	}
	sb.WriteString("--------------------------------------------------")
	return sb.String()
}

// Limit plan's stdout length (limit < 0 means no limit)
func formatValueForDisplay(value any, limit int) string {
	s := fmt.Sprintf("%v", value)
	s = strings.ReplaceAll(s, "\n", "\\n")
	if limit >= 0 && len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}
