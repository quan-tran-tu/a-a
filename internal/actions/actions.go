package actions

import (
	"context"
	"fmt"
	"strings"

	"a-a/internal/actions/llm"
	"a-a/internal/actions/system"
	"a-a/internal/actions/test"
	"a-a/internal/actions/web"
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
		return system.HandleSystemAction(ctx, operation, action.Payload)
	case "web":
		return web.HandleWebAction(ctx, operation, action.Payload)
	case "llm":
		return llm.HandleLlmAction(ctx, operation, action.Payload)
	case "test":
		return test.HandleTestAction(ctx, operation, action.Payload)
	case "intent":
		if operation == "unknown" {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("unknown action category: %s", category)
}
