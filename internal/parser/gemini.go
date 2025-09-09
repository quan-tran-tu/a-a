package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"a-a/internal/llm_client"
)

var registry *ActionRegistry

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
	sb.WriteString("EXAMPLE:\n")
	sb.WriteString("User Goal: \"Summarize the homepages of the NY Times and Reuters and save the summary to a file named 'news.md'\"\n")
	sb.WriteString("Assistant: {\"plan\":[{\"stage\":1,\"actions\":[{\"id\":\"fetch_nytimes\",\"action\":\"web.fetch_page_content\",\"payload\":{\"url\":\"https://www.nytimes.com\"}},{\"id\":\"fetch_reuters\",\"action\":\"web.fetch_page_content\",\"payload\":{\"url\":\"https://www.reuters.com\"}}]},{\"stage\":2,\"actions\":[{\"id\":\"summarize_news\",\"action\":\"llm.generate_text\",\"payload\":{\"prompt\":\"Summarize these two articles:\\n\\nArticle 1: @results.fetch_nytimes.content\\n\\nArticle 2: @results.fetch_reuters.content\"}}]},{\"stage\":3,\"actions\":[{\"id\":\"save_summary\",\"action\":\"system.write_file\",\"payload\":{\"path\":\"news.md\",\"content\":\"@results.summarize_news.generated_text\"}}]}]}\n\n")
	sb.WriteString("Now, generate a plan for the following user goal:\n")
	sb.WriteString(fmt.Sprintf("User Goal: \"%s\"\n", userGoal))
	sb.WriteString("Assistant: ")

	return sb.String()
}

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

	for _, stage := range plan.Plan {
		for _, action := range stage.Actions {
			if err := registry.ValidateAction(&action); err != nil {
				return nil, fmt.Errorf("generated plan contains an invalid action: %w", err)
			}
		}
	}

	return &plan, nil
}
