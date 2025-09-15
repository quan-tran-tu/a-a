package actions

import (
	"context"
	"fmt"
	"strings"

	"a-a/internal/actions/llm"
	"a-a/internal/actions/system"
	"a-a/internal/parser"
)

func Execute(ctx context.Context, action *parser.Action) (map[string]any, error) {
	actionParts := strings.Split(action.Action, ".")
	if len(actionParts) != 2 {
		return nil, fmt.Errorf("invalid action type format: '%s'", action.Action)
	}

	category := actionParts[0]
	operation := actionParts[1]

	switch category {
	case "system":
		return handleSystemAction(operation, action.Payload)
	case "web":
		return handleWebAction(ctx, operation, action.Payload)
	case "llm":
		return handleLlmAction(operation, action.Payload)
	case "intent":
		if operation == "unknown" {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("unknown action category: %s", category)
}

func getStringPayload(payload map[string]any, key string) (string, error) {
	value, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("payload is missing required key: '%s'", key)
	}
	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("payload key '%s' has an invalid type (expected string)", key)
	}
	return strValue, nil
}

func handleSystemAction(operation string, payload map[string]any) (map[string]any, error) {
	path, err := getStringPayload(payload, "path")
	if err != nil {
		if _, ok := payload["content"]; !ok && (operation != "write_file" && operation != "write_file_atomic") {
			return nil, err
		}
	}

	switch operation {
	case "create_file":
		return nil, system.CreateFile(path)
	case "delete_file":
		return nil, system.DeleteFile(path)
	case "create_folder":
		return nil, system.CreateFolder(path)
	case "delete_folder":
		return nil, system.DeleteFolder(path)
	case "write_file":
		content, err := getStringPayload(payload, "content")
		if err != nil {
			return nil, err
		}
		return nil, system.WriteFile(path, content)
	case "write_file_atomic":
		content, err := getStringPayload(payload, "content")
		if err != nil {
			return nil, err
		}
		return nil, system.WriteFileAtomic(path, content)
	case "read_file":
		return system.ReadFile(path)
	case "list_directory":
		return system.ListDirectory(path)
	default:
		return nil, fmt.Errorf("unknown system operation: %s", operation)
	}
}

func handleWebAction(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	default:
		return nil, fmt.Errorf("unknown web operation: %s", operation)
	}
}

func handleLlmAction(operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "generate_content":
		prompt, err := getStringPayload(payload, "prompt")
		if err != nil {
			return nil, err
		}
		model, _ := payload["model"].(string)
		return llm.GenerateContentGemini(prompt, model)
	default:
		return nil, fmt.Errorf("unknown llm operation: %s", operation)
	}
}
