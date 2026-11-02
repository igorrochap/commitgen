package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/igorrochap/commitgen/internal/loading"
)

var providerEndpoints = map[string]string{
	"openai":    "https://api.openai.com/v1/responses",
	"anthropic": "https://api.anthropic.com/v1/messages",
	"gemini":    "https://generativelanguage.googleapis.com/v1beta/models",
}

var providerHTTPClient = http.DefaultClient

var startLoading = loading.Start
var startLoadingWithNotices = loading.StartWithNotices

var (
	retrySleep  = time.Sleep
	retryJitter = fullJitter
)

const (
	maxHostedAttempts = 4
	baseRetryDelay    = time.Second
	maxRetryDelay     = 30 * time.Second
)

var transportURL = regexp.MustCompile(`https?://[^\s]+`)

type providerOptions struct {
	Provider string
	Model    string
	APIKey   string
	Agent    bool
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
	name        string
	model       string
	apiKey      string
	endpoint    string
	client      *http.Client
	codec       providerCodec
	retryNotice func(string)
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

	var done chan struct{}
	var retryNotices chan string
	var wait func()
	if !opts.Agent {
		done = make(chan struct{})
		retryNotices = make(chan string, maxHostedAttempts-1)
		if hosted, ok := provider.(hostedProvider); ok {
			hosted.retryNotice = func(notice string) {
				retryNotices <- notice
			}
			provider = hosted
		}
		wait = startLoadingWithNotices(done, retryNotices)
	}
	result, err := provider.Generate(prompt)

	if !opts.Agent {
		close(retryNotices)
		close(done)
		wait()
	}

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
	defer closeBody(resp.Body)

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
	body, err := p.buildRequestBody(prompt)
	if err != nil {
		return "", err
	}

	for attempt := 1; attempt <= maxHostedAttempts; attempt++ {
		resp, err := p.sendRequest(body)
		if err != nil {
			finished, failure := p.retryAfterTransportFailure(attempt, resp, err)
			if finished {
				return "", failure
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			finished, failure := p.retryAfterStatusFailure(attempt, resp)
			if finished {
				return "", failure
			}
			continue
		}

		text, finished, err := p.handleSuccessfulResponse(attempt, resp)
		if finished {
			return text, err
		}
	}

	return "", fmt.Errorf("%s: request failed after %d attempts", p.name, maxHostedAttempts)
}

func (p hostedProvider) buildRequestBody(prompt string) ([]byte, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return nil, fmt.Errorf("%s: api key is required", p.name)
	}

	payload, err := p.codec.BuildRequest(p.model, prompt)
	if err != nil {
		return nil, err
	}

	return json.Marshal(payload)
}

func (p hostedProvider) handleSuccessfulResponse(attempt int, resp *http.Response) (string, bool, error) {
	text, retryable, err := p.parseSuccessfulResponse(resp, attempt)
	if err == nil {
		return text, true, nil
	}
	if !retryable {
		return "", true, err
	}
	finished, failure := p.retryAfterTransportFailure(attempt, nil, err)
	if finished {
		return "", true, failure
	}
	return "", false, nil
}

func (p hostedProvider) parseSuccessfulResponse(resp *http.Response, attempt int) (string, bool, error) {
	reader := responseBodyReader{source: resp.Body}
	text, err := p.codec.ParseResponse(&reader)
	closeBody(resp.Body)
	if err == nil {
		return strings.TrimSpace(text), false, nil
	}
	if reader.transientError != nil {
		return "", true, reader.transientError
	}
	detail := transportErrorDetail(err, p.apiKey, p.endpointForModel())
	return "", false, fmt.Errorf("%s: request failed after %s: response could not be parsed: %s", p.name, describeAttempts(attempt), detail)
}

type responseBodyReader struct {
	source         io.Reader
	transientError error
}

func (reader *responseBodyReader) Read(buffer []byte) (int, error) {
	n, err := reader.source.Read(buffer)
	if !errors.Is(err, io.EOF) {
		if _, transient := transientTransportDescription(err); transient {
			reader.transientError = err
		}
	}
	return n, err
}

func (p hostedProvider) retryAfterTransportFailure(attempt int, resp *http.Response, err error) (bool, error) {
	closeResponse(resp)
	description, transient := transientTransportDescription(err)
	if !transient || attempt == maxHostedAttempts {
		return true, p.transportFailure(attempt, err)
	}
	p.retry(attempt, description)
	return false, nil
}

