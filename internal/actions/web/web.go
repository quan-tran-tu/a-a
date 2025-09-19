package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"a-a/internal/utils"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

type httpResp struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Content    string `json:"content"`
}

func doRequest(ctx context.Context, method, url string, headers map[string]string) (*httpResp, error) {
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return &httpResp{URL: url, StatusCode: resp.StatusCode, Content: string(b)}, nil
}

func handleRequest(ctx context.Context, payload map[string]any) (map[string]any, error) {
	url, err := utils.GetStringPayload(payload, "url")
	if err != nil {
		return nil, err
	}
	method, _ := payload["method"].(string)
	hdrs := map[string]string{}
	if m, ok := payload["headers"].(map[string]any); ok {
		for k, v := range m {
			if sv, ok := v.(string); ok {
				hdrs[k] = sv
			}
		}
	}
	r, err := doRequest(ctx, method, url, hdrs)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"url":         r.URL,
		"status_code": r.StatusCode,
		"content":     r.Content,
	}, nil
}

func handleBatchRequest(ctx context.Context, payload map[string]any) (map[string]any, error) {
	urlsJSON, err := utils.GetStringPayload(payload, "urls_json")
	if err != nil {
		return nil, err
	}
	var urls []string
	if err := json.Unmarshal([]byte(urlsJSON), &urls); err != nil {
		return nil, fmt.Errorf("urls_json must be JSON array of strings: %w", err)
	}
	conc := 5
	if v, ok := payload["concurrency"]; ok {
		if i, err := utils.GetIntPayload(map[string]any{"v": v}, "v"); err == nil && i > 0 {
			conc = i
		}
	}

	type job struct{ u string }
	jobs := make(chan job, len(urls))
	results := make(chan *httpResp, len(urls))

	worker := func() {
		for j := range jobs {
			r, err := doRequest(ctx, "GET", j.u, nil)
			if err != nil {
				results <- &httpResp{URL: j.u, StatusCode: 0, Content: fmt.Sprintf("ERROR: %v", err)}
				continue
			}
			results <- r
		}
	}

	for i := 0; i < conc; i++ {
		go worker()
	}
	for _, u := range urls {
		jobs <- job{u: u}
	}
	close(jobs)

	out := make([]*httpResp, 0, len(urls))
	for i := 0; i < len(urls); i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-results:
			out = append(out, r)
		}
	}
	b, _ := json.Marshal(out)
	return map[string]any{"responses_json": string(b)}, nil
}

func HandleWebAction(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "request":
		return handleRequest(ctx, payload)
	case "batch_request":
		return handleBatchRequest(ctx, payload)
	default:
		return nil, fmt.Errorf("unknown web operation: %s", operation)
	}
}
