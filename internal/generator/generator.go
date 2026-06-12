package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"

	"github.com/igorrochap/commitgen/internal/loading"
	"github.com/igorrochap/commitgen/internal/prompts"
	"github.com/igorrochap/commitgen/internal/selection"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\r`)

var (
	lineEndWord   = regexp.MustCompile(`([\p{L}\p{N}_]+)[ \t]*$`)
	lineStartWord = regexp.MustCompile(`^([\p{L}\p{N}_]+)`)
)

// unwrapLines joins soft line breaks within paragraphs.
// Mid-word duplicates (e.g. "internationalizatio\ninternationalization") are
// collapsed into the full word. Clean word-boundary wraps are joined with a
// space. Paragraph breaks (\n\n) and list item lines (-, *, +) are preserved.
func unwrapLines(s string) string {
	paragraphs := strings.Split(s, "\n\n")
	for i, p := range paragraphs {
		lines := strings.Split(p, "\n")
		if len(lines) <= 1 {
			continue
		}
		result := lines[0]
		for _, line := range lines[1:] {
			trimmed := strings.TrimLeft(line, " \t")
			if len(trimmed) > 0 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') {
				result += "\n" + line
				continue
			}
			endMatch := lineEndWord.FindStringSubmatch(result)
			startMatch := lineStartWord.FindStringSubmatch(trimmed)
			if len(endMatch) > 1 && len(startMatch) > 1 && strings.HasPrefix(startMatch[1], endMatch[1]) {
				result = result[:len(result)-len(endMatch[1])] + trimmed
			} else {
				result += " " + trimmed
			}
		}
		paragraphs[i] = result
	}
	return strings.Join(paragraphs, "\n\n")
}

type Options struct {
	Context  string
	Language string
	Model    string
}

type promptData struct {
	Context string
	Diff    string
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
	err = selectOption(tmpl, diff, option.Model, option.Context)
	return err
}

func getPrompt(language string) (string, error) {
	prompt, ok := prompts.Get(language)
	if ok == false {
		return "", fmt.Errorf("language %s not supported", language)
	}
	return prompt, nil
}

func selectOption(tmpl *template.Template, diff, model, context string) error {
	end := false
	for end == false {
		commit, err := generateCommit(tmpl, diff, model, context)
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
			updatedCommit, err := edit(commit)
			if err != nil {
				return err
			}
			makeCommit(updatedCommit)
			end = true
		}
	}
	return nil
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Think  bool   `json:"think"`
}

type ollamaStreamChunk struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error"`
}

// modelContextLength queries ollama for the model's context window size.
// Returns 0 on failure so the caller can apply a fallback.
func modelContextLength(model string) int {
	body, _ := json.Marshal(map[string]string{"name": model})
	resp, err := http.Post("http://localhost:11434/api/show", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var info struct {
		ModelInfo map[string]any `json:"model_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return 0
	}
	for _, key := range []string{"llama.context_length", "context_length"} {
		if v, ok := info.ModelInfo[key]; ok {
			if f, ok := v.(float64); ok {
				return int(f)
			}
		}
	}
	return 0
}

func truncateDiff(diff, model, promptTemplate string) string {
	contextLen := modelContextLength(model)
	if contextLen == 0 {
		contextLen = 131072
	}
	// conservative: 3 chars/token, reserve space for prompt template
	maxChars := contextLen*3 - len(promptTemplate)
	if maxChars < 1000 {
		maxChars = 1000
	}
	if len(diff) <= maxChars {
		return diff
	}
	fmt.Fprintf(os.Stderr, "warning: diff truncated (%d → %d chars) to fit model context\n", len(diff), maxChars)
	return diff[:maxChars]
}

func generateCommit(tmpl *template.Template, diff, model, context string) (string, error) {
	var templateBuf bytes.Buffer
	if err := tmpl.Execute(&templateBuf, promptData{Context: context}); err != nil {
		return "", err
	}
	diff = truncateDiff(diff, model, templateBuf.String())

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, promptData{Context: context, Diff: diff}); err != nil {
		return "", err
	}

	body, err := json.Marshal(ollamaRequest{
		Model:  model,
		Prompt: buf.String(),
		Stream: true,
		Think:  false,
	})
	if err != nil {
		return "", err
	}

	done := make(chan struct{})
	wait := loading.Start(done)

	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(body))

	close(done)
	wait()

	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama: %s", strings.TrimSpace(string(b)))
	}

	var result strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk ollamaStreamChunk
		if err := decoder.Decode(&chunk); err != nil {
			break
		}
		if chunk.Error != "" {
			return "", fmt.Errorf("ollama: %s", chunk.Error)
		}
		result.WriteString(chunk.Response)
		if chunk.Done {
			break
		}
	}

	clean := ansiEscape.ReplaceAllString(result.String(), "")
	clean = unwrapLines(clean)
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

func edit(commit string) (string, error) {
	tmp, err := os.CreateTemp("", "commit-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(commit); err != nil {
		return "", err
	}
	tmp.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		if _, err := exec.LookPath("nano"); err == nil {
			editor = "nano"
		} else {
			editor = "vim"
		}
	}

	editCmd := exec.Command(editor, tmp.Name())
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr
	if err := editCmd.Run(); err != nil {
		return "", nil
	}

	content, err := os.ReadFile(tmp.Name())
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(content)), nil
}