func (p hostedProvider) retryAfterStatusFailure(attempt int, resp *http.Response) (bool, error) {
	responseBody, _ := io.ReadAll(resp.Body)
	closeBody(resp.Body)
	failure := statusFailure{code: resp.StatusCode, body: p.redact(string(responseBody))}
	if !isTransientStatus(failure.code) || attempt == maxHostedAttempts {
		return true, p.statusFailure(attempt, failure)
	}
	p.retry(attempt, failure.statusDescription())
	return false, nil
}

func (p hostedProvider) sendRequest(body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, p.endpointForModel(), bytes.NewReader(body))
	if err != nil {
		detail := transportErrorDetail(err, p.apiKey, p.endpointForModel())
		return nil, fmt.Errorf("%s: create request: %s", p.name, detail)
	}
	req.Header.Set("Content-Type", "application/json")
	p.codec.DecorateRequest(req, p.apiKey)
	return p.client.Do(req)
}

func (p hostedProvider) retry(failedAttempt int, description string) {
	delayLimit := retryDelayLimit(failedAttempt)
	delay := retryJitter(delayLimit)
	notice := fmt.Sprintf("%s: %s, retrying in %.1fs (attempt %d/%d)",
		p.name, description, delay.Seconds(), failedAttempt+1, maxHostedAttempts)
	if p.retryNotice != nil {
		p.retryNotice(notice)
	} else {
		fmt.Fprintln(os.Stderr, notice)
	}
	retrySleep(delay)
}

func retryDelayLimit(retryNumber int) time.Duration {
	delay := baseRetryDelay
	for i := 1; i < retryNumber && delay < maxRetryDelay; i++ {
		if delay > maxRetryDelay/2 {
			return maxRetryDelay
		}
		delay *= 2
	}
	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
}

func fullJitter(upperBound time.Duration) time.Duration {
	return time.Duration(rand.Int63n(int64(upperBound) + 1))
}

func isTransientStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func transientTransportDescription(err error) (string, bool) {
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return "timeout", true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return "connection reset", true
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection refused", true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "unexpected end of response", true
	}
	return "", false
}

func (p hostedProvider) transportFailure(attempts int, err error) error {
	detail := transportErrorDetail(err, p.apiKey, p.endpointForModel())
	return fmt.Errorf("%s: request failed after %s: transport error: %s", p.name, describeAttempts(attempts), detail)
}

func transportErrorDetail(err error, apiKey, endpoint string) string {
	var requestError *url.Error
	if errors.As(err, &requestError) {
		err = requestError.Err
	}
	detail := strings.TrimSpace(err.Error())
	detail = redactAPIKey(detail, apiKey)
	if endpoint != "" {
		detail = strings.ReplaceAll(detail, endpoint, "provider endpoint")
	}
	detail = transportURL.ReplaceAllString(detail, "provider endpoint")
	return detail
}

func (p hostedProvider) statusFailure(attempts int, failure statusFailure) error {
	description := failure.statusDescription()
	if failure.body != "" {
		description += ": " + strings.TrimSpace(failure.body)
	}
	return fmt.Errorf("%s: request failed after %s: %s", p.name, describeAttempts(attempts), description)
}

func describeAttempts(attempts int) string {
	if attempts == 1 {
		return "1 attempt"
	}
	return fmt.Sprintf("%d attempts", attempts)
}

func (p hostedProvider) redact(value string) string {
	return redactAPIKey(value, p.apiKey)
}

func redactAPIKey(value, apiKey string) string {
	if apiKey == "" {
		return value
	}
	value = strings.ReplaceAll(value, apiKey, "[redacted]")
	escapedAPIKey := url.QueryEscape(apiKey)
	if escapedAPIKey != apiKey {
		value = strings.ReplaceAll(value, escapedAPIKey, "[redacted]")
	}
	return value
}

type statusFailure struct {
	code int
	body string
}

func (failure statusFailure) statusDescription() string {
	return fmt.Sprintf("HTTP %d %s", failure.code, http.StatusText(failure.code))
}

func closeResponse(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		closeBody(resp.Body)
	}
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

func (geminiCodec) BuildRequest(_ string, prompt string) (any, error) {
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

func (geminiCodec) DecorateRequest(_ *http.Request, _ string) {}

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
