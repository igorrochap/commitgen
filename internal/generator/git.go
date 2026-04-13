package generator

import (
	"errors"
	"fmt"
	"os/exec"
)

func GetDiff() (string, error) {
	staging, err := haveStagingChanges()
	if err != nil {
		return "", err
	}

	addAllChanges(staging)

	staging, err = haveStagingChanges()
	if !staging {
		return "", fmt.Errorf("There is no changes to commit")
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

func addAllChanges(staging bool) {
	if !staging {
		fmt.Println("Adding all changes to the staging")
		addCmd := exec.Command("git", "add", ".")
		addCmd.Run()
	}
}
