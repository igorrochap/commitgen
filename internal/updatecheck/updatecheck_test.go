package updatecheck

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLatestVersionReadsGoProxyVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "commitgen" {
			t.Fatalf("User-Agent = %q, want commitgen", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"Version":"v1.2.3"}`)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}))
	defer server.Close()

	version, err := LatestVersion(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if version != "v1.2.3" {
		t.Fatalf("LatestVersion() = %q, want v1.2.3", version)
	}
}

func TestLatestVersionFallsBackToReleaseTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"tag_name":"v1.2.3"}`)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}))
	defer server.Close()

	version, err := LatestVersion(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if version != "v1.2.3" {
		t.Fatalf("LatestVersion() = %q, want v1.2.3", version)
	}
}

func TestLatestVersionErrorsWhenVersionIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{}`)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}))
	defer server.Close()

	if _, err := LatestVersion(context.Background(), server.Client(), server.URL); err == nil {
		t.Fatal("LatestVersion() error = nil, want error")
	}
}

func TestLatestVersionRejectsServerAndDecodeErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "server status", statusCode: http.StatusServiceUnavailable, body: "try again"},
		{name: "invalid JSON", statusCode: http.StatusOK, body: "{"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
				if _, err := w.Write([]byte(test.body)); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
			}))
			defer server.Close()

			if _, err := LatestVersion(context.Background(), server.Client(), server.URL); err == nil {
				t.Fatal("LatestVersion() error = nil, want response error")
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  int
	}{
		{name: "newer patch", left: "v1.1.1", right: "v1.1.0", want: 1},
		{name: "newer minor", left: "1.2.0", right: "1.1.9", want: 1},
		{name: "newer major", left: "2.0.0", right: "1.9.9", want: 1},
		{name: "same with v prefix", left: "v1.1.0", right: "1.1.0", want: 0},
		{name: "older", left: "1.0.9", right: "1.1.0", want: -1},
		{name: "ignores prerelease suffix", left: "v1.1.0-beta.1", right: "1.1.0", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompareVersions(tt.left, tt.right); got != tt.want {
				t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestCheckReturnsCurrentAndLatestVersions(t *testing.T) {
	withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return versionResponse(`{"Version":"v99.0.0"}`), nil
	}))

	result, err := Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Current == "" || result.Latest != "v99.0.0" || !result.Newer {
		t.Fatalf("Check() = %+v, want current version and newer v99.0.0", result)
	}
}

func TestCheckReturnsLatestVersionError(t *testing.T) {
	wantErr := errors.New("network unavailable")
	withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, wantErr
	}))

	result, err := Check(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Check() error = %v, want %v", err, wantErr)
	}
	if result.Latest != "" || result.Newer {
		t.Fatalf("Check() result = %+v, want current-only result", result)
	}
}

func TestCheckWithTimeout(t *testing.T) {
	withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return versionResponse(`{"tag_name":"v99.0.0"}`), nil
	}))

	result, err := CheckWithTimeout(time.Second)
	if err != nil {
		t.Fatalf("CheckWithTimeout() error = %v", err)
	}
	if result.Latest != "v99.0.0" || !result.Newer {
		t.Fatalf("CheckWithTimeout() = %+v, want newer v99.0.0", result)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func withDefaultTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = original
	})
}

func versionResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
