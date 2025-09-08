package actions

import (
	"fmt"
	"strings"

	"a-a/internal/actions/apps"
	"a-a/internal/actions/llm"
	"a-a/internal/actions/system"
	"a-a/internal/actions/tools"
	"a-a/internal/actions/web"
	"a-a/internal/parser"
)

func Execute(action *parser.Action) (map[string]any, error) {
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
	case "apps":
		return handleAppsAction(operation, action.Payload)
	case "tools":
		return handleToolsAction(operation, action.Payload)
	case "llm":
		return handleLlmAction(operation, action.Payload)
	case "intent":
		if operation == "unknown" {
			// This action succeeds but produces no output.
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

func getStringSlicePayload(payload map[string]any, key string) ([]string, error) {
	var slice []string
	value, ok := payload[key]
	if !ok {
		return slice, nil
	}

	interfaceSlice, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("payload key '%s' has an invalid type (expected a slice)", key)
	}

	for _, v := range interfaceSlice {
		if str, ok := v.(string); ok {
			slice = append(slice, str)
		} else {
			return nil, fmt.Errorf("an item in the '%s' slice was not a string", key)
		}
	}
	return slice, nil
}

func handleSystemAction(operation string, payload map[string]any) (map[string]any, error) {
	path, err := getStringPayload(payload, "path")
	if err != nil {
		if _, ok := payload["content"]; !ok && operation != "write_file" {
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
	default:
		return nil, fmt.Errorf("unknown system operation: %s", operation)
	}
}

func handleWebAction(operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "search":
		query, err := getStringPayload(payload, "query")
		if err != nil {
			return nil, err
		}
		return nil, web.Search(query)
	default:
		return nil, fmt.Errorf("unknown web operation: %s", operation)
	}
}

func handleAppsAction(operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "open":
		appName, err := getStringPayload(payload, "appName")
		if err != nil {
			return nil, err
		}
		return nil, apps.Open(appName)
	default:
		return nil, fmt.Errorf("unknown app operation: %s", operation)
	}
}

func handleToolsAction(operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "git":
		subcommand, err := getStringPayload(payload, "subcommand")
		if err != nil {
			return nil, err
		}
		args, err := getStringSlicePayload(payload, "args")
		if err != nil {
			return nil, err
		}
		path, _ := payload["path"].(string)

		return nil, tools.HandleGit(subcommand, args, path)
	default:
		return nil, fmt.Errorf("unknown tool operation: %s", operation)
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
