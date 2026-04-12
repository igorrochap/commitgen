package generator

import (
	"bytes"
	"commit_generator/internal/loading"
	"commit_generator/internal/prompts"
	"fmt"
	"os/exec"
	"text/template"
)

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
	commit, err := generateCommit(tmpl, diff, option.Model)
	fmt.Print("\033[H\033[2J")
	fmt.Printf("\n%s\n", commit)
	return nil
}

func getPrompt(language string) (string, error) {
	prompt, ok := prompts.Get(language)
	if ok == false {
		return "", fmt.Errorf("language %s not supported", language)
	}
	return prompt, nil
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

	loading.Start(done)
	out, err := cmd.Output()
	close(done)

	if err != nil {
		return "", err
	}
	return string(out), nil
}
