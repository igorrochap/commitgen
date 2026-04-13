package generator

import (
	"bytes"
	"commit_generator/internal/loading"
	"commit_generator/internal/prompts"
	"commit_generator/internal/selection"
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
			//TODO: run commit
			fmt.Println("Commit <id> created")
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
	return string(out), nil
}
