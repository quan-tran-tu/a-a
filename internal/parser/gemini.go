package parser

import (
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

func buildIntentPrompt(userGoal string) string {
	var sb strings.Builder
	sb.WriteString("You are an expert user intent analyzer. Your task is to analyze the user's goal and determine their meta-intents. Respond ONLY with a JSON object. Do not include any other text or markdown formatting.\n\n")
	sb.WriteString("The user's goal is: \"")
	sb.WriteString(userGoal)
	sb.WriteString("\"\n\n")
	sb.WriteString("Analyze the goal for the following intents:\n")
	sb.WriteString("- `requires_confirmation`: Set to `true` if the user explicitly asks to see, review, confirm, or approve the plan before execution. Otherwise, set to `false`.\n\n")
	sb.WriteString("EXAMPLE 1:\n")
	sb.WriteString("User Goal: \"create a file and show me the plan\"\n")
	sb.WriteString("Assistant: {\"requires_confirmation\": true}\n\n")
	sb.WriteString("EXAMPLE 2:\n")
	sb.WriteString("User Goal: \"Summarize the top headlines on CNN\"\n")
	sb.WriteString("Assistant: {\"requires_confirmation\": false}\n\n")
	sb.WriteString("Assistant JSON response: ")
	return sb.String()
}

// Generating plan for a mission
func GeneratePlan(history []ConversationTurn, userGoal string) (*ExecutionPlan, error) {
	prompt := buildPlanPrompt(history, userGoal)

	generatedText, err := llm_client.Generate(prompt, "gemini-2.0-flash")
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan from LLM: %w", err)
	}

	cleanJson := strings.TrimPrefix(generatedText, "```json")
	cleanJson = strings.TrimPrefix(cleanJson, "```")
	cleanJson = strings.TrimSuffix(cleanJson, "```")
	cleanJson = strings.TrimSpace(cleanJson)

	var plan ExecutionPlan
	err = json.Unmarshal([]byte(cleanJson), &plan)
	if err != nil {
		return nil, fmt.Errorf("error parsing generated plan JSON: %v\nRaw Response: %s", err, generatedText)
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

func AnalyzeGoalIntent(userGoal string) (*GoalIntent, error) {
	prompt := buildIntentPrompt(userGoal)

	generatedText, err := llm_client.Generate(prompt, "gemini-2.0-flash")
	if err != nil {
		return nil, fmt.Errorf("failed to generate intent from LLM: %w", err)
	}

	cleanJson := strings.TrimPrefix(generatedText, "```json")
	cleanJson = strings.TrimPrefix(cleanJson, "```")
	cleanJson = strings.TrimSuffix(cleanJson, "```")
	cleanJson = strings.TrimSpace(cleanJson)

	var intent GoalIntent
	err = json.Unmarshal([]byte(cleanJson), &intent)
	if err != nil {
		return nil, fmt.Errorf("error parsing generated intent JSON: %v\nRaw Response: %s", err, generatedText)
	}

	return &intent, nil
}
