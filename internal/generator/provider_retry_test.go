package generator

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestHostedProvidersRetryServiceUnavailable(t *testing.T) {
	tests := []struct {
		provider string
		body     string
		text     string
	}{
		{provider: "openai", body: `{"output_text":"feat: retry openai"}`, text: "feat: retry openai"},
		{provider: "anthropic", body: `{"content":[{"type":"text","text":"feat: retry anthropic"}]}`, text: "feat: retry anthropic"},
		{provider: "gemini", body: `{"candidates":[{"content":{"parts":[{"text":"feat: retry gemini"}]}}]}`, text: "feat: retry gemini"},
	}

	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })

	for _, test := range tests {
		t.Run(test.provider, func(t *testing.T) {
			assertProviderRetriesServiceUnavailable(t, test.provider, test.body, test.text)
		})
	}
}

func assertProviderRetriesServiceUnavailable(t *testing.T, provider, responseBody, wantText string) {
	t.Helper()
	withProviderEndpoint(t, provider, "https://example.test/models")
	requests := 0
	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonResponse(http.StatusServiceUnavailable, "overloaded"), nil
		}
		return jsonResponse(http.StatusOK, responseBody), nil
	}))

	var got string
	var generateErr error
	_, stderr := captureStandardStreams(t, func() {
		got, generateErr = generateText(providerOptions{
			Provider: provider,
			Model:    "test-model",
			APIKey:   "test-key",
			Agent:    true,
		}, "prompt")
	})
	if generateErr != nil {
		t.Fatalf("generateText() error = %v", generateErr)
	}
	if got != wantText || requests != 2 {
		t.Fatalf("generateText() = %q after %d requests, want %q after 2", got, requests, wantText)
	}
	if !strings.Contains(stderr, fmt.Sprintf("%s: HTTP 503 Service Unavailable", provider)) {
		t.Fatalf("stderr = %q, want retry notice for provider and status", stderr)
	}
}

func TestHostedProviderRetriesTransientStatusCodes(t *testing.T) {
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	for _, status := range []int{
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			requests := 0
			text, stderr, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return jsonResponse(status, "temporary failure"), nil
				}
				return jsonResponse(http.StatusOK, `{"output_text":"feat: recovered"}`), nil
			}))
			if err != nil {
				t.Fatalf("generateText() error = %v", err)
			}
			if text != "feat: recovered" || requests != 2 {
				t.Fatalf("generateText() = %q after %d requests, want success after 2", text, requests)
			}
			if !strings.Contains(stderr, fmt.Sprintf("HTTP %d", status)) {
				t.Fatalf("stderr = %q, want status %d", stderr, status)
			}
		})
	}
}

func TestHostedProviderHonorsRetryAfter(t *testing.T) {
	now := time.Date(2026, time.October, 21, 7, 27, 50, 0, time.UTC)
	tests := []retryAfterCase{
		{name: "delay seconds", status: http.StatusServiceUnavailable, retryAfter: "5", includeHeader: true, wantDelay: 5 * time.Second},
		{name: "future HTTP date", status: http.StatusTooManyRequests, retryAfter: "Wed, 21 Oct 2026 07:28:00 GMT", includeHeader: true, wantDelay: 10 * time.Second},
		{name: "capped delay", status: http.StatusServiceUnavailable, retryAfter: "120", includeHeader: true, wantDelay: maxRetryDelay},
		{name: "overflow delay", status: http.StatusServiceUnavailable, retryAfter: "18446744073709551616", includeHeader: true, wantDelay: maxRetryDelay},
		{name: "zero delay", status: http.StatusServiceUnavailable, retryAfter: "0", includeHeader: true},
		{name: "invalid delay", status: http.StatusServiceUnavailable, retryAfter: "soon", includeHeader: true, wantDelay: 250 * time.Millisecond, wantJitter: true},
		{name: "past HTTP date", status: http.StatusServiceUnavailable, retryAfter: "Wed, 21 Oct 2026 07:27:49 GMT", includeHeader: true, wantDelay: 250 * time.Millisecond, wantJitter: true},
		{name: "missing header", status: http.StatusServiceUnavailable, wantDelay: 250 * time.Millisecond, wantJitter: true},
		{name: "empty header", status: http.StatusServiceUnavailable, includeHeader: true, wantDelay: 250 * time.Millisecond, wantJitter: true},
		{name: "negative delay", status: http.StatusServiceUnavailable, retryAfter: "-5", includeHeader: true, wantDelay: 250 * time.Millisecond, wantJitter: true},
		{name: "plus sign", status: http.StatusServiceUnavailable, retryAfter: "+5", includeHeader: true, wantDelay: 250 * time.Millisecond, wantJitter: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertHostedProviderHonorsRetryAfter(t, test, now)
		})
	}
}

