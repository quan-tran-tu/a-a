package web

import (
	"context"
	"fmt"

	"a-a/internal/utils"
)

func HandleWebAction(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	default:
		_, err := utils.GetStringPayload(payload, "temp")
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unknown web operation: %s", operation)
	}
}
