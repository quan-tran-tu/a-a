package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"a-a/internal/actions/html"
	"a-a/internal/actions/list"
	"a-a/internal/actions/llm"
	"a-a/internal/actions/system"
	"a-a/internal/actions/test"
	"a-a/internal/actions/url"
	"a-a/internal/actions/web"
	"a-a/internal/parser"

	"golang.org/x/sync/errgroup"
)

const (
	foreachConcurrency = 8
	defaultInnerMs     = 30000
)

func HandleFlowAction(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "foreach":
		return foreach(ctx, payload)
	default:
		return nil, fmt.Errorf("unknown flow operation: %s", operation)
	}
}

// Applies a single-item action template to each element of items_json.
// Required payload:
//
//	items_json: JSON array string or slice
//	template:   { action: "<category.op>", payload: { ... }, id_prefix: "task_" }
//
// Currently hardcoded settings:
//   - concurrency: 8
//   - on_error:    "continue" (collect failures; do NOT fail-fast)
//
// Output:
//
//	{ "results_json": "<JSON array of inner outputs>", "errors_json": "<JSON array of {item,error}>" }
func foreach(parentCtx context.Context, payload map[string]any) (map[string]any, error) {
	items, err := coerceToSlice(payload["items_json"])
	if err != nil {
		return nil, fmt.Errorf("flow.foreach: invalid items_json: %w", err)
	}
	if len(items) == 0 {
		return map[string]any{"results_json": "[]", "errors_json": "[]"}, nil
	}

	tplRaw, ok := payload["template"].(map[string]any)
	if !ok {
		return nil, errors.New("flow.foreach: payload.template must be an object")
	}
	tplAction, _ := tplRaw["action"].(string)
	if strings.TrimSpace(tplAction) == "" {
		return nil, errors.New("flow.foreach: template.action is required")
	}
	tplPayload, ok := tplRaw["payload"].(map[string]any)
	if !ok {
		return nil, errors.New("flow.foreach: template.payload must be an object")
	}
	idPrefix, _ := tplRaw["id_prefix"].(string)
	if idPrefix == "" {
		idPrefix = "task_"
	}

	// Derive per-item timeout from registry default of the template action
	perItemTimeout := time.Duration(defaultInnerMs) * time.Millisecond
	if def, ok := parser.GetActionDefinition(tplAction); ok && def.DefaultTimeoutMs > 0 {
		perItemTimeout = time.Duration(def.DefaultTimeoutMs) * time.Millisecond
	}

	// Parent context for the batch
	baseCtx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	type okOut map[string]any
	type errOut struct {
		Item  any    `json:"item"`
		Error string `json:"error"`
	}

	okResults := make([]okOut, len(items))
	errResults := make([]errOut, 0, 4)
	var mu sync.Mutex

	// Bounded concurrency with errgroup
	g, gctx := errgroup.WithContext(baseCtx)
	g.SetLimit(foreachConcurrency)

	for i := range items {
		idx := i
		g.Go(func() error {
			select {
			case <-gctx.Done():
				return nil
			default:
			}

			item := items[idx]
			itemID := fmt.Sprintf("%s%04d", idPrefix, idx+1)

			// Build per-item payload (deep copy + placeholder substitution)
			itemPayload := deepCopyJSON(tplPayload).(map[string]any)
			if v := substituteItemPlaceholders(itemPayload, item); v != nil {
				mp, ok := v.(map[string]any)
				if !ok {
					mu.Lock()
					errResults = append(errResults, errOut{Item: item, Error: "template.payload not object after substitution"})
					mu.Unlock()
					return nil // swallow error to continue batch
				}
				itemPayload = mp
			}

			// Execute inner action with per-item timeout
			itemCtx, itemCancel := context.WithTimeout(gctx, perItemTimeout)
			defer itemCancel()

			out, err := dispatch(itemCtx, tplAction, itemPayload)

			mu.Lock()
			if err != nil {
				errResults = append(errResults, errOut{Item: item, Error: err.Error()})
				mu.Unlock()
				return nil // Keep iterating other items
			}
			okResults[idx] = out
			mu.Unlock()

			_ = itemID
			return nil
		})
	}

	_ = g.Wait() // Item errors are recorded, never abort the batch

	compact := make([]okOut, 0, len(okResults))
	for _, r := range okResults {
		if r != nil {
			compact = append(compact, r)
		}
	}
	bOK, _ := json.Marshal(compact)
	bErr, _ := json.Marshal(errResults)

	return map[string]any{
		"results_json": string(bOK),
		"errors_json":  string(bErr),
	}, nil
}

func dispatch(ctx context.Context, fullAction string, payload map[string]any) (map[string]any, error) {
	parts := strings.Split(fullAction, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid action name %q; expected category.operation", fullAction)
	}
	category, op := parts[0], parts[1]
	switch category {
	case "system":
		return system.HandleSystemAction(ctx, op, payload)
	case "web":
		return web.HandleWebAction(ctx, op, payload)
	case "html":
		return html.HandleHtmlAction(ctx, op, payload)
	case "llm":
		return llm.HandleLlmAction(ctx, op, payload)
	case "test":
		return test.HandleTestAction(ctx, op, payload)
	case "url":
		return url.HandleURLAction(ctx, op, payload)
	case "list":
		return list.HandleListAction(ctx, op, payload)
	case "flow":
		return nil, errors.New("flow.foreach does not support nesting flow actions")
	default:
		return nil, fmt.Errorf("unknown action category: %s", category)
	}
}

// Accept string (JSON array) or a slice; return []any.
func coerceToSlice(v any) ([]any, error) {
	if v == nil {
		return []any{}, nil
	}
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return []any{}, nil
		}
		var arr []any
		if err := json.Unmarshal([]byte(t), &arr); err != nil {
			return nil, err
		}
		return arr, nil
	case []any:
		return t, nil
	case []string:
		out := make([]any, len(t))
		for i := range t {
			out[i] = t[i]
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected items_json to be JSON array string or slice, got %T", v)
	}
}

// Deep copy arbitrary JSON-like value via marshal/unmarshal.
func deepCopyJSON(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

var itemRe = regexp.MustCompile(`\{\{\s*item(?:\.([a-zA-Z0-9_\.]+))?\s*\}\}`)

// Recursively replace "{{item}}" and "{{item.field}}" in all string leaves.
func substituteItemPlaceholders(v any, item any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = substituteItemPlaceholders(val, item)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = substituteItemPlaceholders(val, item)
		}
		return t
	case string:
		return replaceItemPlaceholdersInString(t, item)
	default:
		return v
	}
}

func replaceItemPlaceholdersInString(s string, item any) string {
	return itemRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := itemRe.FindStringSubmatch(match)
		if len(sub) == 0 {
			return match
		}
		path := strings.TrimSpace(sub[1])
		if path == "" {
			return fmt.Sprintf("%v", item)
		}
		val, ok := getByPath(item, path)
		if !ok {
			return ""
		}
		return fmt.Sprintf("%v", val)
	})
}

// Support simple dotted paths for map[string]any / nested objects / arrays by numeric index.
func getByPath(root any, path string) (any, bool) {
	cur := root
	parts := strings.Split(path, ".")
	for _, p := range parts {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[p]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			if n, err := strconv.Atoi(p); err == nil {
				if n < 0 || n >= len(node) {
					return nil, false
				}
				cur = node[n]
				continue
			}
			return nil, false
		default:
			return nil, false
		}
	}
	return cur, true
}
