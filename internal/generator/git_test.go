package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetDiffNoStagedChanges(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "feature")
	t.Chdir(repoDir)

	_, err := GetDiff()
	if err == nil {
		t.Fatal("GetDiff() error = nil, want no staged changes error")
	}
	want := "no staged changes; add the files you want to commit and run commitgen again"
	if err.Error() != want {
		t.Fatalf("GetDiff() error = %q, want %q", err, want)
	}
}

func TestGetDiffReturnsStagedChanges(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "feature")
	if err := os.WriteFile(filepath.Join(repoDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, repoDir, "add", "change.txt")
	t.Chdir(repoDir)

	diff, err := GetDiff()
	if err != nil {
		t.Fatalf("GetDiff() error = %v", err)
	}
	if !strings.Contains(diff, "+change") {
		t.Fatalf("GetDiff() = %q, want staged content", diff)
	}
}

func TestHaveStagingChangesReturnsGitErrorOutsideRepository(t *testing.T) {
	t.Chdir(t.TempDir())

	if _, err := haveStagingChanges(); err == nil {
		t.Fatal("haveStagingChanges() error = nil, want git error")
	}
}

func TestMakeCommitCreatesCommit(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "feature")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repoDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, repoDir, "add", "change.txt")
	t.Chdir(repoDir)

	if err := makeCommit("feat: add change"); err != nil {
		t.Fatalf("makeCommit() error = %v", err)
	}
	if got := runGit(t, repoDir, "log", "-1", "--format=%s"); got != "feat: add change" {
		t.Fatalf("commit subject = %q, want %q", got, "feat: add change")
	}
}

func TestBranchHelpers(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "base subject\n\nbase body")
	runGit(t, repoDir, "checkout", "-b", "feature")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "feature subject\n\nfeature body")
	t.Chdir(repoDir)

	if got, err := CurrentBranch(); err != nil || got != "feature" {
		t.Fatalf("CurrentBranch() = %q, %v; want feature, nil", got, err)
	}
	if got, err := DefaultBase(); err != nil || got != "main" {
		t.Fatalf("DefaultBase() = %q, %v; want main, nil", got, err)
	}
	log, err := getBranchLog("main", "feature")
	if err != nil {
		t.Fatalf("getBranchLog() error = %v", err)
	}
	if !strings.Contains(log, "feature subject") || strings.Contains(log, "base subject") {
		t.Fatalf("getBranchLog() = %q, want only feature commit", log)
	}
	if _, err := getBranchLog("missing", "feature"); err == nil {
		t.Fatal("getBranchLog() error = nil, want invalid-ref error")
	}
}

func TestBranchLogStripsSquashMergePullRequestSuffix(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "base subject")
	runGit(t, repoDir, "checkout", "-b", "feature")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "feat: add templates (#40)")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "fix: handle (#12) in the middle\n\nCloses #35 (#7)")
	t.Chdir(repoDir)

	log, err := getBranchLog("main", "feature")
	if err != nil {
		t.Fatalf("getBranchLog() error = %v", err)
	}
	if strings.Contains(log, "#40") {
		t.Errorf("getBranchLog() = %q, want subject suffix (#40) removed", log)
	}
	for _, want := range []string{"feat: add templates", "fix: handle (#12) in the middle", "Closes #35 (#7)"} {
		if !strings.Contains(log, want) {
			t.Errorf("getBranchLog() = %q, want it to contain %q", log, want)
		}
	}
}

func TestDefaultBaseReturnsErrorWithoutCandidate(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "feature")
	t.Chdir(repoDir)

	if _, err := DefaultBase(); err == nil {
		t.Fatal("DefaultBase() error = nil, want missing-base error")
	}
}

func TestDefaultBaseFindsFallbackBranches(t *testing.T) {
	for _, branch := range []string{"master", "develop"} {
		t.Run(branch, func(t *testing.T) {
			repoDir := t.TempDir()
			runGit(t, repoDir, "init", "-b", branch)
			runGit(t, repoDir, "config", "user.email", "test@example.com")
			runGit(t, repoDir, "config", "user.name", "Test User")
			runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")
			t.Chdir(repoDir)

			got, err := DefaultBase()
			if err != nil {
				t.Fatalf("DefaultBase() error = %v", err)
			}
			if got != branch {
				t.Fatalf("DefaultBase() = %q, want %q", got, branch)
			}
		})
	}
}

