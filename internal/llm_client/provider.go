package llm_client

import (
	"context"
	"fmt"
	"strings"
)

type Config struct {
	Backend    string
	Model      string
	OllamaHost string
}

type Provider interface {
	Init(cfg Config) error
	DefaultModel() string
	AllowedModelOrDefault(model string) string
	Generate(ctx context.Context, prompt, model string) (string, error)
	GenerateJSON(ctx context.Context, prompt, model string, schema any) (string, error)
}

var (
	active   Provider
	activeID string
)

func Init(cfg Config) error {
	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))
	if backend == "" {
		backend = "gemini"
	}
	var p Provider
	switch backend {
	case "ollama":
		p = &ollamaProvider{}
		activeID = "ollama"
	case "gemini":
		p = &geminiProvider{}
		activeID = "gemini"
	default:
		return fmt.Errorf("unsupported LLM backend: %s", backend)
	}
	if err := p.Init(cfg); err != nil {
		return err
	}
	active = p
	return nil
}

func ActiveBackend() string {
	if active == nil {
		return ""
	}
	return activeID
}

func AllowedModelOrDefault(m string) string {
	return active.AllowedModelOrDefault(m)
}

func Generate(ctx context.Context, prompt, model string) (string, error) {
	if active == nil {
		return "", ErrNotInitialized
	}
	return active.Generate(ctx, prompt, model)
}

func GenerateJSON(ctx context.Context, prompt, model string, schema any) (string, error) {
	if active == nil {
		return "", ErrNotInitialized
	}
	return active.GenerateJSON(ctx, prompt, model, schema)
}
