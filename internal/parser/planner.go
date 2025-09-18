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

	sb.WriteString("You are an expert AI workflow planner. Your task is to convert a user's goal into a structured JSON execution plan. Respond ONLY with the JSON plan. Do not include any other text, explanations, or markdown formatting.\n\n")
	if len(history) > 0 {
		sb.WriteString("CONVERSATION HISTORY (for context):\n")
		for _, turn := range history {
			sb.WriteString(fmt.Sprintf("User Goal: \"%s\"\n", turn.UserGoal))
			sb.WriteString(fmt.Sprintf("Previous Assistant Plan: %s\n", turn.AssistantPlan))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("The plan consists of stages. All actions in one stage run in parallel. The plan proceeds to the next stage only after the current one is complete. Action outputs can be referenced in later stages using the '@results.action_id.output_key' syntax.\n\n")
	sb.WriteString(registry.GeneratePromptPart() + "\n")

	sb.WriteString("\nHARD RULES:\n")
	sb.WriteString("- The LLM must NOT invent or guess URLs. Do not 'generate' URL lists.\n")
	sb.WriteString("- Discover pagination and detail links ONLY from the provided HTML using html.* actions (e.g., `html.links`).\n")
	sb.WriteString("- All network I/O (fetching pages) must use `web.request` or `web.batch_request`.\n")
	sb.WriteString("- Before passing URLs to actions that expect a list of strings (e.g., batch fetch), extract strings with `list.pluck` (field=\"url\") from link objects.\n")
	sb.WriteString("- Use `url.normalize` and `list.unique` before batch fetches.\n")
	sb.WriteString("- Use `llm.extract_structured` ONLY to map provided HTML/text to a strict schema. If a field is missing, output null/empty; never invent.\n")
	sb.WriteString("- Write files with `system.write_file_atomic`.\n\n")

	sb.WriteString("EXAMPLE:\n")
	sb.WriteString("User Goal: \"Summarize the homepages of the NY Times and Reuters and save the summary to a file named 'news.md'\"\n")
	sb.WriteString("Assistant: {\"plan\":[{\"stage\":1,\"actions\":[{\"id\":\"fetch_nytimes\",\"action\":\"web.fetch_page_content\",\"payload\":{\"url\":\"https://www.nytimes.com\"}},{\"id\":\"fetch_reuters\",\"action\":\"web.fetch_page_content\",\"payload\":{\"url\":\"https://www.reuters.com\"}}]},{\"stage\":2,\"actions\":[{\"id\":\"summarize_news\",\"action\":\"llm.generate_content\",\"payload\":{\"prompt\":\"Summarize these two articles:\\n\\nArticle 1: @results.fetch_nytimes.content\\n\\nArticle 2: @results.fetch_reuters.content\"}}]},{\"stage\":3,\"actions\":[{\"id\":\"save_summary\",\"action\":\"system.write_file\",\"payload\":{\"path\":\"news.md\",\"content\":\"@results.summarize_news.generated_content\"}}]}]}\n\n")
	sb.WriteString("Now, generate a plan for the following user goal:\n")
	sb.WriteString(fmt.Sprintf("User Goal: \"%s\"\n", userGoal))
	sb.WriteString("Assistant: ")

	return sb.String()
}

// Prompt for first hand analyzing user intent
func buildIntentPrompt(userGoal string) string {
	var sb strings.Builder
	sb.WriteString("You are an expert user intent analyzer. Respond ONLY with this JSON (no extra text):\n")
	sb.WriteString("{\"requires_confirmation\": <bool>, \"run_manual_plans\": <bool>, \"manual_plans_path\": \"<string or empty>\", \"manual_plan_names\": [<zero or more strings in order>]}\n\n")

	sb.WriteString("Rules:\n")
	sb.WriteString("- requires_confirmation: true ONLY if the user asks to see/review/confirm/approve/preview before execution OR uses verbs like 'show', 'list', 'preview'.\n")
	sb.WriteString("- run_manual_plans: true if the user asks to execute (or show/preview) plans/missions from a local .json file.\n")
	sb.WriteString("- manual_plans_path: extract the local .json path verbatim (quoted or unquoted). If none, use empty string.\n")
	sb.WriteString("- manual_plan_names: if the user names specific missions, return them in order; otherwise an empty array. If empty and run_manual_plans is true, default behavior is to run ALL missions in the file.\n")
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

	// Check for invalid action found in LLM response
	for _, stage := range plan.Plan {
		for _, action := range stage.Actions {
			if err := registry.ValidateAction(&action); err != nil {
				return nil, fmt.Errorf("generated plan contains an invalid action: %w", err)
			}
		}
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
	return &intent, nil
}