func TestCurrentBranchReturnsErrorOutsideRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := CurrentBranch(); err == nil {
		t.Fatal("CurrentBranch() error = nil, want git error")
	}
}

func TestPushSetsUpstreamForCurrentBranch(t *testing.T) {
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	repoDir := t.TempDir()
	if err := os.Mkdir(remoteDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	runGit(t, remoteDir, "init", "--bare")
	runGit(t, repoDir, "init", "-b", "feature")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "remote", "add", "work", remoteDir)
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")

	t.Chdir(repoDir)
	if err := Push(); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	if got := runGit(t, repoDir, "config", "--get", "branch.feature.remote"); got != "work" {
		t.Fatalf("branch.feature.remote = %q, want work", got)
	}
	if got := runGit(t, repoDir, "config", "--get", "branch.feature.merge"); got != "refs/heads/feature" {
		t.Fatalf("branch.feature.merge = %q, want refs/heads/feature", got)
	}
}

func TestPushUsesConfiguredUpstream(t *testing.T) {
	tempDir := t.TempDir()
	originDir := filepath.Join(tempDir, "origin.git")
	backupDir := filepath.Join(tempDir, "backup.git")
	repoDir := filepath.Join(tempDir, "repo")
	for _, dir := range []string{originDir, backupDir, repoDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Mkdir(%q) error = %v", dir, err)
		}
	}
	runGit(t, originDir, "init", "--bare")
	runGit(t, backupDir, "init", "--bare")
	runGit(t, repoDir, "init", "-b", "feature")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "remote", "add", "origin", originDir)
	runGit(t, repoDir, "remote", "add", "backup", backupDir)
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")
	runGit(t, repoDir, "push", "-u", "backup", "feature")
	initialID := runGit(t, backupDir, "rev-parse", "refs/heads/feature")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "second")

	t.Chdir(repoDir)
	if err := Push(); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	if got := runGit(t, backupDir, "rev-parse", "refs/heads/feature"); got == initialID {
		t.Fatal("configured upstream was not updated")
	}
	if remoteHasBranch(t, originDir, "feature") {
		t.Fatal("push unexpectedly used the first remote instead of the configured upstream")
	}
}

func TestPushReturnsErrorWithoutRemote(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "feature")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")

	t.Chdir(repoDir)
	err := Push()
	if err == nil {
		t.Fatal("Push() error = nil, want missing remote error")
	}
	if !strings.Contains(err.Error(), "no git remote configured") {
		t.Fatalf("Push() error = %q, want missing remote message", err)
	}
}

func TestPushKeepsLocalCommitWhenRemoteRejects(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "remote.git")
	repoDir := filepath.Join(tempDir, "repo")
	otherRepoDir := filepath.Join(tempDir, "other-repo")
	if err := os.Mkdir(remoteDir, 0o755); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", remoteDir, err)
	}
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", repoDir, err)
	}
	runGit(t, remoteDir, "init", "--bare")
	runGit(t, repoDir, "init", "-b", "feature")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "remote", "add", "origin", remoteDir)
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")
	runGit(t, repoDir, "push", "-u", "origin", "feature")

	runGit(t, tempDir, "clone", "-b", "feature", remoteDir, otherRepoDir)
	runGit(t, otherRepoDir, "config", "user.email", "other@example.com")
	runGit(t, otherRepoDir, "config", "user.name", "Other User")
	runGit(t, otherRepoDir, "commit", "--allow-empty", "-m", "remote change")
	runGit(t, otherRepoDir, "push")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "local change")
	localID := runGit(t, repoDir, "rev-parse", "HEAD")

	t.Chdir(repoDir)
	if err := Push(); err == nil {
		t.Fatal("Push() error = nil, want rejection error")
	}

	if got := runGit(t, repoDir, "rev-parse", "HEAD"); got != localID {
		t.Fatalf("local HEAD = %q, want unchanged %q", got, localID)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func remoteHasBranch(t *testing.T, dir, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = dir
	return cmd.Run() == nil
}