type retryAfterCase struct {
	name          string
	status        int
	retryAfter    string
	includeHeader bool
	wantDelay     time.Duration
	wantJitter    bool
}

func assertHostedProviderHonorsRetryAfter(t *testing.T, test retryAfterCase, now time.Time) {
	t.Helper()
	var delays []time.Duration
	jitterCalls := 0
	withRetryTiming(t, func(delay time.Duration) {
		delays = append(delays, delay)
	}, func(limit time.Duration) time.Duration {
		jitterCalls++
		if limit != baseRetryDelay {
			t.Errorf("jitter limit = %s, want %s", limit, baseRetryDelay)
		}
		return 250 * time.Millisecond
	})

	requests := 0
	provider := newHostedProvider("openai", providerOptions{
		Provider: "openai",
		Model:    "test-model",
		APIKey:   "test-key",
	}, openAICodec{})
	provider.endpoint = "https://example.test/v1/responses"
	provider.now = func() time.Time { return now }
	provider.retryNotice = func(string) {}
	provider.client = &http.Client{Transport: retryAfterTestTransport(test, &requests)}

	text, err := provider.Generate("prompt")
	if err != nil || text != "feat: recovered" || requests != 2 {
		t.Fatalf("Generate() = %q, %v after %d requests, want success after 2", text, err, requests)
	}
	if len(delays) != 1 || delays[0] != test.wantDelay {
		t.Fatalf("retry delays = %v, want [%s]", delays, test.wantDelay)
	}
	wantJitterCalls := 0
	if test.wantJitter {
		wantJitterCalls = 1
	}
	if jitterCalls != wantJitterCalls {
		t.Fatalf("jitter calls = %d, want %d", jitterCalls, wantJitterCalls)
	}
}

func retryAfterTestTransport(test retryAfterCase, requests *int) roundTripFunc {
	return func(*http.Request) (*http.Response, error) {
		*requests++
		if *requests != 1 {
			return jsonResponse(http.StatusOK, `{"output_text":"feat: recovered"}`), nil
		}
		response := jsonResponse(test.status, "temporary failure")
		if test.includeHeader {
			response.Header.Set("Retry-After", test.retryAfter)
		}
		return response, nil
	}
}

func TestHostedProviderDoesNotRetryBadRequestWithRetryAfter(t *testing.T) {
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	requests := 0
	_, _, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		response := jsonResponse(http.StatusBadRequest, "invalid request")
		response.Header.Set("Retry-After", "5")
		return response, nil
	}))
	if err == nil || requests != 1 {
		t.Fatalf("generateText() error = %v after %d requests, want one failed request", err, requests)
	}
}

func TestHostedProviderRetryAfterDoesNotChangeAttemptLimit(t *testing.T) {
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	requests := 0
	_, _, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		response := jsonResponse(http.StatusServiceUnavailable, "overloaded")
		response.Header.Set("Retry-After", "1")
		return response, nil
	}))
	if err == nil || requests != maxHostedAttempts {
		t.Fatalf("generateText() error = %v after %d requests, want %d requests", err, requests, maxHostedAttempts)
	}
}

