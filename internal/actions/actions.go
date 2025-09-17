package actions

import (
	"context"
	"fmt"
	"strings"

	"a-a/internal/actions/llm"
	"a-a/internal/actions/system"
	"a-a/internal/actions/test"
	"a-a/internal/parser"
	"a-a/internal/utils"
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
		return handleWebAction(operation, action.Payload)
	case "llm":
		return handleLlmAction(operation, action.Payload)
	case "test":
		return handleTestAction(ctx, operation, action.Payload)
	case "intent":
		if operation == "unknown" {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("unknown action category: %s", category)
}

func handleSystemAction(operation string, payload map[string]any) (map[string]any, error) {
	path, err := utils.GetStringPayload(payload, "path")
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
		content, err := utils.GetStringPayload(payload, "content")
		if err != nil {
			return nil, err
		}
		return nil, system.WriteFile(path, content)
	case "write_file_atomic":
		content, err := utils.GetStringPayload(payload, "content")
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

func handleWebAction(operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	default:
		_, err := utils.GetStringPayload(payload, "temp")
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unknown web operation: %s", operation)
	}
}

func handleLlmAction(operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "generate_content":
		prompt, err := utils.GetStringPayload(payload, "prompt")
		if err != nil {
			return nil, err
		}
		model, _ := payload["model"].(string)
		return llm.GenerateContentGemini(prompt, model)
	default:
		return nil, fmt.Errorf("unknown llm operation: %s", operation)
	}
}

func handleTestAction(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "sleep":
		sleepSecond, err := utils.GetIntPayload(payload, "duration_ms")
		if err != nil {
			return nil, err
		}
		return nil, test.Sleep(ctx, sleepSecond)
	case "fail":
		afterSeconds, err := utils.GetIntPayload(payload, "duration_ms")
		if err != nil {
			afterSeconds = 0
		}
		return nil, test.Fail(ctx, "", afterSeconds)
	case "sleep_with_return":
		sleepSecond, err := utils.GetIntPayload(payload, "duration_ms")
		if err != nil {
			return nil, err
		}
		return test.SleepWithReturn(ctx, sleepSecond)
	default:
		return nil, fmt.Errorf("unknown test operation: %s", operation)
	}
}
