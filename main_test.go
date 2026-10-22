package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igorrochap/commitgen/cmd"
)

func TestMainExecutesVersionCommand(t *testing.T) {
	previousArgs := os.Args
	previousStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	os.Args = []string{"commitgen", "--version"}
	os.Stdout = writer
	t.Cleanup(func() {
		os.Args = previousArgs
		os.Stdout = previousStdout
	})

	main()

	if err := writer.Close(); err != nil {
		t.Fatalf("Close(stdout) error = %v", err)
	}
	os.Stdout = previousStdout
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close(reader) error = %v", err)
	}
	if !strings.HasPrefix(string(output), "commitgen ") {
		t.Fatalf("main() output = %q, want version output", output)
	}
}

func TestMainPullRequestCommand(t *testing.T) {
	repoDir := makePullRequestRepository(t)
	t.Chdir(repoDir)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir() error = %v", err)
	}
	configPath := filepath.Join(configDir, "commitgen", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	config := `{"language":"en","model":"gpt-5.5","provider":"openai","api_keys":{"openai":"sk-test"}}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	previousTransport := http.DefaultTransport
	http.DefaultTransport = mainPullRequestTransport()
	t.Cleanup(func() {
		http.DefaultTransport = previousTransport
	})

	output := executeMainCommand(t, "pr", "--version=false")
	if !strings.Contains(output, "feat: summarize from main integration\n") {
		t.Fatalf("PR command output = %q, want generated description", output)
	}

	output = executeMainCommand(t, "config", "set", "--language", "pt-BR", "--version=false")
	if !strings.Contains(output, "Defaults updated: language=pt-BR") {
		t.Fatalf("config set output = %q, want saved language", output)
	}
	output = executeMainCommand(t, "config", "show", "--version=false")
	if !strings.Contains(output, "language=pt-BR\n") {
		t.Fatalf("config show output = %q, want saved language", output)
	}

	goBinDir := t.TempDir()
	goPath := filepath.Join(t.TempDir(), "gopath")
	goExecutable := filepath.Join(goBinDir, "go")
	goScript := "#!/bin/sh\ncase \"$1\" in\n" +
		"install) exit 0 ;;\n" +
		"env) printf '%s\\n' \"$COMMITGEN_UPDATE_GOPATH\" ;;\n" +
		"*) exit 1 ;;\nesac\n"
	if err := os.WriteFile(goExecutable, []byte(goScript), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("PATH", goBinDir)
	t.Setenv("COMMITGEN_UPDATE_GOPATH", goPath)
	output = executeMainCommand(t, "update", "--version=false")
	if !strings.Contains(output, "Installed to "+filepath.Join(goPath, "bin", "commitgen")) {
		t.Fatalf("update output = %q, want install location", output)
	}
}

func mainPullRequestTransport() http.RoundTripper {
	return mainRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "proxy.golang.org" {
			return mainHTTPResponse(`{"Version":"v1.1.0"}`), nil
		}
		if request.URL.Host != "api.openai.com" || request.Header.Get("Authorization") != "Bearer sk-test" {
			return nil, errors.New("unexpected provider request")
		}
		var payload struct {
			Input string `json:"input"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return nil, err
		}
		if !strings.Contains(payload.Input, "feat: add integration summary") {
			return nil, errors.New("provider prompt omitted feature commit")
		}
		return mainHTTPResponse(`{"output_text":"feat: summarize from main integration"}`), nil
	})
}

func executeMainCommand(t *testing.T, args ...string) string {
	t.Helper()
	previousArgs := os.Args
	previousStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	os.Args = append([]string{"commitgen"}, args...)
	os.Stdout = writer
	cmd.Execute()
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(stdout) error = %v", err)
	}
	os.Args = previousArgs
	os.Stdout = previousStdout
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close(reader) error = %v", err)
	}
	return string(output)
}

func makePullRequestRepository(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	runMainTestGit(t, repoDir, "init", "-b", "main")
	runMainTestGit(t, repoDir, "config", "user.email", "test@example.com")
	runMainTestGit(t, repoDir, "config", "user.name", "Test User")
	runMainTestGit(t, repoDir, "commit", "--allow-empty", "-m", "base commit")
	runMainTestGit(t, repoDir, "checkout", "-b", "feature")
	runMainTestGit(t, repoDir, "commit", "--allow-empty", "-m", "feat: add integration summary")
	return repoDir
}

func runMainTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

type mainRoundTripFunc func(*http.Request) (*http.Response, error)

func (f mainRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func mainHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
