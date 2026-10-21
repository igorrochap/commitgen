package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNotifyUpdatePrintsAvailableVersion(t *testing.T) {
	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return updateResponse(`{"Version":"v99.0.0"}`), nil
	}))

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&output)
	notifyUpdate(cmd)

	if !strings.Contains(output.String(), "A new commitgen version is available") {
		t.Fatalf("notifyUpdate() output = %q, want update notice", output.String())
	}
}

func TestNotifyUpdateIgnoresCheckError(t *testing.T) {
	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	}))

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&output)
	notifyUpdate(cmd)
	if output.Len() != 0 {
		t.Fatalf("notifyUpdate() output = %q, want no notice", output.String())
	}
}

func TestNotifyUpdateDoesNotPrintWhenNoNewerVersionExists(t *testing.T) {
	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return updateResponse(`{"Version":"v0.0.1"}`), nil
	}))

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&output)
	notifyUpdate(cmd)
	if output.Len() != 0 {
		t.Fatalf("notifyUpdate() output = %q, want no notice", output.String())
	}
}

type commandRoundTripFunc func(*http.Request) (*http.Response, error)

func (f commandRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func withUpdateTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = original
	})
}

func updateResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
