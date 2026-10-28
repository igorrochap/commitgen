package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/igorrochap/commitgen/internal/prompts"
	"github.com/igorrochap/commitgen/internal/selection"
)

func TestUnwrapLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "soft wrap at a word boundary",
			input: "A soft wrap\nat a word boundary.",
			want:  "A soft wrap at a word boundary.",
		},
		{
			name:  "mid-word duplicate",
			input: "internationalizatio\ninternationalization",
			want:  "internationalization",
		},
		{
			name:  "paragraph break",
			input: "first paragraph\ncontinues\n\nsecond paragraph\ncontinues",
			want:  "first paragraph continues\n\nsecond paragraph continues",
		},
		{
			name:  "dash list item",
			input: "intro\n- first item",
			want:  "intro\n- first item",
		},
		{
			name:  "asterisk list item",
			input: "intro\n* first item",
			want:  "intro\n* first item",
		},
		{
			name:  "plus list item",
			input: "intro\n+ first item",
			want:  "intro\n+ first item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unwrapLines(tt.input); got != tt.want {
				t.Fatalf("unwrapLines(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

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
			assertPromptContext(t, tt.context, tt.wantContext)
		})
	}
}

func TestGetPromptRejectsUnsupportedLanguage(t *testing.T) {
	if _, err := getPrompt("fr"); err == nil {
		t.Fatal("getPrompt() error = nil, want unsupported-language error")
	}
}

func TestRunAndRunPushRejectUnsupportedLanguage(t *testing.T) {
	if err := Run(Options{Language: "fr"}); err == nil {
		t.Fatal("Run() error = nil, want unsupported-language error")
	}
	if err := RunPush(Options{Language: "fr"}); err == nil {
		t.Fatal("RunPush() error = nil, want unsupported-language error")
	}
}

func TestRunAndRunPushUseInjectedSelectionWithoutTTY(t *testing.T) {
	withProviderLoading(t, func(done <-chan struct{}) func() {
		return func() { <-done }
	})
	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: add change"}`), nil
	}))

	originalSelectionRunner := selectionRunner
	t.Cleanup(func() { selectionRunner = originalSelectionRunner })

	for _, testCase := range []struct {
		name string
		run  func(Options) error
		push bool
	}{
		{name: "run", run: Run},
		{name: "push", run: RunPush, push: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assertInjectedRunCreatesCommit(t, testCase.name, testCase.run, testCase.push)
		})
	}
}

func TestRunAgentCommitsWithoutReviewAndPrintsCommitMessage(t *testing.T) {
	repoDir := makeStagedChangeRepository(t, false)
	t.Chdir(repoDir)
	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: agent commit"}`), nil
	}))

	selectionCalled := false
	originalSelectionRunner := selectionRunner
	selectionRunner = func(string) (selection.Result, error) {
		selectionCalled = true
		return selection.Result{Choice: selection.Accept}, nil
	}
	t.Cleanup(func() { selectionRunner = originalSelectionRunner })

	loadingStarts := 0
	withProviderLoading(t, func(<-chan struct{}) func() {
		loadingStarts++
		return func() {}
	})

	output := captureStdout(t, func() {
		err := Run(Options{
			Language: "en",
			Model:    "gpt-5.5",
			Provider: "openai",
			APIKey:   "sk-test",
			Agent:    true,
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})
	if selectionCalled {
		t.Fatal("Review started in Agent mode")
	}
	if loadingStarts != 0 {
		t.Fatalf("loading starts = %d, want 0 in Agent mode", loadingStarts)
	}

	shortHash := runGit(t, repoDir, "rev-parse", "--short", "HEAD")
	if got, want := output, "Commit "+shortHash+" created\n\nfeat: agent commit\n"; got != want {
		t.Fatalf("Run() output = %q, want %q", got, want)
	}
	if got := runGit(t, repoDir, "log", "-1", "--format=%B"); got != "feat: agent commit" {
		t.Fatalf("commit message = %q, want generated message", got)
	}
}

func TestMakeAgentCommitPrintsMessageStoredByGit(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")
	t.Chdir(repoDir)

	hookPath := filepath.Join(repoDir, ".git", "hooks", "commit-msg")
	hook := "#!/bin/sh\nprintf '%s\\n' 'feat: message stored by git' > \"$1\"\n"
	if err := os.WriteFile(hookPath, []byte(hook), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, repoDir, "add", "change.txt")

	output := captureStdout(t, func() {
		if err := makeAgentCommit("feat: generated message"); err != nil {
			t.Fatalf("makeAgentCommit() error = %v", err)
		}
	})
	shortHash := runGit(t, repoDir, "rev-parse", "--short", "HEAD")
	if got, want := output, "Commit "+shortHash+" created\n\nfeat: message stored by git\n"; got != want {
		t.Fatalf("makeAgentCommit() output = %q, want %q", got, want)
	}
	if got := runGit(t, repoDir, "log", "-1", "--format=%B"); got != "feat: message stored by git" {
		t.Fatalf("committed message = %q, want stored message", got)
	}
}

func TestGitCommitMessageReadsBodyAfterMultilineSignatureHeader(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "feat: signed header message")
	t.Chdir(repoDir)

	commit, err := exec.Command("git", "cat-file", "commit", "HEAD").Output()
	if err != nil {
		t.Fatalf("git cat-file error = %v", err)
	}
	separator := bytes.Index(commit, []byte("\n\n"))
	if separator < 0 {
		t.Fatal("commit object is missing header separator")
	}

	signedCommit := append([]byte(nil), commit[:separator]...)
	signedCommit = append(signedCommit, []byte("\ngpgsig -----BEGIN PGP SIGNATURE-----\n signature data\n -----END PGP SIGNATURE-----")...)
	signedCommit = append(signedCommit, commit[separator:]...)
	writeCommit := exec.Command("git", "hash-object", "-t", "commit", "-w", "--stdin")
	writeCommit.Stdin = bytes.NewReader(signedCommit)
	commitID, err := writeCommit.Output()
	if err != nil {
		t.Fatalf("git hash-object error = %v", err)
	}
	runGit(t, repoDir, "update-ref", "HEAD", strings.TrimSpace(string(commitID)))

	message, err := gitCommitMessage()
	if err != nil {
		t.Fatalf("gitCommitMessage() error = %v", err)
	}
	if message != "feat: signed header message" {
		t.Fatalf("gitCommitMessage() = %q, want commit message", message)
	}
}

