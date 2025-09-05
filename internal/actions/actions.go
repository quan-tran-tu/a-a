package actions

import (
	"fmt"
	"strings"

	"a-a/internal/actions/apps"
	"a-a/internal/actions/system"
	"a-a/internal/actions/web"
	"a-a/internal/parser"
)

func Execute(action *parser.Action) error {
	actionParts := strings.Split(action.Action, ".")
	if len(actionParts) != 2 {
		return fmt.Errorf("invalid action type format: '%s'", action.Action)
	}

	category := actionParts[0]
	operation := actionParts[1]

	payloadValue := action.Payload.Value

	switch category {
	case "system":
		return handleSystemAction(operation, payloadValue)
	case "web":
		return handleWebAction(operation, payloadValue)
	case "apps":
		return handleAppsAction(operation, payloadValue)
	default:
		return fmt.Errorf("unknown action category: %s", category)
	}
}

func handleSystemAction(operation, payload string) error {
	switch operation {
	case "create_file":
		return system.CreateFile(payload)
	case "delete_file":
		return system.DeleteFile(payload)
	case "create_folder":
		return system.CreateFolder(payload)
	case "delete_folder":
		return system.DeleteFolder(payload)
	default:
		return fmt.Errorf("unknown system operation: %s", operation)
	}
}

func handleWebAction(operation, payload string) error {
	switch operation {
	case "search":
		return web.Search(payload)
	default:
		return fmt.Errorf("unknown web operation: %s", operation)
	}
}

func handleAppsAction(operation, payload string) error {
	switch operation {
	case "open":
		return apps.Open(payload)
	default:
		return fmt.Errorf("unknown app operation: %s", operation)
	}
}
