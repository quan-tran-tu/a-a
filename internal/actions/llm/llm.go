package llm

import (
	"a-a/internal/llm_client"
)

func GenerateContentGemini(prompt string, model_name string) (map[string]any, error) {
	generatedText, err := llm_client.Generate(prompt, model_name)
	if err != nil {
		return nil, err
	}

	output := make(map[string]any)
	output["generated_text"] = generatedText
	return output, nil
}
