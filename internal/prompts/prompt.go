// Package prompts provides localized commit message prompts.
package prompts

import _ "embed"

//go:embed en.md
var english string

//go:embed pt_br.md
var ptBr string

//go:embed pr_en.md
var prEnglish string

//go:embed pr_pt_br.md
var prPtBr string

//go:embed pr_repository_en.md
var prRepositoryEnglish string

//go:embed pr_repository_pt_br.md
var prRepositoryPtBr string

var registry = map[string]string{
	"en":    english,
	"pt-BR": ptBr,
}

var prRegistry = map[string]string{
	"en":    prEnglish,
	"pt-BR": prPtBr,
}

var prRepositoryTemplateRegistry = map[string]string{
	"en":    prRepositoryEnglish,
	"pt-BR": prRepositoryPtBr,
}

// Get returns the commit message prompt for lang when it is supported.
func Get(lang string) (string, bool) {
	p, ok := registry[lang]
	return p, ok
}

// GetPR returns the pull request prompt for lang when it is supported.
func GetPR(lang string) (string, bool) {
	p, ok := prRegistry[lang]
	return p, ok
}

// GetPRRepositoryTemplate returns the localized prompt for a repository PR template.
func GetPRRepositoryTemplate(lang string) (string, bool) {
	p, ok := prRepositoryTemplateRegistry[lang]
	return p, ok
}

// IsSupported reports whether lang has a commit message prompt.
func IsSupported(lang string) bool {
	_, ok := registry[lang]
	return ok
}