func TestRunInteractiveReviewsStartsLoadingAndPrintsOnlyCommitNotice(t *testing.T) {
	repoDir := makeStagedChangeRepository(t, false)
	t.Chdir(repoDir)
	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: interactive commit"}`), nil
	}))

	selectionCalls := 0
	originalSelectionRunner := selectionRunner
	selectionRunner = func(message string) (selection.Result, error) {
		selectionCalls++
		if message != "feat: interactive commit" {
			t.Errorf("Review message = %q, want generated message", message)
		}
		return selection.Result{Choice: selection.Accept}, nil
	}
	t.Cleanup(func() { selectionRunner = originalSelectionRunner })

	loadingStarts := 0
	withProviderLoading(t, func(done <-chan struct{}) func() {
		loadingStarts++
		return func() { <-done }
	})

	output := captureStdout(t, func() {
		err := Run(Options{
			Language: "en",
			Model:    "gpt-5.5",
			Provider: "openai",
			APIKey:   "sk-test",
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})
	if selectionCalls != 1 {
		t.Fatalf("Review calls = %d, want 1", selectionCalls)
	}
	if loadingStarts != 1 {
		t.Fatalf("loading starts = %d, want 1", loadingStarts)
	}

	shortHash := runGit(t, repoDir, "rev-parse", "--short", "HEAD")
	if got, want := output, "Commit "+shortHash+" created\n"; got != want {
		t.Fatalf("Run() output = %q, want only %q", got, want)
	}
}

func TestRunPushAgentCommitsWithoutReviewAndPushes(t *testing.T) {
	repoDir := makeStagedChangeRepository(t, true)
	t.Chdir(repoDir)
	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: agent push"}`), nil
	}))

	selectionCalled := false
	originalSelectionRunner := selectionRunner
	selectionRunner = func(string) (selection.Result, error) {
		selectionCalled = true
		return selection.Result{Choice: selection.Accept}, nil
	}
	t.Cleanup(func() { selectionRunner = originalSelectionRunner })

	loadingStarts := 0
	withProviderLoading(t, func(<-chan struct{}) func() {
		loadingStarts++
		return func() {}
	})

	if err := RunPush(Options{
		Language: "en",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKey:   "sk-test",
		Agent:    true,
	}); err != nil {
		t.Fatalf("RunPush() error = %v", err)
	}
	if selectionCalled {
		t.Fatal("Review started in Agent mode")
	}
	if loadingStarts != 0 {
		t.Fatalf("loading starts = %d, want 0 in Agent mode", loadingStarts)
	}

	localCommit := runGit(t, repoDir, "rev-parse", "HEAD")
	assertRemoteCommit(t, filepath.Join(repoDir, "remote.git"), localCommit)
}

