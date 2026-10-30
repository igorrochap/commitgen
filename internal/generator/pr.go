package generator

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/igorrochap/commitgen/internal/prompts"
)

const prDiffstatFileLimit = 100

// commitRecordSeparator marks the start of each commit in the branch log.
const commitRecordSeparator = "\x1e"

// squashMergeSuffix matches the pull request number that GitHub appends to
// squash-merged commit subjects, such as "feat: add x (#40)".
var squashMergeSuffix = regexp.MustCompile(`\s*\(#\d+\)$`)

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
	templatePath, repositoryTemplate, err := findRepositoryPRTemplate()
	if err != nil {
		return err
	}
	prompt, err = selectPRPrompt(opts.Language, prompt, templatePath)
	if err != nil {
		return err
	}

	tmpl, err := template.New("pr").Parse(prompt)
	if err != nil {
		return err
	}

	printPRTemplateNotice(opts.Agent, templatePath)

	result, err := generatePrompt(tmpl, promptData{
		Context:            opts.Context,
		Diff:               log,
		Diffstat:           diffstat,
		Branch:             opts.Head,
		RepositoryTemplate: repositoryTemplate,
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

func selectPRPrompt(language, builtInPrompt, templatePath string) (string, error) {
	if templatePath == "" {
		return builtInPrompt, nil
	}
	prompt, ok := prompts.GetPRRepositoryTemplate(language)
	if !ok {
		return "", fmt.Errorf("language %s not supported", language)
	}
	return prompt, nil
}

func printPRTemplateNotice(agent bool, templatePath string) {
	if agent {
		return
	}
	if templatePath == "" {
		fmt.Fprintln(os.Stderr, "Using built-in PR template")
		return
	}
	fmt.Fprintf(os.Stderr, "Using PR template: %s\n", templatePath)
}

func findRepositoryPRTemplate() (string, string, error) {
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	rootPath := strings.TrimSpace(string(root))

	for _, relativePath := range []string{
		".github/pull_request_template.md",
		"pull_request_template.md",
		"docs/pull_request_template.md",
		".gitlab/merge_request_templates/Default.md",
		".azuredevops/pull_request_template.md",
	} {
		foundPath, content, found, err := readRepositoryPRTemplate(rootPath, relativePath)
		if err != nil {
			return "", "", err
		}
		if found {
			return foundPath, content, nil
		}
	}
	return "", "", nil
}

func readRepositoryPRTemplate(rootPath, relativePath string) (string, string, bool, error) {
	directory := filepath.Dir(relativePath)
	directoryPath, exists, err := findRepositoryTemplateDirectory(rootPath, directory)
	if err != nil {
		return "", "", false, err
	}
	if !exists {
		return "", "", false, nil
	}
	entries, err := os.ReadDir(directoryPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("read PR template %s: %w", relativePath, err)
	}

	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), filepath.Base(relativePath)) {
			continue
		}
		foundPath, content, found, err := readRepositoryPRTemplateFile(directoryPath, directory, entry.Name())
		if err != nil {
			return "", "", false, err
		}
		if found {
			return foundPath, content, true, nil
		}
	}
	return "", "", false, nil
}

func readRepositoryPRTemplateFile(directoryPath, directory, filename string) (string, string, bool, error) {
	foundPath := filepath.ToSlash(filepath.Join(directory, filename))
	path := filepath.Join(directoryPath, filename)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("read PR template %s: %w", foundPath, err)
	}
	if !info.Mode().IsRegular() {
		return "", "", false, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", false, fmt.Errorf("read PR template %s: %w", foundPath, err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return "", "", false, nil
	}
	return foundPath, string(content), true, nil
}

func findRepositoryTemplateDirectory(rootPath, relativeDirectory string) (string, bool, error) {
	path := rootPath
	for _, part := range strings.Split(filepath.ToSlash(relativeDirectory), "/") {
		path = filepath.Join(path, part)
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("read PR template directory %s: %w", relativeDirectory, err)
		}
		if !info.IsDir() {
			return "", false, nil
		}
	}
	return path, true, nil
}

func getBranchLog(base, head string) (string, error) {
	ref := base + ".." + head
	cmd := exec.Command("git", "log", ref, "--no-merges", "--format=%x1e%s%n%b")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git log %s: %w", ref, err)
	}
	return stripSquashMergeSuffixes(string(out)), nil
}

// stripSquashMergeSuffixes removes squash-merge pull request numbers from
// commit subjects so the model does not mistake them for issue references.
func stripSquashMergeSuffixes(log string) string {
	records := strings.Split(log, commitRecordSeparator)
	commits := make([]string, 0, len(records))
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		subject, body, _ := strings.Cut(record, "\n")
		subject = squashMergeSuffix.ReplaceAllString(subject, "")
		commits = append(commits, strings.TrimSpace(subject+"\n"+body))
	}
	return strings.Join(commits, "\n\n")
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
