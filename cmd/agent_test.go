package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	appconfig "github.com/igorrochap/commitgen/internal/config"
	"github.com/igorrochap/commitgen/internal/generator"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestAgentFlagIsAvailableOnEveryCommandAndSkipsUpdateChecks(t *testing.T) {
	repoDir := createSingleCommitRepo(t)
	t.Chdir(repoDir)
	configPath := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, configPath)
	if err := appconfig.SaveFile(configPath, appconfig.Defaults()); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	updateChecks := 0
	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		updateChecks++
		return updateResponse(`{"Version":"v99.0.0"}`), nil
	}))
	installFakeGo(t)
	withTerminalStreams(t, true, true)

	commands := []struct {
		args      []string
		wantError string
	}{
		{args: []string{"--agent"}, wantError: "no staged changes"},
		{args: []string{"push", "--agent"}, wantError: "no staged changes"},
		{args: []string{"pr", "--agent"}, wantError: "no commits found"},
		{args: []string{"config", "--agent"}},
		{args: []string{"config", "show", "--agent"}},
		{args: []string{"config", "set", "--model", "test-model", "--agent"}},
		{args: []string{"version", "--agent"}},
		{args: []string{"update", "--agent"}},
	}
	for _, command := range commands {
		t.Run(strings.Join(command.args, " "), func(t *testing.T) {
			assertAgentCommandAccepted(t, command.args, command.wantError, &updateChecks)
		})
	}
}

func TestAutomaticAgentModeSkipsUpdateChecksForEveryCommand(t *testing.T) {
	repoDir := createSingleCommitRepo(t)
	t.Chdir(repoDir)
	configPath := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, configPath)
	if err := appconfig.SaveFile(configPath, appconfig.Defaults()); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	updateChecks := 0
	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		updateChecks++
		return updateResponse(`{"Version":"v99.0.0"}`), nil
	}))
	installFakeGo(t)
	withTerminalStreams(t, false, false)

	commands := []struct {
		args      []string
		wantError string
	}{
		{args: []string{}, wantError: "no staged changes"},
		{args: []string{"push"}, wantError: "no staged changes"},
		{args: []string{"pr"}, wantError: "no commits found"},
		{args: []string{"config"}},
		{args: []string{"config", "show"}},
		{args: []string{"config", "set", "--model", "test-model"}},
		{args: []string{"version"}},
		{args: []string{"update"}},
	}
	for _, command := range commands {
		t.Run(strings.Join(command.args, " "), func(t *testing.T) {
			assertAgentCommandAccepted(t, command.args, command.wantError, &updateChecks)
		})
	}
}

func assertAgentCommandAccepted(t *testing.T, args []string, wantError string, updateChecks *int) {
	t.Helper()
	resetCommandTree(t)
	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assertCommandError(t, args, err, wantError)
	if *updateChecks != 0 {
		t.Fatalf("update checks = %d after %q, want 0", *updateChecks, args)
	}
	if strings.Contains(stderr.String(), "A new commitgen version is available") {
		t.Fatalf("stderr for %q contains an update notice: %q", args, stderr.String())
	}
}

func assertCommandError(t *testing.T, args []string, err error, wantError string) {
	t.Helper()
	if wantError == "" {
		if err != nil {
			t.Fatalf("Execute(%q) error = %v, want success", args, err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), wantError) {
		t.Fatalf("Execute(%q) error = %v, want %q", args, err, wantError)
	}
}

func withTerminalStreams(t *testing.T, stdinIsTerminal, stdoutIsTerminal bool) {
	t.Helper()
	originalDetectTerminalStreams := detectTerminalStreams
	detectTerminalStreams = func() (bool, bool) {
		return stdinIsTerminal, stdoutIsTerminal
	}
	t.Cleanup(func() { detectTerminalStreams = originalDetectTerminalStreams })
}

func TestRootCommandAutomaticallyEntersAgentModeWithoutTerminal(t *testing.T) {
	repoDir := createPullRequestRepo(t)
	stageCommandChange(t, repoDir)
	t.Chdir(repoDir)
	configureHostedProvider(t)

	updateChecks := 0
	withUpdateTransport(t, commandRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "proxy.golang.org" {
			updateChecks++
			return updateResponse(`{"Version":"v99.0.0"}`), nil
		}
		return updateResponse(`{"output_text":"feat: agent commit"}`), nil
	}))

	resetCommandTree(t)
	withTerminalStreams(t, false, false)
	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(nil)
	var commandErr error
	output := captureCommandStdout(t, func() {
		commandErr = rootCmd.Execute()
	})
	if commandErr != nil {
		t.Fatalf("root command error = %v", commandErr)
	}
	shortHash := commandGitOutput(t, repoDir, "rev-parse", "--short", "HEAD")
	if got, want := output, "Commit "+shortHash+" created\n\nfeat: agent commit\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got := commandGitOutput(t, repoDir, "log", "-1", "--format=%B"); got != "feat: agent commit" {
		t.Fatalf("commit message = %q, want generated message", got)
	}
	if updateChecks != 0 {
		t.Fatalf("update checks = %d, want 0 in Agent mode", updateChecks)
	}
	if strings.Contains(stderr.String(), "A new commitgen version is available") {
		t.Fatalf("stderr contains update notice: %q", stderr.String())
	}
}

