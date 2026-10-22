package prompts

import (
	"strings"
	"testing"
)

func TestGetReturnsSupportedCommitPrompts(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{language: "en", want: "Conventional Commit"},
		{language: "pt-BR", want: "Conventional Commit"},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			prompt, ok := Get(test.language)
			if !ok {
				t.Fatalf("Get(%q) ok = false, want true", test.language)
			}
			if !strings.Contains(prompt, test.want) {
				t.Fatalf("Get(%q) prompt does not contain %q", test.language, test.want)
			}
		})
	}
	if prompt, ok := Get("fr"); ok || prompt != "" {
		t.Fatalf("Get(%q) = %q, %t; want empty prompt and false", "fr", prompt, ok)
	}
}

func TestGetPRReturnsSupportedPrompts(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{language: "en", want: "Pull Request description generator"},
		{language: "pt-BR", want: "descrições de Pull Request"},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			prompt, ok := GetPR(test.language)
			if !ok {
				t.Fatalf("GetPR(%q) ok = false, want true", test.language)
			}
			if !strings.Contains(prompt, test.want) {
				t.Fatalf("GetPR(%q) prompt does not contain %q", test.language, test.want)
			}
		})
	}
	if prompt, ok := GetPR("fr"); ok || prompt != "" {
		t.Fatalf("GetPR(%q) = %q, %t; want empty prompt and false", "fr", prompt, ok)
	}
}

func TestIsSupportedReportsPromptLanguages(t *testing.T) {
	for _, test := range []struct {
		language string
		want     bool
	}{
		{language: "en", want: true},
		{language: "pt-BR", want: true},
		{language: "fr", want: false},
	} {
		t.Run(test.language, func(t *testing.T) {
			if got := IsSupported(test.language); got != test.want {
				t.Fatalf("IsSupported(%q) = %t, want %t", test.language, got, test.want)
			}
		})
	}
}
