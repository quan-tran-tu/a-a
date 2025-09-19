package list

import (
	"context"
	"encoding/json"
	"fmt"

	"a-a/internal/utils"
)

func handlePluck(_ context.Context, payload map[string]any) (map[string]any, error) {
	listJSON, err := utils.GetStringPayload(payload, "list_json")
	if err != nil {
		return nil, err
	}
	field, err := utils.GetStringPayload(payload, "field")
	if err != nil {
		return nil, err
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(listJSON), &arr); err != nil {
		return nil, fmt.Errorf("list_json must be array of objects: %w", err)
	}
	out := make([]string, 0, len(arr))
	for _, obj := range arr {
		if v, ok := obj[field]; ok {
			out = append(out, fmt.Sprintf("%v", v))
		}
	}
	b, _ := json.Marshal(out)
	return map[string]any{"values_json": string(b)}, nil
}

func handleUnique(_ context.Context, payload map[string]any) (map[string]any, error) {
	listJSON, err := utils.GetStringPayload(payload, "list_json")
	if err != nil {
		return nil, err
	}
	var arr []any
	if err := json.Unmarshal([]byte(listJSON), &arr); err != nil {
		return nil, fmt.Errorf("list_json must be array: %w", err)
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		s := fmt.Sprintf("%v", v)
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	b, _ := json.Marshal(out)
	return map[string]any{"list_json": string(b)}, nil
}

func handleConcat(_ context.Context, payload map[string]any) (map[string]any, error) {
	a, err := utils.GetStringPayload(payload, "a_json")
	if err != nil {
		return nil, err
	}
	bb, err := utils.GetStringPayload(payload, "b_json")
	if err != nil {
		return nil, err
	}
	var A, B []any
	if err := json.Unmarshal([]byte(a), &A); err != nil {
		return nil, fmt.Errorf("a_json invalid array: %w", err)
	}
	if err := json.Unmarshal([]byte(bb), &B); err != nil {
		return nil, fmt.Errorf("b_json invalid array: %w", err)
	}
	out := append(A, B...)
	res, _ := json.Marshal(out)
	return map[string]any{"list_json": string(res)}, nil
}

func HandleListAction(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "pluck":
		return handlePluck(ctx, payload)
	case "unique":
		return handleUnique(ctx, payload)
	case "concat":
		return handleConcat(ctx, payload)
	default:
		return nil, fmt.Errorf("unknown list operation: %s", operation)
	}
}
