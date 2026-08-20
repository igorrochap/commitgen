package generator

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func GetDiff() (string, error) {
	staging, err := haveStagingChanges()
	if err != nil {
		return "", err
	}
	if !staging {
		return "", fmt.Errorf("There is not staged changes. Please add the files you want to commit and run commitgen again.")
	}

	var diff string
	diffCmd := exec.Command("git", "diff", "--cached")
	out, err := diffCmd.Output()
	if err != nil {
		return "", err
	}
	diff = string(out)

	return diff, nil
}

func haveStagingChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, err
	}
	return false, nil
}

func Push() error {
	branch, err := CurrentBranch()
	if err != nil {
		return err
	}

	args := []string{"push"}
	if !hasUpstream() {
		remote, err := firstRemote()
		if err != nil {
			return err
		}
		args = append(args, "-u", remote, branch)
	}

	pushCmd := exec.Command("git", args...)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

func hasUpstream() bool {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	return cmd.Run() == nil
}

func firstRemote() (string, error) {
	cmd := exec.Command("git", "remote")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git remote: %w", err)
	}

	remotes := strings.Fields(string(output))
	if len(remotes) == 0 {
		return "", errors.New("no git remote configured")
	}
	return remotes[0], nil
}