func TestHostedProviderDoesNotRetryFinalResponses(t *testing.T) {
	tests := []struct {
		name string
		code int
		body string
	}{
		{name: "bad request", code: http.StatusBadRequest, body: "invalid request"},
		{name: "unauthorized", code: http.StatusUnauthorized, body: "unauthorized"},
		{name: "forbidden", code: http.StatusForbidden, body: "forbidden"},
		{name: "not found", code: http.StatusNotFound, body: "not found"},
		{name: "unparseable success", code: http.StatusOK, body: "{"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			_, _, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests++
				return jsonResponse(test.code, test.body), nil
			}))
			if err == nil || requests != 1 {
				t.Fatalf("generateText() error = %v after %d requests, want one failed request", err, requests)
			}
			if test.code != http.StatusOK && !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", test.code)) {
				t.Fatalf("error = %q, want HTTP %d", err, test.code)
			}
			if !strings.Contains(err.Error(), "after 1 attempt") {
				t.Fatalf("error = %q, want one attempt", err)
			}
		})
	}
}

func TestHostedProviderRedactsRequestAndParseErrors(t *testing.T) {
	const apiKey = "secret-gemini-error-key"
	const endpoint = "https://example.test/v1beta/models"
	withProviderEndpoint(t, "gemini", endpoint)
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })

	requests := 0
	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(http.StatusOK, `{}`), nil
	}))
	provider := newHostedProvider("gemini", providerOptions{
		Provider: "gemini",
		Model:    "invalid\nmodel",
		APIKey:   apiKey,
	}, geminiCodec{})
	_, requestErr := provider.Generate("prompt")
	malformedURL := endpoint + "/invalid\nmodel:generateContent?key=" + apiKey
	assertSecretAbsent(t, requestErr, apiKey, malformedURL)
	if requests != 0 {
		t.Fatalf("requests = %d, want no request for malformed endpoint", requests)
	}

	parseURL := endpoint + "?key=" + apiKey
	provider = hostedProvider{
		name:     "gemini",
		model:    "test-model",
		apiKey:   apiKey,
		endpoint: parseURL,
		client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{}`), nil
		})},
		codec: exposingCodec{details: "could not parse response at " + parseURL},
	}
	_, parseErr := provider.Generate("prompt")
	assertSecretAbsent(t, parseErr, apiKey, parseURL)
}

func TestHostedProviderRetriesTransientResponseBodyReadErrors(t *testing.T) {
	tests := []struct {
		name        string
		failure     error
		description string
	}{
		{name: "unexpected end", failure: io.ErrUnexpectedEOF, description: "unexpected end of response"},
		{name: "connection reset", failure: syscall.ECONNRESET, description: "connection reset"},
	}
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertRetriesResponseReadError(t, test.failure, test.description)
		})
	}
}

func assertRetriesResponseReadError(t *testing.T, failure error, description string) {
	t.Helper()
	requests := 0
	failedBody := &failingReadCloser{
		reader: strings.NewReader(`{"output_text":"truncated"`),
		err:    failure,
	}
	text, stderr, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return &http.Response{StatusCode: http.StatusOK, Body: failedBody}, nil
		}
		if !failedBody.closed {
			t.Error("response body remained open before retry")
		}
		return jsonResponse(http.StatusOK, `{"output_text":"feat: recovered"}`), nil
	}))
	if err != nil || text != "feat: recovered" || requests != 2 {
		t.Fatalf("generateText() = %q, %v after %d requests, want success after 2", text, err, requests)
	}
	if !failedBody.closed || !strings.Contains(stderr, description) {
		t.Fatalf("closed = %t, stderr = %q, want closed body and %q notice", failedBody.closed, stderr, description)
	}
}

func TestHostedProviderStopsWhenRetryReturnsFinalStatus(t *testing.T) {
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	requests := 0
	_, stderr, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonResponse(http.StatusServiceUnavailable, "overloaded"), nil
		}
		return jsonResponse(http.StatusBadRequest, "invalid retry request"), nil
	}))
	if err == nil || requests != 2 {
		t.Fatalf("generateText() error = %v after %d requests, want final error after 2", err, requests)
	}
	for _, want := range []string{"after 2 attempts", "HTTP 400", "invalid retry request"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q", err, want)
		}
	}
	if got := strings.Count(stderr, "retrying in"); got != 1 {
		t.Fatalf("retry notices = %d, want 1: %q", got, stderr)
	}
}

func TestHostedProviderStopsAfterAttemptLimit(t *testing.T) {
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	requests := 0
	_, stderr, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(http.StatusServiceUnavailable, "last response body"), nil
	}))
	if err == nil || requests != maxHostedAttempts {
		t.Fatalf("generateText() error = %v after %d requests, want %d requests", err, requests, maxHostedAttempts)
	}
	for _, want := range []string{"openai", "after 4 attempts", "HTTP 503", "last response body"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q", err, want)
		}
	}
	if got := strings.Count(stderr, "retrying in"); got != maxHostedAttempts-1 {
		t.Fatalf("retry notices = %d, want %d: %q", got, maxHostedAttempts-1, stderr)
	}
	if strings.Contains(stderr, "\x1b[") {
		t.Fatalf("retry notice contains ANSI escape sequence: %q", stderr)
	}
}

func TestHostedProviderRetryNoticesNameEachNextAttempt(t *testing.T) {
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	requests := 0
	_, stderr, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		if requests < 3 {
			return jsonResponse(http.StatusServiceUnavailable, "overloaded"), nil
		}
		return jsonResponse(http.StatusOK, `{"output_text":"feat: recovered"}`), nil
	}))
	if err != nil || requests != 3 {
		t.Fatalf("generateText() error = %v after %d requests, want success after 3", err, requests)
	}
	notices := strings.Split(strings.TrimSpace(stderr), "\n")
	if len(notices) != 2 {
		t.Fatalf("retry notices = %q, want 2 lines", stderr)
	}
	for i, wantAttempt := range []string{"attempt 2/4", "attempt 3/4"} {
		for _, want := range []string{"openai", "HTTP 503", "0.0s", wantAttempt} {
			if !strings.Contains(notices[i], want) {
				t.Errorf("notice %d = %q, want %q", i+1, notices[i], want)
			}
		}
		if strings.Contains(notices[i], "\x1b[") {
			t.Errorf("notice %d contains ANSI escape sequence: %q", i+1, notices[i])
		}
	}
}

func TestInteractiveGeminiRetryNoticeRedactsAPIKeyAndURL(t *testing.T) {
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	const apiKey = "secret-interactive-gemini-key"
	const endpoint = "https://example.test/v1beta/models"
	withProviderEndpoint(t, "gemini", endpoint)
	generated, notices, stderr, requests, err := runInteractiveGeminiRetry(t, apiKey)
	assertInteractiveGeminiResult(t, generated, err)
	assertInteractiveGeminiNotice(t, notices, apiKey, endpoint)
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want notice routed through the spinner update", stderr)
	}
}

func runInteractiveGeminiRetry(t *testing.T, apiKey string) (string, []string, string, int, error) {
	t.Helper()
	originalStart := startLoadingWithNotices
	var notices []string
	startLoadingWithNotices = func(done <-chan struct{}, retryNotices <-chan string) func() {
		noticesRead := make(chan struct{})
		go func() {
			defer close(noticesRead)
			for notice := range retryNotices {
				notices = append(notices, notice)
			}
		}()
		return func() {
			<-done
			<-noticesRead
		}
	}
	t.Cleanup(func() { startLoadingWithNotices = originalStart })

	requests := 0
	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return nil, timeoutTransportError("timed out")
		}
		return jsonResponse(http.StatusOK, `{"candidates":[{"content":{"parts":[{"text":"feat: interactive retry"}]}}]}`), nil
	}))
	var generated string
	var generateErr error
	_, stderr := captureStandardStreams(t, func() {
		generated, generateErr = generateText(providerOptions{
			Provider: "gemini",
			Model:    "test-model",
			APIKey:   apiKey,
		}, "prompt")
	})
	return generated, notices, stderr, requests, generateErr
}

func assertInteractiveGeminiResult(t *testing.T, generated string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("generateText() error = %v", err)
	}
	if generated != "feat: interactive retry" {
		t.Fatalf("generateText() = %q, want successful retry result", generated)
	}
}

func assertInteractiveGeminiNotice(t *testing.T, notices []string, apiKey, endpoint string) {
	t.Helper()
	if len(notices) != 1 {
		t.Fatalf("interactive notices = %q, want one notice", notices)
	}
	if !strings.Contains(notices[0], "timeout") || !strings.Contains(notices[0], "attempt 2/4") {
		t.Fatalf("interactive notice = %q, want timeout and attempt 2/4", notices[0])
	}
	if strings.Contains(notices[0], apiKey) || strings.Contains(notices[0], endpoint) || strings.Contains(notices[0], "?key=") {
		t.Fatalf("interactive notice leaked API key or URL: %q", notices[0])
	}
}

func TestHostedProviderClosesFailedResponseBeforeRetry(t *testing.T) {
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	failedBody := &closeTrackingBody{reader: strings.NewReader("overloaded")}
	requests := 0
	_, _, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: failedBody}, nil
		}
		if !failedBody.closed {
			t.Error("failed response body remained open before retry")
		}
		return jsonResponse(http.StatusOK, `{"output_text":"feat: recovered"}`), nil
	}))
	if err != nil || !failedBody.closed {
		t.Fatalf("generateText() error = %v, failed body closed = %t", err, failedBody.closed)
	}
}

func TestAgentRetryKeepsStdoutUnchanged(t *testing.T) {
	t.Setenv("GIT_AUTHOR_DATE", "2020-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2020-01-01T00:00:00Z")
	withoutRetry := runAgentCommitOutput(t, false)
	withRetry := runAgentCommitOutput(t, true)
	if withoutRetry != withRetry {
		t.Fatalf("Agent stdout differs after retry:\nwithout retry: %q\nwith retry: %q", withoutRetry, withRetry)
	}
}

func runAgentCommitOutput(t *testing.T, retry bool) string {
	t.Helper()
	repoDir := makeStagedChangeRepository(t, false)
	t.Chdir(repoDir)
	requests := 0
	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		if retry && requests == 1 {
			return jsonResponse(http.StatusServiceUnavailable, "overloaded"), nil
		}
		return jsonResponse(http.StatusOK, `{"output_text":"feat: stable agent output"}`), nil
	}))
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	stdout, stderr := captureStandardStreams(t, func() {
		if err := Run(Options{
			Language: "en",
			Model:    "gpt-5.5",
			Provider: "openai",
			APIKey:   "test-key",
			Agent:    true,
		}); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})
	assertAgentRunOutput(t, retry, requests, stderr)
	return stdout
}

func assertAgentRunOutput(t *testing.T, retry bool, requests int, stderr string) {
	t.Helper()
	wantRequests := 1
	if retry {
		wantRequests = 2
		if !strings.Contains(stderr, "retrying in") {
			t.Fatalf("stderr = %q, want retry notice", stderr)
		}
	} else if stderr != "" {
		t.Fatalf("stderr = %q, want no retry notice", stderr)
	}
	if requests != wantRequests {
		t.Fatalf("requests = %d, want %d", requests, wantRequests)
	}
}

func TestHostedProviderRetriesTransientTransportErrors(t *testing.T) {
	tests := []struct {
		name        string
		failure     error
		description string
	}{
		{name: "timeout", failure: timeoutTransportError("deadline exceeded"), description: "timeout"},
		{name: "connection reset", failure: syscall.ECONNRESET, description: "connection reset"},
		{name: "connection refused", failure: syscall.ECONNREFUSED, description: "connection refused"},
		{name: "unexpected end", failure: io.ErrUnexpectedEOF, description: "unexpected end of response"},
	}
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			text, stderr, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return nil, test.failure
				}
				return jsonResponse(http.StatusOK, `{"output_text":"feat: recovered"}`), nil
			}))
			if err != nil || text != "feat: recovered" || requests != 2 {
				t.Fatalf("generateText() = %q, %v after %d requests, want success after 2", text, err, requests)
			}
			if !strings.Contains(stderr, test.description) {
				t.Fatalf("stderr = %q, want %q", stderr, test.description)
			}
		})
	}
}

func TestHostedProviderJittersRetryDelaysAndReusesRequestBody(t *testing.T) {
	var limits []time.Duration
	var delays []time.Duration
	values := []time.Duration{100 * time.Millisecond, time.Second, 3 * time.Second}
	withRetryTiming(t, func(delay time.Duration) { delays = append(delays, delay) }, func(limit time.Duration) time.Duration {
		limits = append(limits, limit)
		return values[len(limits)-1]
	})

	var bodies [][]byte
	requests := 0
	_, _, err := generateWithTransport(t, "openai", roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			return nil, readErr
		}
		bodies = append(bodies, body)
		if requests < maxHostedAttempts {
			return jsonResponse(http.StatusServiceUnavailable, "overloaded"), nil
		}
		return jsonResponse(http.StatusOK, `{"output_text":"feat: recovered"}`), nil
	}))
	if err != nil {
		t.Fatalf("generateText() error = %v", err)
	}
	assertRetrySchedule(t, limits, delays)
	assertRequestBodiesWereReused(t, requests, bodies)
}

func assertRetrySchedule(t *testing.T, limits, delays []time.Duration) {
	t.Helper()
	if len(delays) != 3 || len(limits) != 3 {
		t.Fatalf("retry limits/delays = %v/%v, want 3 of each", limits, delays)
	}
	assertRetryDelayBounds(t, limits, delays)
	if delays[0] == delays[1] {
		t.Fatalf("different random values produced equal delays: %v", delays[:2])
	}
	if got := retryDelayLimit(7); got != maxRetryDelay {
		t.Fatalf("retryDelayLimit(7) = %s, want %s", got, maxRetryDelay)
	}
}

func assertRequestBodiesWereReused(t *testing.T, requests int, bodies [][]byte) {
	t.Helper()
	if requests != maxHostedAttempts || len(bodies) != requests || len(bodies[0]) == 0 {
		t.Fatalf("requests/bodies = %d/%d, want %d non-empty request bodies", requests, len(bodies), maxHostedAttempts)
	}
	assertEqualRequestBodies(t, bodies)
}

func assertRetryDelayBounds(t *testing.T, limits, delays []time.Duration) {
	t.Helper()
	wantLimits := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	for i, delay := range delays {
		if limits[i] != wantLimits[i] || delay < 0 || delay > limits[i] {
			t.Errorf("retry %d limit/delay = %s/%s, want limit %s and delay within range", i+1, limits[i], delay, wantLimits[i])
		}
	}
}

func assertEqualRequestBodies(t *testing.T, bodies [][]byte) {
	t.Helper()
	for i := 1; i < len(bodies); i++ {
		if !bytes.Equal(bodies[0], bodies[i]) {
			t.Fatalf("request body %d differs from first body", i+1)
		}
	}
}

func TestGeminiTransportRetriesDoNotExposeAPIKeyOrURL(t *testing.T) {
	withRetryTiming(t, func(time.Duration) {}, func(time.Duration) time.Duration { return 0 })
	const apiKey = "secret-gemini-key"
	const endpoint = "https://generativelanguage.googleapis.com/v1beta/models"
	withProviderEndpoint(t, "gemini", endpoint)
	requests := 0
	_, stderr, err := generateWithAPIKey(t, "gemini", apiKey, roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, timeoutTransportError("timed out")
	}))
	if err == nil || requests != maxHostedAttempts {
		t.Fatalf("generateText() error = %v after %d requests, want %d failures", err, requests, maxHostedAttempts)
	}
	for _, output := range []string{err.Error(), stderr} {
		if strings.Contains(output, apiKey) || strings.Contains(output, endpoint) || strings.Contains(output, "?key=") {
			t.Fatalf("API key or request URL leaked in %q", output)
		}
	}
	if strings.Count(stderr, "retrying in") != maxHostedAttempts-1 {
		t.Fatalf("retry notices = %q, want %d notices", stderr, maxHostedAttempts-1)
	}
}

func TestHostedProviderMissingAPIKeySendsNoRequest(t *testing.T) {
	requests := 0
	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(http.StatusOK, `{"output_text":"unexpected"}`), nil
	}))
	_, err := generateText(providerOptions{Provider: "openai", Model: "test-model", Agent: true}, "prompt")
	if err == nil || requests != 0 {
		t.Fatalf("generateText() error = %v after %d requests, want missing-key error and no request", err, requests)
	}
}

func TestOllamaDoesNotRetryServiceUnavailable(t *testing.T) {
	requests := 0
	withDefaultHTTPTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(http.StatusServiceUnavailable, "unavailable"), nil
	}))
	_, err := (ollamaProvider{model: "test-model"}).Generate("prompt")
	if err == nil || requests != 1 {
		t.Fatalf("Generate() error = %v after %d requests, want one request", err, requests)
	}
}

func TestHostedProviderFirstAttemptSuccessHasNoRetryNotice(t *testing.T) {
	_, stderr, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: first try"}`), nil
	}))
	if err != nil {
		t.Fatalf("generateText() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no retry notice", stderr)
	}
}

