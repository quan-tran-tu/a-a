package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"a-a/internal/llm_client"
)

var registry *ActionRegistry

// Main prompt for generating plan of a mission
// TODO: refine
func buildPlanPrompt(history []ConversationTurn, userGoal string) string {
	var sb strings.Builder

	sb.WriteString("You are an expert AI workflow planner. Convert the user's goal into a STRICT JSON execution plan.\n")
	sb.WriteString("Respond ONLY with JSON. No extra text.\n\n")

	if len(history) > 0 {
		sb.WriteString("CONVERSATION HISTORY (context):\n")
		for _, turn := range history {
			sb.WriteString(fmt.Sprintf("User Goal: \"%s\"\n", turn.UserGoal))
			sb.WriteString(fmt.Sprintf("Previous Assistant Plan: %s\n", turn.AssistantPlan))
			if strings.TrimSpace(turn.ExecutionError) != "" {
				sb.WriteString(fmt.Sprintf("Previous Execution Error: %s\n", turn.ExecutionError))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("OUTPUT JSON SCHEMA:\n")
	sb.WriteString("{\"meta\": {\"plan_type\": \"<string>\", \"replan\": <bool>, \"handoff_path\": \"<string or empty>\"}, \"plan\": [{\"stage\": <int>, \"actions\": [{\"id\": \"<slug>\", \"action\": \"<category.operation>\", \"payload\": {}}]}]}\n\n")

	sb.WriteString("SEMANTICS:\n")
	sb.WriteString("- Actions in the SAME stage run IN PARALLEL.\n")
	sb.WriteString("- Stages run SEQUENTIALLY (stage N completes before stage N+1).\n")
	sb.WriteString("- Later stages may reference earlier outputs via '@results.<action_id>.<key>'.\n")
	sb.WriteString("- RE-PLANNING continues the SAME mission: do NOT redefine previously used action IDs; refer to their outputs instead. If the previous context includes 'PREV_LAST_STAGE: N', start your first stage at N+1.\n\n")

	sb.WriteString(registry.GeneratePromptPart() + "\n")

	sb.WriteString("\nHARD RULES:\n")
	sb.WriteString("1) FIRST PLAN (UNSEEN URL): If the target page structure is unknown, your first plan MUST be an EXPLORATION with meta.plan_type=\"exploration\", meta.replan=true, and EXACTLY 2 stages:\n")
	sb.WriteString("   - Stage 1: one 'web.request' to fetch the seed URL.\n")
	sb.WriteString("   - Stage 2: one 'system.write_file_atomic' that writes a concise JSON evidence file to 'tmp/<name>.json'. The JSON should summarize only what helps the next plan (e.g., any discovered pagination hints, whether links look like profiles, simple selector hints). Do NOT include raw HTML or free text in this JSON.\n")
	sb.WriteString("2) CONTENT–EXTENSION MATCH: Write JSON only to '.json' files. If you must persist raw HTML, use '.html' (under 'tmp/'). If free text, use '.txt'.\n")
	sb.WriteString("3) TEMP ARTIFACTS: All temporary/intermediate artifacts (evidence, caches, snapshots) MUST be under 'tmp/'. Also ensure meta.handoff_path begins with 'tmp/'.\n")
	sb.WriteString("4) CONTINUED STAGES: For follow-up plans, continue stage numbering after the previous plan’s last stage (look for 'PREV_LAST_STAGE: N' in the context). Do not restart from 1.\n")
	sb.WriteString("5) DEPENDENCIES: NEVER reference '@results.<id>' from an action in the SAME stage. If A depends on B, place A in a LATER stage.\n")
	sb.WriteString("6) NETWORK I/O: Use 'web.request' for single URLs. For lists, prefer 'flow.foreach' with 'template.action=web.request'. Do NOT invent URLs; discover links from actual HTML via 'html.links'/'html.links_bulk'.\n")
	sb.WriteString("7) LIST TYPE DISCIPLINE: If you have an array of link OBJECTS, use 'list.pluck(field=\"url\")' to get an array of strings BEFORE 'url.normalize', 'list.unique', 'list.concat', or 'flow.foreach'. Never mix arrays of objects and arrays of strings.\n")
	sb.WriteString("8) FOREACH SHAPE: 'flow.foreach' REQUIRES 'template.action' and 'template.payload'. Use '{{item}}' or '{{item.field}}' in 'template.payload'.\n")
	sb.WriteString("9) STRUCTURED EXTRACTION: Use 'llm.extract_structured' to map provided text/HTML to a strict JSON schema; if a field is missing, output empty string/array; never invent facts.\n")
	sb.WriteString("10) FINAL OUTPUTS: Persist final deliverables with 'system.write_file_atomic'. Final outputs need not be under 'tmp/'.\n")
	sb.WriteString("11) IDS: Action IDs must be short, unique, lowercase across the WHOLE mission. Use new IDs in re-plans and reference old outputs via '@results.<old_id>.<key>'.\n")
	sb.WriteString("12) URL RESOLUTION: Provide 'base_url' when calling 'html.links' and 'url.normalize' so relative hrefs resolve.\n\n")

	sb.WriteString("Generate the plan now for this goal:\n")
	sb.WriteString(fmt.Sprintf("User Goal: \"%s\"\n", userGoal))
	sb.WriteString("Assistant: ")

	return sb.String()
}

// Prompt for first hand analyzing user intent
func buildIntentPrompt(userGoal string) string {
	var sb strings.Builder
	sb.WriteString("You are an expert user intent analyzer. Respond ONLY with this JSON (no extra text):\n")
	sb.WriteString("{\"requires_confirmation\": <bool>, \"run_manual_plans\": <bool>, \"manual_plans_path\": \"<string or empty>\", \"manual_plan_names\": [<zero or more strings in order>], \"cancel\": <bool>, \"target_mission_id\": \"<string or empty>\", \"target_is_previous\": <bool>}\n\n")

	sb.WriteString("Rules:\n")
	sb.WriteString("- requires_confirmation: true ONLY if the user asks to see/review/confirm/approve/preview before execution OR uses verbs like 'show', 'list', 'preview'.\n")
	sb.WriteString("- run_manual_plans: true if the user asks to execute (or show/preview) plans/missions from a local .json file.\n")
	sb.WriteString("- manual_plans_path: extract the local .json path verbatim (quoted or unquoted). If none, use empty string.\n")
	sb.WriteString("- manual_plan_names: if the user names specific missions, return them in order; otherwise an empty array. If empty and run_manual_plans is true, default behavior is to run ALL missions in the file.\n")
	sb.WriteString("- cancel: true if the user asks to stop/abort/kill/cancel a mission or plan (treat plan == mission).\n")
	sb.WriteString("- target_mission_id: if the user mentions a specific mission/plan ID, put it here (otherwise empty).\n")
	sb.WriteString("- target_is_previous: true if the user says 'previous', 'last', or 'most recent' mission/plan (otherwise false).\n")
	sb.WriteString("- Only consider local files ending with .json. Ignore URLs.\n\n")

	sb.WriteString("Examples:\n")
	sb.WriteString("User: \"show me the plans from tests/test_plans.json\"\n")
	sb.WriteString("Assistant: {\"requires_confirmation\": true, \"run_manual_plans\": true, \"manual_plans_path\": \"tests/test_plans.json\", \"manual_plan_names\": []}\n\n")
	sb.WriteString("User: \"execute the plans 'Create file', 'Import Data' in test.json\"\n")
	sb.WriteString("Assistant: {\"requires_confirmation\": false, \"run_manual_plans\": true, \"manual_plans_path\": \"test.json\", \"manual_plan_names\": [\"Create file\", \"Import Data\"]}\n\n")

	sb.WriteString("User Goal: \"")
	sb.WriteString(userGoal)
	sb.WriteString("\"\nAssistant JSON response: ")
	return sb.String()
}

// Generating plan for a mission
func GeneratePlan(ctx context.Context, history []ConversationTurn, userGoal string) (*ExecutionPlan, error) {
	prompt := buildPlanPrompt(history, userGoal)

	cleanJson, err := llm_client.GenerateJSON(ctx, prompt, "gemini-2.0-flash", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan from LLM: %w", err)
	}

	var plan ExecutionPlan
	err = json.Unmarshal([]byte(cleanJson), &plan)
	if err != nil {
		return nil, fmt.Errorf("error parsing generated plan JSON: %v\nRaw Response: %s", err, cleanJson)
	}

	// Validate actions and plan structure
	if err := ValidatePlan(&plan); err != nil {
		return nil, fmt.Errorf("generated plan invalid: %w", err)
	}

	return &plan, nil
}

func AnalyzeGoalIntent(ctx context.Context, userGoal string) (*GoalIntent, error) {
	prompt := buildIntentPrompt(userGoal)

	cleanJson, err := llm_client.GenerateJSON(ctx, prompt, "gemini-2.0-flash", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate intent from LLM: %w", err)
	}

	var intent GoalIntent
	err = json.Unmarshal([]byte(cleanJson), &intent)
	if err != nil {
		return nil, fmt.Errorf("error parsing generated intent JSON: %v\nRaw Response: %s", err, cleanJson)
	}

	if !intent.RunManualPlans {
		intent.ManualPlansPath = ""
	}
	if !intent.Cancel {
		intent.TargetMissionID = ""
		intent.TargetIsPrevious = false
	}
	return &intent, nil
}
