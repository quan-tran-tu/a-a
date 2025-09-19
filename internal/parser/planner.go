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
func buildPlanPrompt(history []ConversationTurn, userGoal string) string {
	var sb strings.Builder

	sb.WriteString("You are an expert AI workflow planner. Convert the user's goal into a STRICT JSON execution plan with schema: {\"plan\":[{\"stage\":<int>,\"actions\":[{\"id\":\"<slug>\",\"action\":\"<category.operation>\",\"payload\":{...}}]}]}.\n")
	sb.WriteString("Respond ONLY with JSON. No extra text.\n\n")

	if len(history) > 0 {
		sb.WriteString("CONVERSATION HISTORY (context):\n")
		for _, turn := range history {
			sb.WriteString(fmt.Sprintf("User Goal: \"%s\"\n", turn.UserGoal))
			sb.WriteString(fmt.Sprintf("Previous Assistant Plan: %s\n", turn.AssistantPlan))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("SEMANTICS:\n")
	sb.WriteString("- Actions in the SAME stage run IN PARALLEL.\n")
	sb.WriteString("- Stages run SEQUENTIALLY (stage N completes before stage N+1).\n")
	sb.WriteString("- Only outputs from PRIOR stages may be referenced via '@results.<action_id>.<key>'.\n\n")

	sb.WriteString(registry.GeneratePromptPart() + "\n")

	sb.WriteString("\nHARD RULES:\n")
	sb.WriteString("1) NO SAME-STAGE DEPENDENCIES: If an action references '@results.<id>', that referenced action MUST be in a STRICTLY EARLIER stage. Do not place dependent actions in the same stage. Within any stage, actions must be independent (no '@results' that point to same-stage IDs).\n")
	sb.WriteString("2) NETWORK I/O: Use only 'web.request' (single) or 'web.batch_request' (many) for fetching content.\n")
	sb.WriteString("3) LINK DISCOVERY: Discover links ONLY from provided HTML using 'html.links' (single page) or 'html.links_bulk' (multiple pages). Never invent URLs.\n")
	sb.WriteString("4) URL LIST PIPELINE: When preparing URLs for batch fetching, use: 'list.pluck' (field=\"url\") → 'url.normalize' → 'list.unique'.\n")
	sb.WriteString("5) BASE URL:\n")
	sb.WriteString("   - For 'html.links', ALWAYS provide 'base_url' equal to the exact page URL whose HTML you pass.\n")
	sb.WriteString("   - For 'url.normalize', provide an appropriate 'base_url' when inputs may be relative.\n")
	sb.WriteString("   - For 'html.links_bulk', do NOT override per-page bases unless necessary (omit 'base_url' so each page's own URL is used).\n")
	sb.WriteString("6) *_json PAYLOADS: Any payload key ending with '_json' (e.g., 'urls_json', 'list_json', 'pages_json', 'values_json') MUST be a STRING containing a valid JSON array. If empty, use \"[]\". Do NOT pass a bare array value.\n")
	sb.WriteString("7) LIST FILTERING: Use 'llm.select_from_list' to select a subset from an array (verbatim copies of items). Do not rewrite items.\n")
	sb.WriteString("8) STRUCTURED EXTRACTION: Use 'llm.extract_structured' to map raw HTML/text into a strict JSON schema. If a field is missing, output an empty string/array; never fabricate data.\n")
	sb.WriteString("9) PERSISTENCE: Save files using 'system.write_file_atomic'.\n")
	sb.WriteString("10) IDS: All action IDs must be unique, short, lowercase (e.g., 'fetch_seed', 'select_pages', 'fetch_profiles', 'extract_items').\n")

	sb.WriteString("\nNow, generate a plan for the following user goal:\n")
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