func TestHostedProviderNonTransientTransportErrorIsFinal(t *testing.T) {
	requests := 0
	_, stderr, err := generateWithTransport(t, "openai", roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("x509: certificate signed by unknown authority")
	}))
	if err == nil || requests != 1 || stderr != "" {
		t.Fatalf("generateText() error = %v after %d requests with stderr %q, want one final failure", err, requests, stderr)
	}
}

func generateWithTransport(t *testing.T, provider string, transport roundTripFunc) (string, string, error) {
	t.Helper()
	return generateWithAPIKey(t, provider, "test-key", transport)
}

func generateWithAPIKey(t *testing.T, provider, apiKey string, transport roundTripFunc) (string, string, error) {
	t.Helper()
	withProviderHTTPClient(t, transport)
	var generated string
	var generateErr error
	_, stderr := captureStandardStreams(t, func() {
		generated, generateErr = generateText(providerOptions{
			Provider: provider,
			Model:    "test-model",
			APIKey:   apiKey,
			Agent:    true,
		}, "prompt")
	})
	return generated, stderr, generateErr
}

func assertSecretAbsent(t *testing.T, err error, secrets ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want failure")
	}
	for _, secret := range secrets {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked %q: %v", secret, err)
		}
	}
}

type exposingCodec struct {
	details string
}