func assertInjectedRunCreatesCommit(t *testing.T, name string, run func(Options) error, push bool) {
	t.Helper()
	repoDir := makeStagedChangeRepository(t, push)
	t.Chdir(repoDir)

	var selectedMessage string
	selectionRunner = func(message string) (selection.Result, error) {
		selectedMessage = message
		return selection.Result{Choice: selection.Accept}, nil
	}
	err := run(Options{Language: "en", Model: "gpt-5.5", Provider: "openai", APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	if selectedMessage != "feat: add change" {
		t.Fatalf("selected message = %q, want generated message", selectedMessage)
	}

	localCommit := runGit(t, repoDir, "rev-parse", "HEAD")
	if message := runGit(t, repoDir, "log", "-1", "--format=%s"); message != selectedMessage {
		t.Fatalf("commit message = %q, want %q", message, selectedMessage)
	}
	if push {
		assertRemoteCommit(t, filepath.Join(repoDir, "remote.git"), localCommit)
	}
}

func assertRemoteCommit(t *testing.T, remoteDir, localCommit string) {
	t.Helper()
	remoteCommit := runGit(t, remoteDir, "rev-parse", "refs/heads/feature")
	if remoteCommit != localCommit {
		t.Fatalf("remote commit = %q, want local commit %q", remoteCommit, localCommit)
	}
}

func makeStagedChangeRepository(t *testing.T, withRemote bool) string {
	t.Helper()
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "feature")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	if withRemote {
		remoteDir := filepath.Join(repoDir, "remote.git")
		if err := os.Mkdir(remoteDir, 0o755); err != nil {
			t.Fatalf("Mkdir() error = %v", err)
		}
		runGit(t, remoteDir, "init", "--bare")
		runGit(t, repoDir, "remote", "add", "origin", remoteDir)
	}
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")
	if withRemote {
		runGit(t, repoDir, "push", "-u", "origin", "feature")
	}
	filePath := filepath.Join(repoDir, "change.txt")
	if err := os.WriteFile(filePath, []byte("change\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, repoDir, "add", "change.txt")
	return repoDir
}

func TestRunPRRejectsUnsupportedLanguage(t *testing.T) {
	if err := RunPR(PROptions{Language: "fr"}); err == nil {
		t.Fatal("RunPR() error = nil, want unsupported-language error")
	}
}

func TestRunPRRejectsEmptyCommitRange(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")
	t.Chdir(repoDir)

	err := RunPR(PROptions{Language: "en", Base: "main", Head: "main"})
	if err == nil || !strings.Contains(err.Error(), "no commits found") {
		t.Fatalf("RunPR() error = %v, want empty-range error", err)
	}
}

func TestRunPRReturnsGitLogError(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "feature")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "feature commit")
	t.Chdir(repoDir)

	err := RunPR(PROptions{Language: "en", Base: "missing", Head: "feature"})
	if err == nil || !strings.Contains(err.Error(), "git log missing..feature") {
		t.Fatalf("RunPR() error = %v, want invalid base error", err)
	}
}

func TestRunPRReturnsProviderError(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "base commit")
	runGit(t, repoDir, "checkout", "-b", "feature")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "feat: add branch summary")
	t.Chdir(repoDir)
	withProviderLoading(t, func(<-chan struct{}) func() {
		return func() {}
	})
	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusServiceUnavailable, "provider unavailable"), nil
	}))

	err := RunPR(PROptions{
		Language: "en",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKey:   "sk-test",
		Base:     "main",
		Head:     "feature",
	})
	if err == nil || !strings.Contains(err.Error(), "provider unavailable") {
		t.Fatalf("RunPR() error = %v, want provider error", err)
	}
}

