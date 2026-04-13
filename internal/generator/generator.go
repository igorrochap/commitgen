package generator

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"text/template"

	"github.com/igorrochap/commit-generator/internal/loading"
	"github.com/igorrochap/commit-generator/internal/prompts"
	"github.com/igorrochap/commit-generator/internal/selection"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\r`)

type Options struct {
	Language string
	Model    string
}

func Run(option Options) error {
	prompt, err := getPrompt(option.Language)
	if err != nil {
		return err
	}
	diff, err := GetDiff()
	if err != nil {
		return err
	}
	tmpl, err := template.New("prompt").Parse(prompt)
	if err != nil {
		return err
	}
	err = selectOption(tmpl, diff, option.Model)
	return err
}

func getPrompt(language string) (string, error) {
	prompt, ok := prompts.Get(language)
	if ok == false {
		return "", fmt.Errorf("language %s not supported", language)
	}
	return prompt, nil
}

func selectOption(tmpl *template.Template, diff string, model string) error {
	end := false
	for end == false {
		commit, err := generateCommit(tmpl, diff, model)
		if err != nil {
			return err
		}
		result, err := selection.Run(commit)
		if err != nil {
			return err
		}
		switch result.Choice {
		case selection.Accept:
			makeCommit(commit)
			end = true
		case selection.Edit:
			//TODO: edit commit
			fmt.Println("Editing commit message")
			end = true
		}
	}
	return nil
}

func generateCommit(tmpl *template.Template, diff string, model string) (string, error) {
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, map[string]string{"Diff": diff})
	if err != nil {
		return "", err
	}
	cmd := exec.Command("ollama", "run", model, "--hidethinking")
	cmd.Stdin = &buf

	done := make(chan struct{})

	wait := loading.Start(done)
	out, err := cmd.Output()
	close(done)
	wait()

	if err != nil {
		return "", err
	}
	clean := ansiEscape.ReplaceAllString(string(out), "")
	return strings.TrimSpace(clean), nil
}

func makeCommit(commit string) error {
	commitCmd := exec.Command("git", "commit", "-m", commit)
	err := commitCmd.Run()
	if err != nil {
		return err
	}
	getIdCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	id, err := getIdCmd.Output()
	if err != nil {
		return err
	}
	fmt.Printf("Commit %s created\n", strings.TrimSpace(string(id)))
	return nil
}