func TestRootCommandSelectsModeFromTerminalStreamsAndAgentFlag(t *testing.T) {
	repoDir := createSingleCommitRepo(t)
	t.Chdir(repoDir)
	configureHostedProvider(t)

	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return updateResponse(`{"Version":"v1.0.0"}`), nil
	}))

	tests := []struct {
		name             string
		stdinIsTerminal  bool
		stdoutIsTerminal bool
		agentFlag        bool
		wantAgentMode    bool
	}{
		{name: "terminal stdin and terminal stdout", stdinIsTerminal: true, stdoutIsTerminal: true},
		{name: "terminal stdin and terminal stdout with flag", stdinIsTerminal: true, stdoutIsTerminal: true, agentFlag: true, wantAgentMode: true},
		{name: "terminal stdin and piped stdout", stdinIsTerminal: true, wantAgentMode: true},
		{name: "terminal stdin and piped stdout with flag", stdinIsTerminal: true, agentFlag: true, wantAgentMode: true},
		{name: "piped stdin and terminal stdout", stdoutIsTerminal: true, wantAgentMode: true},
		{name: "piped stdin and terminal stdout with flag", stdoutIsTerminal: true, agentFlag: true, wantAgentMode: true},
		{name: "piped stdin and piped stdout", wantAgentMode: true},
		{name: "piped stdin and piped stdout with flag", agentFlag: true, wantAgentMode: true},
	}

	modeError := errors.New("stop after resolving mode")
	originalRunCommitGeneration := runCommitGeneration
	var gotAgentMode bool
	runCommitGeneration = func(options generator.Options) error {
		gotAgentMode = options.Agent
		return modeError
	}
	t.Cleanup(func() { runCommitGeneration = originalRunCommitGeneration })

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetCommandTree(t)
			terminalChecks := 0
			originalDetectTerminalStreams := detectTerminalStreams
			detectTerminalStreams = func() (bool, bool) {
				terminalChecks++
				return test.stdinIsTerminal, test.stdoutIsTerminal
			}
			t.Cleanup(func() { detectTerminalStreams = originalDetectTerminalStreams })
			gotAgentMode = false
			args := []string{}
			if test.agentFlag {
				args = append(args, "--agent")
			}
			rootCmd.SetArgs(args)

			err := rootCmd.Execute()
			if !errors.Is(err, modeError) {
				t.Fatalf("root command error = %v, want %v", err, modeError)
			}
			if gotAgentMode != test.wantAgentMode {
				t.Fatalf("Agent mode = %t, want %t", gotAgentMode, test.wantAgentMode)
			}
			if terminalChecks != 1 {
				t.Fatalf("terminal checks = %d, want 1", terminalChecks)
			}
		})
	}
}

func TestRootCommandAgentWithoutStagedChangesReportsError(t *testing.T) {
	repoDir := createSingleCommitRepo(t)
	t.Chdir(repoDir)
	configureHostedProvider(t)
	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return updateResponse(`{"Version":"v99.0.0"}`), nil
	}))

	resetCommandTree(t)
	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"--agent"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no staged changes") {
		t.Fatalf("root command error = %v, want no staged changes error", err)
	}
	if !strings.Contains(stderr.String(), "no staged changes") {
		t.Fatalf("stderr = %q, want no staged changes error", stderr.String())
	}
}

func TestPushCommandAgentCommitsAndPushes(t *testing.T) {
	repoDir := createPullRequestRepo(t)
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	if err := os.Mkdir(remoteDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	commandGit(t, remoteDir, "init", "--bare")
	commandGit(t, repoDir, "remote", "add", "origin", remoteDir)
	commandGit(t, repoDir, "push", "-u", "origin", "feature")
	stageCommandChange(t, repoDir)
	t.Chdir(repoDir)
	configureHostedProvider(t)

	withUpdateTransport(t, commandRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "proxy.golang.org" {
			return updateResponse(`{"Version":"v99.0.0"}`), nil
		}
		return updateResponse(`{"output_text":"feat: agent push"}`), nil
	}))

	resetCommandTree(t)
	rootCmd.SetArgs([]string{"push", "--agent"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("push --agent error = %v", err)
	}
	localCommit := commandGitOutput(t, repoDir, "rev-parse", "HEAD")
	remoteCommit := commandGitOutput(t, remoteDir, "rev-parse", "refs/heads/feature")
	if remoteCommit != localCommit {
		t.Fatalf("remote commit = %q, want %q", remoteCommit, localCommit)
	}
}