func TestRunPRGeneratesDescriptionFromBranchCommits(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "base commit")
	runGit(t, repoDir, "checkout", "-b", "feature")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "feat: add branch summary")
	t.Chdir(repoDir)

	var gotPrompt string
	loadingStarts := 0
	withProviderHTTPClient(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body struct {
			Input string `json:"input"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			return nil, err
		}
		gotPrompt = body.Input
		return jsonResponse(http.StatusOK, `{"output_text":"feat: summarize branch"}`), nil
	}))
	withProviderLoading(t, func(<-chan struct{}) func() {
		loadingStarts++
		return func() {}
	})
	withProviderEndpoint(t, "openai", "https://example.test/v1/responses")

	output := captureStdout(t, func() {
		err := RunPR(PROptions{
			Language: "en",
			Model:    "gpt-5.5",
			Provider: "openai",
			APIKey:   "sk-test",
			Base:     "main",
			Head:     "feature",
			Context:  "closes issue 18",
		})
		if err != nil {
			t.Fatalf("RunPR() error = %v", err)
		}
	})
	if output != "feat: summarize branch\n" {
		t.Fatalf("RunPR() output = %q, want %q", output, "feat: summarize branch\n")
	}
	if loadingStarts != 1 {
		t.Fatalf("loading starts = %d, want 1 in Interactive mode", loadingStarts)
	}
	if !strings.Contains(gotPrompt, "feat: add branch summary") {
		t.Fatalf("prompt = %q, want branch commit subject", gotPrompt)
	}
	if !strings.Contains(gotPrompt, "closes issue 18") {
		t.Fatalf("prompt = %q, want supplied context", gotPrompt)
	}
}

func TestRunPRAgentSkipsLoading(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "base commit")
	runGit(t, repoDir, "checkout", "-b", "feature")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "feat: add branch summary")
	t.Chdir(repoDir)

	withProviderHTTPClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: summarize branch"}`), nil
	}))
	withProviderEndpoint(t, "openai", "https://example.test/v1/responses")
	loadingStarts := 0
	withProviderLoading(t, func(<-chan struct{}) func() {
		loadingStarts++
		return func() {}
	})

	output := captureStdout(t, func() {
		err := RunPR(PROptions{
			Language: "en",
			Model:    "gpt-5.5",
			Provider: "openai",
			APIKey:   "sk-test",
			Base:     "main",
			Head:     "feature",
			Agent:    true,
		})
		if err != nil {
			t.Fatalf("RunPR() error = %v", err)
		}
	})
	if output != "feat: summarize branch\n" {
		t.Fatalf("RunPR() output = %q, want generated text only", output)
	}
	if loadingStarts != 0 {
		t.Fatalf("loading starts = %d, want 0 in Agent mode", loadingStarts)
	}
}

func TestRunAgentReturnsNoStagedChangesError(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	t.Chdir(repoDir)

	err := Run(Options{Language: "en", Agent: true})
	if err == nil || !strings.Contains(err.Error(), "no staged changes") {
		t.Fatalf("Run() error = %v, want no staged changes error", err)
	}
}

func captureStdout(t *testing.T, run func()) string {
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

func TestGenerateTextRejectsUnknownProvider(t *testing.T) {
	if _, err := generateText(providerOptions{Provider: "custom"}, "prompt"); err == nil {
		t.Fatal("generateText() error = nil, want unsupported-provider error")
	}
}

func TestHostedProviderGenerateReturnsErrors(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		transport roundTripFunc
		wantError string
	}{
		{name: "missing API key", wantError: "api key is required"},
		{
			name:   "request failure",
			apiKey: "sk-test",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("connection failed")
			},
			wantError: "connection failed",
		},
		{
			name:   "server failure",
			apiKey: "sk-test",
			transport: func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusServiceUnavailable, "temporarily unavailable"), nil
			},
			wantError: "temporarily unavailable",
		},
		{
			name:   "invalid response",
			apiKey: "sk-test",
			transport: func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, "{"), nil
			},
			wantError: "unexpected EOF",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := hostedProvider{
				name:     "openai",
				model:    "gpt-5.5",
				apiKey:   test.apiKey,
				endpoint: "https://example.test/v1/responses",
				client:   &http.Client{Transport: test.transport},
				codec:    openAICodec{},
			}

			_, err := provider.Generate("prompt")
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Generate() error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestModelContextLength(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "model-specific context length", body: `{"model_info":{"llama.context_length":4096}}`, want: 4096},
		{name: "generic context length", body: `{"model_info":{"context_length":2048}}`, want: 2048},
		{name: "missing context length", body: `{"model_info":{}}`},
		{name: "invalid response", body: `{`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withDefaultHTTPTransport(t, roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, test.body), nil
			}))

			if got := modelContextLength("test-model"); got != test.want {
				t.Fatalf("modelContextLength() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestTruncateDiff(t *testing.T) {
	tests := []struct {
		name       string
		contextLen int
		wantLength int
	}{
		{name: "truncate to minimum", contextLen: 100, wantLength: 1000},
		{name: "keep diff within context", contextLen: 1000, wantLength: 1500},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withDefaultHTTPTransport(t, roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				body := `{"model_info":{"context_length":` + strconv.Itoa(test.contextLen) + `}}`
				return jsonResponse(http.StatusOK, body), nil
			}))

			got := truncateDiff(strings.Repeat("x", 1500), "test-model", "prompt")
			if len(got) != test.wantLength {
				t.Fatalf("truncateDiff() length = %d, want %d", len(got), test.wantLength)
			}
		})
	}
}

