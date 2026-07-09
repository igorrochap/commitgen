package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/igorrochap/commitgen/internal/loading"
)

var providerEndpoints = map[string]string{
	"openai":    "https://api.openai.com/v1/responses",
	"anthropic": "https://api.anthropic.com/v1/messages",
	"gemini":    "https://generativelanguage.googleapis.com/v1beta/models",
}

var providerHTTPClient = http.DefaultClient

var startLoading = loading.Start

type providerOptions struct {
	Provider string
	Model    string
	APIKey   string
}

type textProvider interface {
	Generate(prompt string) (string, error)
}

var (
	_ textProvider = ollamaProvider{}
	_ textProvider = hostedProvider{}
)

type ollamaProvider struct {
	model string
}

type hostedProvider struct {
	name     string
	model    string
	apiKey   string
	endpoint string
	client   *http.Client
	codec    providerCodec
}

type providerCodec interface {
	BuildRequest(model, prompt string) (any, error)
	DecorateRequest(req *http.Request, apiKey string)
	ParseResponse(body io.Reader) (string, error)
}

func generateText(opts providerOptions, prompt string) (string, error) {
	provider, err := newTextProvider(opts)
	if err != nil {
		return "", err
	}

	done := make(chan struct{})
	wait := startLoading(done)

	result, err := provider.Generate(prompt)

	close(done)
	wait()

	if err != nil {
		return "", err
	}
	return result, nil
}

func newTextProvider(opts providerOptions) (textProvider, error) {
	switch opts.Provider {
	case "ollama":
		return ollamaProvider{model: opts.Model}, nil
	case "openai":
		return newHostedProvider("openai", opts, openAICodec{}), nil
	case "anthropic":
		return newHostedProvider("anthropic", opts, anthropicCodec{}), nil
	case "gemini":
		return newHostedProvider("gemini", opts, geminiCodec{}), nil
	default:
		return nil, fmt.Errorf("provider %s not supported", opts.Provider)
	}
}

func newHostedProvider(name string, opts providerOptions, codec providerCodec) hostedProvider {
	return hostedProvider{
		name:     name,
		model:    opts.Model,
		apiKey:   opts.APIKey,
		endpoint: providerEndpoints[name],
		client:   providerHTTPClient,
		codec:    codec,
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Think  bool   `json:"think"`
}

type ollamaStreamChunk struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error"`
}

func (p ollamaProvider) Generate(prompt string) (string, error) {
	body, err := json.Marshal(ollamaRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: true,
		Think:  false,
	})
	if err != nil {
		return "", err
	}

	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(body))

	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama: %s", strings.TrimSpace(string(b)))
	}

	var result strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk ollamaStreamChunk
		if err := decoder.Decode(&chunk); err != nil {
			break
		}
		if chunk.Error != "" {
			return "", fmt.Errorf("ollama: %s", chunk.Error)
		}
		result.WriteString(chunk.Response)
		if chunk.Done {
			break
		}
	}
	return result.String(), nil
}

func (p hostedProvider) Generate(prompt string) (string, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return "", fmt.Errorf("%s: api key is required", p.name)
	}

	payload, err := p.codec.BuildRequest(p.model, prompt)
	if err != nil {
		return "", err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, p.endpointForModel(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	p.codec.DecorateRequest(req, p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: %w", p.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%s: %s", p.name, strings.TrimSpace(string(b)))
	}

	text, err := p.codec.ParseResponse(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%s: %w", p.name, err)
	}
	return strings.TrimSpace(text), nil
}

func (p hostedProvider) endpointForModel() string {
	if p.name != "gemini" {
		return p.endpoint
	}
	return strings.TrimRight(p.endpoint, "/") + "/" + p.model + ":generateContent?key=" + p.apiKey
}

type openAICodec struct{}

func (openAICodec) BuildRequest(model, prompt string) (any, error) {
	return map[string]string{
		"model": model,
		"input": prompt,
	}, nil
}

func (openAICodec) DecorateRequest(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

func (openAICodec) ParseResponse(body io.Reader) (string, error) {
	var result struct {
		OutputText string `json:"output_text"`
	}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return "", err
	}
	return result.OutputText, nil
}

type anthropicCodec struct{}

func (anthropicCodec) BuildRequest(model, prompt string) (any, error) {
	return map[string]any{
		"model":      model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}, nil
}

func (anthropicCodec) DecorateRequest(req *http.Request, apiKey string) {
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
}

func (anthropicCodec) ParseResponse(body io.Reader) (string, error) {
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return "", err
	}

	var text strings.Builder
	for _, content := range result.Content {
		text.WriteString(content.Text)
	}
	return text.String(), nil
}

type geminiCodec struct{}

func (geminiCodec) BuildRequest(model, prompt string) (any, error) {
	return map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}, nil
}

func (geminiCodec) DecorateRequest(req *http.Request, apiKey string) {}

func (geminiCodec) ParseResponse(body io.Reader) (string, error) {
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return "", err
	}

	var text strings.Builder
	for _, candidate := range result.Candidates {
		for _, part := range candidate.Content.Parts {
			text.WriteString(part.Text)
		}
	}
	return text.String(), nil
}
