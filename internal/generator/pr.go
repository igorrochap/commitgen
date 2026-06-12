package generator

import (
	"fmt"
	"os/exec"
	"strings"
	"text/template"

	"github.com/igorrochap/commitgen/internal/prompts"
)

type PROptions struct {
	Context  string
	Language string
	Model    string
	Base     string
	Head     string
}

func RunPR(opts PROptions) error {
	prompt, ok := prompts.GetPR(opts.Language)
	if !ok {
		return fmt.Errorf("language %s not supported", opts.Language)
	}

	log, err := getBranchLog(opts.Base, opts.Head)
	if err != nil {
		return err
	}
	if strings.TrimSpace(log) == "" {
		return fmt.Errorf("no commits found between %s and %s", opts.Base, opts.Head)
	}

	tmpl, err := template.New("pr").Parse(prompt)
	if err != nil {
		return err
	}

	result, err := generateCommit(tmpl, log, opts.Model, opts.Context)
	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}

func getBranchLog(base, head string) (string, error) {
	ref := base + ".." + head
	cmd := exec.Command("git", "log", ref, "--no-merges", "--format=%s%n%b")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git log %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func DefaultBase() (string, error) {
	for _, candidate := range []string{"main", "master", "develop"} {
		cmd := exec.Command("git", "rev-parse", "--verify", candidate)
		if err := cmd.Run(); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not detect base branch; use --base to specify one")
}

func CurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