func assertPromptContext(t *testing.T, context string, wantContext bool) {
	t.Helper()
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
		Context: context,
		Diff:    "example diff",
	}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := buf.String()
	hasContext := strings.Contains(got, "## Additional context")
	if hasContext != wantContext {
		t.Fatalf("context section presence = %t, want %t", hasContext, wantContext)
	}
	if wantContext && !strings.Contains(got, context) {
		t.Fatalf("prompt does not contain context %q", context)
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

func TestSelectOptionReturnsCommitError(t *testing.T) {
	commitErr := errors.New("commit failed")
	runSelection := func(string) (selection.Result, error) {
		return selection.Result{Choice: selection.Accept}, nil
	}
	createCommit := func(string) error {
		return commitErr
	}

	withProviderLoading(t, func(done <-chan struct{}) func() {
		return func() {
			<-done
		}
	})
	withProviderHTTPClient(t, roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: test commit"}`), nil
	}))

	tmpl := template.Must(template.New("prompt").Parse("{{.Diff}}"))
	err := selectOption(tmpl, "diff", providerOptions{
		Provider: "openai",
		Model:    "gpt-5.5",
		APIKey:   "sk-test",
	}, "", false, runSelection, createCommit)
	if !errors.Is(err, commitErr) {
		t.Fatalf("selectOption() error = %v, want %v", err, commitErr)
	}
}

func TestRunRegenerateThenAcceptCreatesSecondMessage(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "feature")
	filePath := filepath.Join(repoDir, "change.txt")
	if err := os.WriteFile(filePath, []byte("change\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, repoDir, "add", "change.txt")
	t.Chdir(repoDir)

	generated := 0
	withProviderLoading(t, func(done <-chan struct{}) func() {
		return func() { <-done }
	})
	withProviderHTTPClient(t, roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		switch generated {
		case 0:
			generated++
			return jsonResponse(http.StatusOK, `{"output_text":"feat: first message"}`), nil
		case 1:
			generated++
			return jsonResponse(http.StatusOK, `{"output_text":"feat: second message"}`), nil
		default:
			return nil, errors.New("provider called more than twice")
		}
	}))

	choices := []selection.Choice{selection.Regenerate, selection.Accept}
	selected := 0
	runSelection := func(string) (selection.Result, error) {
		if selected >= len(choices) {
			return selection.Result{}, errors.New("selection called too many times")
		}
		choice := choices[selected]
		selected++
		return selection.Result{Choice: choice}, nil
	}
	var created []string
	createCommit := func(commit string) error {
		created = append(created, commit)
		return nil
	}

	err := run(Options{
		Language: "en",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKey:   "sk-test",
	}, runSelection, createCommit)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if generated != len(choices) {
		t.Fatalf("provider calls = %d, want %d", generated, len(choices))
	}
	if selected != len(choices) {
		t.Fatalf("selection calls = %d, want %d", selected, len(choices))
	}
	if len(created) != 1 || created[0] != "feat: second message" {
		t.Fatalf("created commits = %q, want only %q", created, "feat: second message")
	}
}

func TestSelectOptionReturnsCommitErrorAfterEdit(t *testing.T) {
	commitErr := errors.New("edited commit failed")
	runSelection := func(string) (selection.Result, error) {
		return selection.Result{Choice: selection.Edit}, nil
	}
	createCommit := func(string) error {
		return commitErr
	}

	editorPath := filepath.Join(t.TempDir(), "editor")
	if err := os.WriteFile(editorPath, []byte("#!/bin/sh\nprintf '%s' 'edited commit' > \"$1\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("EDITOR", editorPath)

	withProviderLoading(t, func(done <-chan struct{}) func() {
		return func() {
			<-done
		}
	})
	withProviderHTTPClient(t, roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: test commit"}`), nil
	}))

	tmpl := template.Must(template.New("prompt").Parse("{{.Diff}}"))
	err := selectOption(tmpl, "diff", providerOptions{
		Provider: "openai",
		Model:    "gpt-5.5",
		APIKey:   "sk-test",
	}, "", false, runSelection, createCommit)
	if !errors.Is(err, commitErr) {
		t.Fatalf("selectOption() error = %v, want %v", err, commitErr)
	}
}