func (exposingCodec) BuildRequest(model, prompt string) (any, error) {
	return map[string]string{"model": model, "input": prompt}, nil
}

func (exposingCodec) DecorateRequest(*http.Request, string) {}

func (codec exposingCodec) ParseResponse(io.Reader) (string, error) {
	return "", errors.New(codec.details)
}

type timeoutTransportError string

func (err timeoutTransportError) Error() string   { return string(err) }
func (err timeoutTransportError) Timeout() bool   { return true }
func (err timeoutTransportError) Temporary() bool { return true }

type closeTrackingBody struct {
	reader io.Reader
	closed bool
}

type failingReadCloser struct {
	reader io.Reader
	err    error
	failed bool
	closed bool
}

func (body *failingReadCloser) Read(buffer []byte) (int, error) {
	if body.failed {
		return 0, io.EOF
	}
	body.failed = true
	n, _ := body.reader.Read(buffer)
	return n, body.err
}

func (body *failingReadCloser) Close() error {
	body.closed = true
	return nil
}

func (body *closeTrackingBody) Read(buffer []byte) (int, error) { return body.reader.Read(buffer) }
func (body *closeTrackingBody) Close() error {
	body.closed = true
	return nil
}

func withRetryTiming(t *testing.T, sleep func(time.Duration), jitter func(time.Duration) time.Duration) {
	t.Helper()
	originalSleep := retrySleep
	originalJitter := retryJitter
	retrySleep = sleep
	retryJitter = jitter
	t.Cleanup(func() {
		retrySleep = originalSleep
		retryJitter = originalJitter
	})
}
