package generator

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/igorrochap/commitgen/internal/prompts"
)

func TestPromptContext(t *testing.T) {
	tests := []struct {
		name        string
		context     string
		wantContext bool
	}{
		{
			name:        "with context",
			context:     "fix CI failure",
			wantContext: true,
		},
		{
			name: "without context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := getPrompt("en")
			if err != nil {
				t.Fatalf("getPrompt() error = %v", err)
			}
			tmpl, err := template.New("prompt").Parse(prompt)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, promptData{
				Context: tt.context,
				Diff:    "example diff",
			}); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			got := buf.String()
			hasContext := strings.Contains(got, "## Additional context")
			if hasContext != tt.wantContext {
				t.Fatalf("context section presence = %t, want %t", hasContext, tt.wantContext)
			}
			if tt.wantContext && !strings.Contains(got, tt.context) {
				t.Fatalf("prompt does not contain context %q", tt.context)
			}
		})
	}
}

func TestPRPromptContext(t *testing.T) {
	prompt, ok := prompts.GetPR("en")
	if !ok {
		t.Fatal("GetPR() ok = false, want true")
	}
	tmpl, err := template.New("pr").Parse(prompt)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	const context = "closes the issue #15 on github"
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, promptData{
		Context: context,
		Diff:    "example commit log",
	}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "## Additional context") {
		t.Fatal("prompt does not contain additional context section")
	}
	if !strings.Contains(got, context) {
		t.Fatalf("prompt does not contain context %q", context)
	}
}
