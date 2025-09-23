package url

import (
	"context"
	"encoding/json"
	"fmt"

	"a-a/internal/utils"
)

func HandleURLAction(_ context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "normalize":
		urlsJSON, err := utils.GetStringPayload(payload, "urls_json")
		if err != nil {
			return nil, err
		}
		base, _ := payload["base_url"].(string)
		var urls []string
		if err := json.Unmarshal([]byte(urlsJSON), &urls); err != nil {
			return nil, fmt.Errorf("urls_json must be array of strings: %w", err)
		}
		out := make([]string, 0, len(urls))
		for _, u := range urls {
			out = append(out, utils.Absolute(base, u))
		}
		b, _ := json.Marshal(out)
		return map[string]any{"urls_json": string(b)}, nil
	default:
		return nil, fmt.Errorf("unknown url operation: %s", operation)
	}
}