func TestPullRequestCommandAgentPrintsOnlyGeneratedText(t *testing.T) {
	repoDir := createPullRequestRepo(t)
	t.Chdir(repoDir)
	configureHostedProvider(t)

	updateChecks := 0
	withUpdateTransport(t, commandRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "proxy.golang.org" {
			updateChecks++
			return updateResponse(`{"Version":"v99.0.0"}`), nil
		}
		return updateResponse(`{"output_text":"Generated pull request text"}`), nil
	}))

	resetCommandTree(t)
	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"pr", "--agent"})
	var commandErr error
	output := captureCommandStdout(t, func() {
		commandErr = rootCmd.Execute()
	})
	if commandErr != nil {
		t.Fatalf("pr --agent error = %v", commandErr)
	}
	if output != "Generated pull request text\n" {
		t.Fatalf("stdout = %q, want generated text only", output)
	}
	if updateChecks != 0 || strings.Contains(stderr.String(), "A new commitgen version is available") {
		t.Fatalf("Agent mode performed update check or printed notice: checks=%d stderr=%q", updateChecks, stderr.String())
	}
}

func TestAgentFlagHelpIsInheritedByGenerationCommands(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"push", "--help"}, {"pr", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			resetCommandTree(t)
			var output bytes.Buffer
			rootCmd.SetOut(&output)
			rootCmd.SetArgs(args)
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("Execute(%q) error = %v", args, err)
			}
			if !strings.Contains(output.String(), "--agent") ||
				!strings.Contains(output.String(), "Run without Review") ||
				!strings.Contains(output.String(), "commit the generated message directly") ||
				!strings.Contains(output.String(), "plain output") ||
				!strings.Contains(output.String(), "auto-enabled when no terminal is detected") {
				t.Fatalf("help for %q does not describe Agent mode: %q", args, output.String())
			}
		})
	}
}

func TestCommandWithoutAgentStillPrintsUpdateNotice(t *testing.T) {
	resetCommandTree(t)
	withTerminalStreams(t, true, true)
	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"version"})
	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return updateResponse(`{"Version":"v99.0.0"}`), nil
	}))
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command error = %v", err)
	}
	if !strings.Contains(stderr.String(), "A new commitgen version is available") {
		t.Fatalf("stderr = %q, want update notice", stderr.String())
	}
}

func configureHostedProvider(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{
		Language: "en",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKeys:  map[string]string{"openai": "sk-test"},
	}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
}

func stageCommandChange(t *testing.T, repoDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, "staged.txt"), []byte("staged change\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	commandGit(t, repoDir, "add", "staged.txt")
}

func createSingleCommitRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	commandGit(t, repoDir, "init", "-b", "main")
	commandGit(t, repoDir, "config", "user.email", "test@example.com")
	commandGit(t, repoDir, "config", "user.name", "Test User")
	commandGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")
	return repoDir
}

func commandGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output))
}

func installFakeGo(t *testing.T) {
	t.Helper()
	binDir := t.TempDir()
	goExecutable := filepath.Join(binDir, "go")
	script := "#!/bin/sh\ncase \"$1\" in\n" +
		"install) exit 0 ;;\n" +
		"env) printf '%s\\n' \"$COMMITGEN_TEST_GOPATH\" ;;\n" +
		"*) exit 1 ;;\nesac\n"
	if err := os.WriteFile(goExecutable, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("COMMITGEN_TEST_GOPATH", t.TempDir())
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func resetCommandTree(t *testing.T) {
	t.Helper()
	var resetFlags func(*cobra.Command)
	resetFlags = func(command *cobra.Command) {
		command.Flags().VisitAll(resetCommandFlag(t))
		command.PersistentFlags().VisitAll(resetCommandFlag(t))
		for _, child := range command.Commands() {
			resetFlags(child)
		}
	}
	resetFlags(rootCmd)
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	t.Cleanup(func() {
		resetFlags(rootCmd)
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
	})
}

func resetCommandFlag(t *testing.T) func(*pflag.Flag) {
	t.Helper()
	return func(flag *pflag.Flag) {
		if err := flag.Value.Set(flag.DefValue); err != nil {
			t.Errorf("reset --%s: %v", flag.Name, err)
		}
		flag.Changed = false
	}
}
