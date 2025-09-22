package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"a-a/internal/llm_client"
	"a-a/internal/utils"
)

// Hard guardrails on model names
func allowedModelOrDefault(m string) string {
	return llm_client.AllowedModelOrDefault(m)
}

func GenerateContentGemini(ctx context.Context, prompt string, model_name string) (map[string]any, error) {
	model := allowedModelOrDefault(model_name)
	generatedText, err := llm_client.Generate(ctx, prompt, model)
	if err != nil {
		return nil, err
	}
	return map[string]any{"generated_content": generatedText}, nil
}

func ExtractStructured(ctx context.Context, input string, schema any, instruction, model string) (map[string]any, error) {
	model = allowedModelOrDefault(model)
	var sb strings.Builder
	if strings.TrimSpace(instruction) != "" {
		sb.WriteString(instruction)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Extract structured data that conforms to the provided JSON schema. Return ONLY valid JSON (no extra text).\n\n")
	sb.WriteString("=== Input Start ===\n")
	sb.WriteString(input)
	sb.WriteString("\n=== Input End ===\n")
	jsonOut, err := llm_client.GenerateJSON(ctx, sb.String(), model, schema)
	if err != nil {
		return nil, err
	}
	var scratch any
	if err := json.Unmarshal([]byte(jsonOut), &scratch); err != nil {
		return nil, fmt.Errorf("LLM did not return valid JSON: %w\nRaw: %s", err, jsonOut)
	}
	return map[string]any{"json": jsonOut}, nil
}

func SelectFromList(ctx context.Context, listJSON, instruction, model string, limit int) (map[string]any, error) {
	model = allowedModelOrDefault(model)
	if strings.TrimSpace(instruction) == "" {
		instruction = "From the input array, return ONLY the items that match the criteria. Do not rewrite items; just copy them. Return a JSON array."
	}
	if limit <= 0 {
		limit = 5000
	}
	prompt := fmt.Sprintf(`%s

RULES:
- Input is a JSON array.
- Output must be a JSON array with a subset of the input items (verbatim copies).
- No commentary.

INPUT:
%s

OUTPUT (JSON only):`, instruction, listJSON)

	jsonOut, err := llm_client.GenerateJSON(ctx, prompt, model, nil)
	if err != nil {
		return nil, err
	}
	// quick size sanity (optional)
	if len(jsonOut) > 2*1024*1024 {
		return nil, fmt.Errorf("selected_json too large")
	}
	return map[string]any{"selected_json": jsonOut}, nil
}

func HandleLlmAction(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "generate_content":
		prompt, err := utils.GetStringPayload(payload, "prompt")
		if err != nil {
			return nil, err
		}
		model, _ := payload["model"].(string)
		return GenerateContentGemini(ctx, prompt, model)

	case "extract_structured":
		input, err := utils.GetStringPayload(payload, "input")
		if err != nil {
			return nil, err
		}
		instruction, _ := payload["instruction"].(string)
		model, _ := payload["model"].(string)

		var schema any
		if s, ok := payload["schema"].(string); ok && strings.TrimSpace(s) != "" {
			if err := json.Unmarshal([]byte(s), &schema); err != nil {
				return nil, fmt.Errorf("schema must be JSON: %w", err)
			}
		} else if m, ok := payload["schema"].(map[string]any); ok {
			schema = m
		} else {
			return nil, fmt.Errorf("payload missing 'schema'")
		}
		return ExtractStructured(ctx, input, schema, instruction, model)

	case "select_from_list":
		listJSON, err := utils.GetStringPayload(payload, "list_json")
		if err != nil {
			return nil, err
		}
		instruction, _ := payload["instruction"].(string)
		model, _ := payload["model"].(string)
		limit := 0
		if v, ok := payload["limit"]; ok {
			if i, err := utils.GetIntPayload(map[string]any{"v": v}, "v"); err == nil {
				limit = i
			}
		}
		return SelectFromList(ctx, listJSON, instruction, model, limit)

	default:
		return nil, fmt.Errorf("unknown llm operation: %s", operation)
	}
}