func TestWriteAndCloseTemporaryCommitReturnsCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	temporaryFile := &closeErrorWriter{closeErr: closeErr}

	err := writeAndCloseTemporaryCommit(temporaryFile, "commit message")
	if !errors.Is(err, closeErr) {
		t.Fatalf("writeAndCloseTemporaryCommit() error = %v, want %v", err, closeErr)
	}
	if temporaryFile.closeCalls != 1 {
		t.Fatalf("temporary file close calls = %d, want 1", temporaryFile.closeCalls)
	}
	if got := temporaryFile.String(); got != "commit message" {
		t.Fatalf("temporary file content = %q, want commit message", got)
	}
}

type closeErrorWriter struct {
	bytes.Buffer
	closeErr   error
	closeCalls int
}

func (f *closeErrorWriter) Close() error {
	f.closeCalls++
	return f.closeErr
}

func TestRunPushDoesNotPushAfterCommitError(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "feature")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
	filePath := filepath.Join(repoDir, "change.txt")
	if err := os.WriteFile(filePath, []byte("change\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, repoDir, "add", "change.txt")

	commitErr := errors.New("commit failed")
	runSelection := func(string) (selection.Result, error) {
		return selection.Result{Choice: selection.Accept}, nil
	}
	createCommit := func(string) error {
		return commitErr
	}

	withProviderLoading(t, func(done <-chan struct{}) func() {
		return func() {
			<-done
		}
	})
	withProviderHTTPClient(t, roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: test commit"}`), nil
	}))

	t.Chdir(repoDir)
	err := runPush(Options{
		Language: "en",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKey:   "sk-test",
	}, runSelection, createCommit)
	if !errors.Is(err, commitErr) {
		t.Fatalf("RunPush() error = %v, want %v", err, commitErr)
	}
}

func TestRunPushCommitsAndPushesGeneratedMessage(t *testing.T) {
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	repoDir := t.TempDir()
	if err := os.Mkdir(remoteDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	runGit(t, remoteDir, "init", "--bare")
	runGit(t, repoDir, "init", "-b", "feature")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "remote", "add", "origin", remoteDir)
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")
	runGit(t, repoDir, "push", "-u", "origin", "feature")
	if err := os.WriteFile(filepath.Join(repoDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, repoDir, "add", "change.txt")
	t.Chdir(repoDir)

	withProviderLoading(t, func(done <-chan struct{}) func() {
		return func() { <-done }
	})
	withProviderHTTPClient(t, roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"output_text":"feat: push generated change"}`), nil
	}))
	runSelection := func(string) (selection.Result, error) {
		return selection.Result{Choice: selection.Accept}, nil
	}

	err := runPush(Options{
		Language: "en",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKey:   "sk-test",
	}, runSelection, makeCommit)
	if err != nil {
		t.Fatalf("runPush() error = %v", err)
	}
	localCommit := runGit(t, repoDir, "rev-parse", "HEAD")
	remoteCommit := runGit(t, remoteDir, "rev-parse", "refs/heads/feature")
	if remoteCommit != localCommit {
		t.Fatalf("remote commit = %q, want local commit %q", remoteCommit, localCommit)
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
	withProviderHTTPClient(t, roundTripFunc(func(_ *http.Request) (*http.Response, error) {
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

func TestOllamaGenerateReadsStream(t *testing.T) {
	withDefaultHTTPTransport(t, roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, "{\"response\":\"feat: add\"}\n{\"response\":\" stream\",\"done\":true}\n"), nil
	}))

	got, err := (ollamaProvider{model: "llama3.2"}).Generate("prompt")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "feat: add stream" {
		t.Fatalf("Generate() = %q, want %q", got, "feat: add stream")
	}
}

func TestOllamaGenerateReturnsStreamError(t *testing.T) {
	withDefaultHTTPTransport(t, roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"error":"model unavailable"}`), nil
	}))

	_, err := (ollamaProvider{model: "missing"}).Generate("prompt")
	if err == nil || !strings.Contains(err.Error(), "model unavailable") {
		t.Fatalf("Generate() error = %v, want model error", err)
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

func withDefaultHTTPTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = original
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
