package display

import (
	"fmt"
	"strings"

	"a-a/internal/metrics"
)

func FormatMissionMetrics(mm *metrics.MissionMetrics) string {
	if mm == nil {
		return "No metrics available."
	}
	var sb strings.Builder
	sb.WriteString("Execution metrics:\n")
	sb.WriteString(fmt.Sprintf("- Total: %d ms  (success=%v)\n", mm.DurationMs, mm.Succeeded))
	for _, s := range mm.Stages {
		sb.WriteString(fmt.Sprintf("  Stage %d: %d ms\n",
			s.Stage, s.DurationMs))
		for _, a := range s.Actions {
			status := "ok"
			if !a.Success {
				status = "err"
			}
			sb.WriteString(fmt.Sprintf("    • %-12s %-22s %5d ms  [%s]\n",
				a.ID, "("+a.Action+")", a.DurationMs, status))
		}
	}
	return sb.String()
}
