package llm

import (
	"fmt"
	"strings"

	"a-a/internal/llm_client"
	"a-a/internal/utils"
)

// Hard guardrails on model names
func allowedModelOrDefault(m string) string {
	m = strings.TrimSpace(strings.ToLower(m))
	if m == "" || !strings.HasPrefix(m, "gemini-") {
		return "gemini-2.0-flash"
	}
	return m
}

func GenerateContentGemini(prompt string, model_name string) (map[string]any, error) {
	model := allowedModelOrDefault(model_name)
	generatedText, err := llm_client.Generate(prompt, model)
	if err != nil {
		return nil, err
	}

	output := make(map[string]any)
	output["generated_content"] = generatedText
	return output, nil
}

func HandleLlmAction(operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "generate_content":
		prompt, err := utils.GetStringPayload(payload, "prompt")
		if err != nil {
			return nil, err
		}
		model, _ := payload["model"].(string)
		return GenerateContentGemini(prompt, model)
	default:
		return nil, fmt.Errorf("unknown llm operation: %s", operation)
	}
}
