package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	appconfig "github.com/igorrochap/commitgen/internal/config"
	"github.com/spf13/pflag"
)

func TestPullRequestCommandUsesDetectedBranchesAndPrintsDescription(t *testing.T) {
	repoDir := createPullRequestRepo(t)
	t.Chdir(repoDir)
	withConfigPath(t, filepath.Join(t.TempDir(), "config.json"))
	if err := appconfig.SaveFile(configPathValue(t), appconfig.Config{
		Language: "en",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKeys:  map[string]string{"openai": "sk-test"},
	}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	resetPullRequestCommand(t)

	var gotPrompt string
	withUpdateTransport(t, commandRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "proxy.golang.org" {
			return updateResponse(`{"Version":"v1.1.0"}`), nil
		}
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer sk-test" {
			return nil, fmt.Errorf("unexpected provider request: %s %s", request.Method, request.URL)
		}
		var payload struct {
			Input string `json:"input"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return nil, err
		}
		gotPrompt = payload.Input
		return updateResponse(`{"output_text":"feat: summarize pull request"}`), nil
	}))

	output := captureCommandStdout(t, func() {
		rootCmd.SetArgs([]string{"pr"})
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("pr command error = %v", err)
		}
	})
	if !strings.Contains(output, "feat: summarize pull request\n") {
		t.Fatalf("pr command output = %q, want generated description", output)
	}
	if !strings.Contains(gotPrompt, "feat: add branch summary") {
		t.Fatalf("provider prompt = %q, want detected feature branch commit", gotPrompt)
	}
}

func createPullRequestRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	commandGit(t, repoDir, "init", "-b", "main")
	commandGit(t, repoDir, "config", "user.email", "test@example.com")
	commandGit(t, repoDir, "config", "user.name", "Test User")
	commandGit(t, repoDir, "commit", "--allow-empty", "-m", "base commit")
	commandGit(t, repoDir, "checkout", "-b", "feature")
	commandGit(t, repoDir, "commit", "--allow-empty", "-m", "feat: add branch summary")
	return repoDir
}

func commandGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func configPathValue(t *testing.T) string {
	t.Helper()
	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath() error = %v", err)
	}
	return path
}

func resetPullRequestCommand(t *testing.T) {
	t.Helper()
	prCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if err := flag.Value.Set(flag.DefValue); err != nil {
			t.Fatalf("reset --%s: %v", flag.Name, err)
		}
		flag.Changed = false
	})
	prCmd.SetOut(io.Discard)
	prCmd.SetErr(io.Discard)
}

func captureCommandStdout(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	original := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()
	run()
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	os.Stdout = original
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return string(output)
}
