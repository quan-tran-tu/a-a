package llm_client

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

type geminiProvider struct {
	client *genai.Client
	model  string
}

const geminiDefault = "gemini-2.0-flash"

func (p *geminiProvider) Init(cfg Config) error {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY is not set")
	}
	c, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("gemini client init: %w", err)
	}
	p.client = c
	if strings.TrimSpace(cfg.Model) != "" {
		p.model = cfg.Model
	} else {
		p.model = geminiDefault
	}
	return nil
}

func (p *geminiProvider) DefaultModel() string { return geminiDefault }

func (p *geminiProvider) AllowedModelOrDefault(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return p.model
	}
	if !strings.HasPrefix(strings.ToLower(model), "gemini-") {
		return geminiDefault
	}

	return m
}

func (p *geminiProvider) Generate(ctx context.Context, prompt, model string) (string, error) {
	if p.client == nil {
		return "", ErrNotInitialized
	}
	m := p.AllowedModelOrDefault(model)
	resp, err := p.client.Models.GenerateContent(ctx, m, genai.Text(prompt), nil)
	if err != nil {
		return "", fmt.Errorf("gemini generate: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty response")
	}
	return resp.Candidates[0].Content.Parts[0].Text, nil
}

func (p *geminiProvider) GenerateJSON(ctx context.Context, prompt, model string, schema any) (string, error) {
	if p.client == nil {
		return "", ErrNotInitialized
	}
	m := p.AllowedModelOrDefault(model)
	cfg := &genai.GenerateContentConfig{
		// Force JSON output in candidates
		ResponseMIMEType: "application/json",
	}
	if schema != nil {
		cfg.ResponseJsonSchema = schema
	}
	resp, err := p.client.Models.GenerateContent(ctx, m, genai.Text(prompt), cfg)
	if err != nil {
		return "", fmt.Errorf("gemini generate json: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty json response")
	}
	return resp.Candidates[0].Content.Parts[0].Text, nil
}
