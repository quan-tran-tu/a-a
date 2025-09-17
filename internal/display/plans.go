// internal/display/plans.go
package display

import (
	"fmt"
	"strings"

	"a-a/internal/parser"
	"a-a/internal/supervisor"
)

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
