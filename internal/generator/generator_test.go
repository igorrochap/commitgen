package generator

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"text/template"

	"github.com/igorrochap/commitgen/internal/prompts"
)

func TestPromptContext(t *testing.T) {
	tests := []struct {
		name        string
		context     string
		wantContext bool
	}{
		{
			name:        "with context",
			context:     "fix CI failure",
			wantContext: true,
		},
		{
			name: "without context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := getPrompt("en")
			if err != nil {
				t.Fatalf("getPrompt() error = %v", err)
			}
			tmpl, err := template.New("prompt").Parse(prompt)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, promptData{
				Context: tt.context,
				Diff:    "example diff",
			}); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			got := buf.String()
			hasContext := strings.Contains(got, "## Additional context")
			if hasContext != tt.wantContext {
				t.Fatalf("context section presence = %t, want %t", hasContext, tt.wantContext)
			}
			if tt.wantContext && !strings.Contains(got, tt.context) {
				t.Fatalf("prompt does not contain context %q", tt.context)
			}
		})
	}
}

func TestGenerateTextOpenAI(t *testing.T) {
	var gotAuth string
	var gotBody struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}
	withProviderLoading(t, func(done <-chan struct{}) func() {
		return func() {
			<-done
		}
	})
	withProviderHTTPClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		return jsonResponse(http.StatusOK, `{"output_text":"feat: add provider selection"}`), nil
	}))

	withProviderEndpoint(t, "openai", "https://example.test/v1/responses")

	got, err := generateText(providerOptions{
		Provider: "openai",
		Model:    "gpt-5.5",
		APIKey:   "sk-test",
	}, "prompt")
	if err != nil {
		t.Fatalf("generateText() error = %v", err)
	}

	if got != "feat: add provider selection" {
		t.Fatalf("generateText() = %q, want feat: add provider selection", got)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q, want Bearer sk-test", gotAuth)
	}
	if gotBody.Model != "gpt-5.5" {
		t.Fatalf("Model = %q, want gpt-5.5", gotBody.Model)
	}
	if gotBody.Input != "prompt" {
		t.Fatalf("Input = %q, want prompt", gotBody.Input)
	}
}

func TestGenerateTextAnthropic(t *testing.T) {
	var gotKey string
	var gotBody struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	withProviderLoading(t, func(done <-chan struct{}) func() {
		return func() {
			<-done
		}
	})
	withProviderHTTPClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotKey = r.Header.Get("x-api-key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		return jsonResponse(http.StatusOK, `{"content":[{"type":"text","text":"fix: support claude"}]}`), nil
	}))

	got, err := generateText(providerOptions{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-5",
		APIKey:   "anthropic-key",
	}, "prompt")
	if err != nil {
		t.Fatalf("generateText() error = %v", err)
	}

	if got != "fix: support claude" {
		t.Fatalf("generateText() = %q, want fix: support claude", got)
	}
	if gotKey != "anthropic-key" {
		t.Fatalf("x-api-key = %q, want anthropic-key", gotKey)
	}
	if gotBody.Model != "claude-sonnet-4-5" {
		t.Fatalf("Model = %q, want claude-sonnet-4-5", gotBody.Model)
	}
	if len(gotBody.Messages) != 1 || gotBody.Messages[0].Content != "prompt" {
		t.Fatalf("Messages = %+v, want single prompt message", gotBody.Messages)
	}
}

