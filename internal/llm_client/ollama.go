package llm_client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/ollama/ollama/api"
)

type ollamaProvider struct {
	client *api.Client
	model  string
}

const ollamaDefault = "phi4:latest"

func (p *ollamaProvider) Init(cfg Config) error {
	c, err := api.ClientFromEnvironment()
	if err != nil {
		host := cfg.OllamaHost
		if host == "" {
			host = "http://localhost:11434"
		}
		if host == "" {
			host = os.Getenv("OLLAMA_HOST")
		}
		u, uerr := url.Parse(host)
		if uerr != nil {
			return fmt.Errorf("ollama: bad host %q: %w", host, uerr)
		}
		c = api.NewClient(u, nil)
	}
	p.client = c
	if strings.TrimSpace(cfg.Model) != "" {
		p.model = cfg.Model
	} else {
		p.model = ollamaDefault
	}
	return nil
}

func (p *ollamaProvider) DefaultModel() string { return ollamaDefault }

func (p *ollamaProvider) AllowedModelOrDefault(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return p.model
	}
	return m
}

func (p *ollamaProvider) Generate(ctx context.Context, prompt, model string) (string, error) {
	if p.client == nil {
		return "", ErrNotInitialized
	}
	stream := false
	req := &api.GenerateRequest{
		Model:  p.AllowedModelOrDefault(model),
		Prompt: prompt,
		Stream: &stream,
	}
	var out strings.Builder
	if err := p.client.Generate(ctx, req, func(gr api.GenerateResponse) error {
		out.WriteString(gr.Response)
		return nil
	}); err != nil {
		return "", fmt.Errorf("ollama generate: %w", err)
	}
	return out.String(), nil
}

func (p *ollamaProvider) GenerateJSON(ctx context.Context, prompt, model string, schema any) (string, error) {
	if p.client == nil {
		return "", ErrNotInitialized
	}
	// Force JSON output. If schema supplied, pass it; else "json".
	var fmtRaw json.RawMessage
	if schema != nil {
		b, err := json.Marshal(schema)
		if err != nil {
			return "", fmt.Errorf("ollama marshal schema: %w", err)
		}
		fmtRaw = b
	} else {
		fmtRaw = json.RawMessage(`"json"`)
	}

	stream := false
	req := &api.GenerateRequest{
		Model:  p.AllowedModelOrDefault(model),
		Prompt: prompt + "\n\nReturn ONLY strict JSON. No extra text.",
		Format: fmtRaw,
		Stream: &stream,
	}
	var out strings.Builder
	if err := p.client.Generate(ctx, req, func(gr api.GenerateResponse) error {
		out.WriteString(gr.Response)
		return nil
	}); err != nil {
		return "", fmt.Errorf("ollama generate json: %w", err)
	}
	return out.String(), nil
}
