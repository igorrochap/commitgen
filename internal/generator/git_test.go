package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
