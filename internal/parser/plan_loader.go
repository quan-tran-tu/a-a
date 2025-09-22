package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type NamedPlan struct {
	Name string
	Plan *ExecutionPlan
}

var resultsRefRe = regexp.MustCompile(`@results\.([A-Za-z0-9_\-]+)\.`)

// All @results.<id> reference points only to IDs produced by PRIOR stages.
func checkNoIntraStageRefs(v any, seen map[string]struct{}, stageIdx int, actID string) error {
	switch t := v.(type) {
	case map[string]any:
		for _, vv := range t {
			if err := checkNoIntraStageRefs(vv, seen, stageIdx, actID); err != nil {
				return err
			}
		}
	case []any:
		for _, vv := range t {
			if err := checkNoIntraStageRefs(vv, seen, stageIdx, actID); err != nil {
				return err
			}
		}
	case string:
		matches := resultsRefRe.FindAllStringSubmatch(t, -1)
		for _, m := range matches {
			refID := m[1]
			if _, ok := seen[refID]; !ok {
				return fmt.Errorf(
					"stage %d action '%s' references @results.%s, which is not available yet (same or later stage). Move this action to a later stage",
					stageIdx+1, actID, refID,
				)
			}
		}
	}
	return nil
}

func validateStageDependencies(plan *ExecutionPlan) error {
	seen := map[string]struct{}{} // IDs completed in prior stages

	for si, stage := range plan.Plan {
		// Check all actions' payloads in this stage
		for _, act := range stage.Actions {
			if err := checkNoIntraStageRefs(act.Payload, seen, si, act.ID); err != nil {
				return err
			}
		}
		// Mark actions from this stage as available for later stages
		for _, act := range stage.Actions {
			if act.ID != "" {
				seen[act.ID] = struct{}{}
			}
		}
	}
	return nil
}

/*
LoadExecutionPlansFromFile loads one or many plans from a JSON file and always
returns a slice. It supports these shapes:

 1. Multi-plan (preferred):
    {
    "plans": [
    { "name": "alpha", "plan": [ {..stage..}, ... ] },
    { "plan": [ {..stage..}, ... ] },          // name optional
    [ {..stage..}, ... ]                       // an entry can be a bare stages array
    ]
    }

 2. Multi-plan (bare array):
    [
    { "name": "alpha", "plan": [ ... ] },
    { "plan": [ ... ] },
    [ {..stage..}, ... ]
    ]

 3. Single-plan (treated as 1-element list):
    { "plan": [ ... ] }
    [ {..stage..}, ... ]   // bare array of stages at top level

Unnamed plans are auto-named as "manual:<base>#<index>".
*/
func LoadExecutionPlansFromFile(path string) ([]NamedPlan, error) {
	clean := filepath.Clean(path)
	if _, err := os.Stat(clean); err != nil {
		return nil, fmt.Errorf("plans file not found: %s", clean)
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", clean, err)
	}

	// Format 1: object with "plans"
	var obj struct {
		Plans []json.RawMessage `json:"plans"`
	}
	if err := json.Unmarshal(data, &obj); err == nil && len(obj.Plans) > 0 {
		return parsePlanList(obj.Plans, filepath.Base(clean))
	}

	// Format 2: bare array
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		return parsePlanList(arr, filepath.Base(clean))
	}

	// Format 3: single plan -> wrap as one
	np, ok := parseOneTopLevelPlan(data, filepath.Base(clean))
	if ok {
		return []NamedPlan{np}, nil
	}

	return nil, fmt.Errorf("unrecognized plans format in %s", clean)
}

func parsePlanList(items []json.RawMessage, base string) ([]NamedPlan, error) {
	var out []NamedPlan
	for i, raw := range items {
		np, ok := parseOneNamedPlan(raw)
		if !ok || np.Plan == nil || len(np.Plan.Plan) == 0 {
			// Try: entry is a bare array of stages
			var stages []ExecutionStage
			if err := json.Unmarshal(raw, &stages); err == nil && len(stages) > 0 {
				np = NamedPlan{
					Name: fmt.Sprintf("manual:%s#%d", base, i+1),
					Plan: &ExecutionPlan{Plan: stages},
				}
				ok = true
			}
		}
		if !ok {
			return nil, fmt.Errorf("could not parse plan #%d", i+1)
		}
		if strings.TrimSpace(np.Name) == "" {
			np.Name = fmt.Sprintf("manual:%s#%d", base, i+1)
		}
		out = append(out, np)
	}
	return out, nil
}

// parseOneNamedPlan tries {"name":"...", "plan":[...]}, then {"plan":[...]}.
func parseOneNamedPlan(raw json.RawMessage) (NamedPlan, bool) {
	// {"name":"...", "plan":[...]}
	var wrap struct {
		Name string           `json:"name"`
		Plan []ExecutionStage `json:"plan"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && len(wrap.Plan) > 0 {
		return NamedPlan{
			Name: strings.TrimSpace(wrap.Name),
			Plan: &ExecutionPlan{Plan: wrap.Plan},
		}, true
	}

	// {"plan":[...]}
	var ep ExecutionPlan
	if err := json.Unmarshal(raw, &ep); err == nil && len(ep.Plan) > 0 {
		return NamedPlan{
			Name: "",
			Plan: &ep,
		}, true
	}

	return NamedPlan{}, false
}

// parseOneTopLevelPlan handles a single-plan top-level document:
//
//	{"plan":[...]}  OR  [ {..stage..}, ... ]
func parseOneTopLevelPlan(data []byte, base string) (NamedPlan, bool) {
	// {"plan":[...]}
	var ep ExecutionPlan
	if err := json.Unmarshal(data, &ep); err == nil && len(ep.Plan) > 0 {
		return NamedPlan{
			Name: "manual:" + base,
			Plan: &ep,
		}, true
	}
	// [ {..stage..}, ... ]
	var stages []ExecutionStage
	if err := json.Unmarshal(data, &stages); err == nil && len(stages) > 0 {
		return NamedPlan{
			Name: "manual:" + base,
			Plan: &ExecutionPlan{Plan: stages},
		}, true
	}
	return NamedPlan{}, false
}

// SelectPlansByNames returns plans matching the given names (case-insensitive).
func SelectPlansByNames(plans []NamedPlan, names []string) ([]NamedPlan, []string, error) {
	if len(names) == 0 {
		return plans, nil, nil
	}

	var selected []NamedPlan
	var missing []string

	for _, want := range names {
		w := strings.TrimSpace(want)
		if w == "" {
			continue
		}

		found := false
		for i := range plans {
			if strings.EqualFold(plans[i].Name, w) {
				selected = append(selected, plans[i])
				found = true
				break
			}
		}

		if !found {
			missing = append(missing, want)
		}
	}

	return selected, missing, nil
}

// ValidatePlan checks actions against the loaded actions registry.
func ValidatePlan(plan *ExecutionPlan) error {
	if registry == nil {
		return fmt.Errorf("action registry not loaded")
	}
	for _, stage := range plan.Plan {
		for _, action := range stage.Actions {
			if err := registry.ValidateAction(&action); err != nil {
				return err
			}
		}
	}
	return validateStageDependencies(plan)
}