func TestGenerateTextGemini(t *testing.T) {
	var gotURL string
	var gotBody struct {
		Contents []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
	}
	withProviderLoading(t, func(done <-chan struct{}) func() {
		return func() {
			<-done
		}
	})
	withProviderHTTPClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		return jsonResponse(http.StatusOK, `{"candidates":[{"content":{"parts":[{"text":"chore: support gemini"}]}}]}`), nil
	}))
	withProviderEndpoint(t, "gemini", "https://example.test/v1beta/models")

	got, err := generateText(providerOptions{
		Provider: "gemini",
		Model:    "gemini-3.5-flash",
		APIKey:   "gemini-key",
	}, "prompt")
	if err != nil {
		t.Fatalf("generateText() error = %v", err)
	}

	if got != "chore: support gemini" {
		t.Fatalf("generateText() = %q, want chore: support gemini", got)
	}
	if !strings.Contains(gotURL, "/gemini-3.5-flash:generateContent") {
		t.Fatalf("URL = %q, want generateContent model path", gotURL)
	}
	if !strings.Contains(gotURL, "key=gemini-key") {
		t.Fatalf("URL = %q, want API key query", gotURL)
	}
	if len(gotBody.Contents) != 1 || len(gotBody.Contents[0].Parts) != 1 || gotBody.Contents[0].Parts[0].Text != "prompt" {
		t.Fatalf("Contents = %+v, want single prompt part", gotBody.Contents)
	}
}

func TestGenerateCommitUsesSelectedProvider(t *testing.T) {
	var gotBody struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}
	withProviderLoading(t, func(done <-chan struct{}) func() {
		return func() {
			<-done
		}
	})
	withProviderHTTPClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		return jsonResponse(http.StatusOK, `{"output_text":"feat: wire provider"}`), nil
	}))

	tmpl := template.Must(template.New("prompt").Parse("context={{.Context}} diff={{.Diff}}"))
	got, err := generateCommit(tmpl, "diff", providerOptions{
		Provider: "openai",
		Model:    "gpt-5.5",
		APIKey:   "sk-test",
	}, "extra")
	if err != nil {
		t.Fatalf("generateCommit() error = %v", err)
	}

	if got != "feat: wire provider" {
		t.Fatalf("generateCommit() = %q, want feat: wire provider", got)
	}
	if gotBody.Model != "gpt-5.5" {
		t.Fatalf("Model = %q, want gpt-5.5", gotBody.Model)
	}
	if gotBody.Input != "context=extra diff=diff" {
		t.Fatalf("Input = %q, want rendered prompt", gotBody.Input)
	}
}

func TestGenerateTextStartsLoadingForHostedProvider(t *testing.T) {
	started := 0
	waited := 0
	withProviderLoading(t, func(done <-chan struct{}) func() {
		started++
		return func() {
			<-done
			waited++
		}
	})
	withProviderHTTPClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: use shared loading"}`), nil
	}))

	_, err := generateText(providerOptions{
		Provider: "openai",
		Model:    "gpt-5.5",
		APIKey:   "sk-test",
	}, "prompt")
	if err != nil {
		t.Fatalf("generateText() error = %v", err)
	}

	if started != 1 {
		t.Fatalf("loading starts = %d, want 1", started)
	}
	if waited != 1 {
		t.Fatalf("loading waits = %d, want 1", waited)
	}
}

func withProviderEndpoint(t *testing.T, provider, endpoint string) {
	t.Helper()
	original := providerEndpoints[provider]
	providerEndpoints[provider] = endpoint
	t.Cleanup(func() {
		providerEndpoints[provider] = original
	})
}

func withProviderLoading(t *testing.T, start func(<-chan struct{}) func()) {
	t.Helper()
	original := startLoading
	startLoading = start
	t.Cleanup(func() {
		startLoading = original
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func withProviderHTTPClient(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	original := providerHTTPClient
	providerHTTPClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		providerHTTPClient = original
	})
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestPRPromptContext(t *testing.T) {
	prompt, ok := prompts.GetPR("en")
	if !ok {
		t.Fatal("GetPR() ok = false, want true")
	}
	tmpl, err := template.New("pr").Parse(prompt)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	const context = "closes the issue #15 on github"
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, promptData{
		Context: context,
		Diff:    "example commit log",
	}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "## Additional context") {
		t.Fatal("prompt does not contain additional context section")
	}
	if !strings.Contains(got, context) {
		t.Fatalf("prompt does not contain context %q", context)
	}
}
