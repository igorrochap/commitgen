package generator

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"text/template"

	"github.com/igorrochap/commitgen/internal/prompts"
)

const prDiffstatFileLimit = 100

// PROptions configures pull request description generation.
type PROptions struct {
	Context  string
	Language string
	Model    string
	Provider string
	APIKey   string
	Base     string
	Head     string
	// Agent selects Agent mode's plain output without showing a loading spinner.
	Agent bool
}

// RunPR generates and prints a pull request description.
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
	diffstat, err := getBranchDiffstat(opts.Base, opts.Head)
	if err != nil {
		return err
	}

	tmpl, err := template.New("pr").Parse(prompt)
	if err != nil {
		return err
	}

	result, err := generatePrompt(tmpl, promptData{
		Context:  opts.Context,
		Diff:     log,
		Diffstat: diffstat,
		Branch:   opts.Head,
	}, providerOptions{
		Provider: opts.Provider,
		Model:    opts.Model,
		APIKey:   opts.APIKey,
		Agent:    opts.Agent,
	})
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

func getBranchDiffstat(base, head string) (string, error) {
	ref := base + "..." + head
	cmd := exec.Command("git", "diff", "--stat", fmt.Sprintf("--stat-count=%d", prDiffstatFileLimit), ref)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --stat --stat-count=%d %s: %w", prDiffstatFileLimit, ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DefaultBase returns the first available base branch among main, master, and develop.
func DefaultBase() (string, error) {
	for _, candidate := range []string{"main", "master", "develop"} {
		cmd := exec.Command("git", "rev-parse", "--verify", candidate)
		if err := cmd.Run(); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("could not detect base branch; use --base to specify one")
}

// CurrentBranch returns the name of the current git branch.
func CurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
