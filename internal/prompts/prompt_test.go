package prompts

import (
	"regexp"
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

func TestGetPRPromptsUseBuiltInTemplate(t *testing.T) {
	tests := []struct {
		language                string
		title                   string
		outputFormatHeading     string
		guidelinesHeading       string
		forbiddenSummaryPattern string
		sections                []string
		rules                   []string
	}{
		{
			language:                "en",
			title:                   "Title:",
			outputFormatHeading:     "Output format",
			guidelinesHeading:       "Guidelines",
			forbiddenSummaryPattern: `(?i)\b(?:2|two|a pair of)[\s-]+sentences?\b`,
			sections: []string{
				"Summary",
				"Motivation / Context",
				"Changes",
				"How to test",
				"Breaking changes / Migration notes",
				"Related issues",
			},
			rules: []string{
				"1 to 3 sentences",
				"Explain why the change is necessary",
				"do not state one",
				"Synthesize related commits into themes",
				"bold label",
				"Do not write one bullet per commit",
				"evidence in the input",
				"verification text in commit bodies",
				"test files",
				"CLI commands or flags named in commits",
				"`--context`",
				"do not invent commands, flags, endpoints, URLs, or test names",
				"short, high-level verification approach",
				"omit this section when there are no breaking changes, required migrations, new environment variables, or renamed endpoints",
				"omit this section when the input contains no real issue reference",
				"Closes #N",
				"do not invent issue references",
				"<type>(<scope>): <description>",
				"imperative mood",
				"72 characters",
				"No period at the end",
			},
		},
		{
			language:                "pt-BR",
			title:                   "Titulo:",
			outputFormatHeading:     "Formato de saída",
			guidelinesHeading:       "Diretrizes",
			forbiddenSummaryPattern: `(?i)\b(?:2|duas?)[\s-]+frases?\b`,
			sections: []string{
				"Resumo",
				"Motivação / Contexto",
				"Mudanças",
				"Como testar",
				"Mudanças incompatíveis / Notas de migração",
				"Problemas relacionados",
			},
			rules: []string{
				"1 a 3 frases",
				"Explique por que a mudança é necessária",
				"não informam a motivação",
				"Sintetize commits relacionados em temas",
				"rótulo em negrito",
				"Não escreva um bullet por commit",
				"evidências na entrada",
				"texto de verificação nos corpos dos commits",
				"arquivos de teste",
				"comandos ou flags de CLI mencionados nos commits",
				"`--context`",
				"não invente comandos, flags, endpoints, URLs ou nomes de testes",
				"abordagem breve e de alto nível",
				"Omita esta seção quando não houver mudanças incompatíveis, migrações necessárias, novas variáveis de ambiente ou endpoints renomeados",
				"omita esta seção quando a entrada não contiver uma referência real a issue",
				"Closes #N",
				"não invente referências a issues",
				"<type>(<scope>): <description>",
				"modo imperativo",
				"72 caracteres",
				"sem ponto no final",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			prompt, ok := GetPR(test.language)
			if !ok {
				t.Fatalf("GetPR(%q) ok = false, want true", test.language)
			}

			if !strings.Contains(prompt, test.title) {
				t.Errorf("GetPR(%q) prompt does not contain title prefix %q", test.language, test.title)
			}
			assertHeadingsInOrder(t, prompt, test.outputFormatHeading, test.guidelinesHeading, test.sections)
			assertPromptContains(t, prompt, test.rules)
			assertPromptOmitsPattern(t, prompt, test.forbiddenSummaryPattern)
			assertPromptOmitsTerms(t, prompt, []string{
				"screenshot",
				"type of change",
				"checklist",
			})
		})
	}
}

func assertHeadingsInOrder(t *testing.T, prompt, outputFormatHeading, guidelinesHeading string, headings []string) {
	t.Helper()

	allHeadings := levelTwoHeadings(prompt)
	outputHeadings, foundOutputFormat, foundGuidelines := headingsInOutputFormat(allHeadings, outputFormatHeading, guidelinesHeading)
	if !foundOutputFormat {
		t.Errorf("prompt is missing output heading %q", outputFormatHeading)
	}
	if !foundGuidelines {
		t.Errorf("prompt is missing guidelines heading %q after its output template", guidelinesHeading)
	}
	assertHeadingSequence(t, "output template", outputHeadings, headings)
	assertHeadingSequence(t, "all template headings", matchingHeadings(allHeadings, headings), headings)
}

func levelTwoHeadings(prompt string) []string {
	allHeadings := make([]string, 0)
	for _, line := range strings.Split(prompt, "\n") {
		if strings.HasPrefix(line, "## ") {
			allHeadings = append(allHeadings, strings.TrimPrefix(line, "## "))
		}
	}
	return allHeadings
}

func headingsInOutputFormat(allHeadings []string, outputFormatHeading, guidelinesHeading string) ([]string, bool, bool) {
	outputHeadings := make([]string, 0)
	insideOutputFormat := false
	foundOutputFormat := false
	foundGuidelines := false
	for _, heading := range allHeadings {
		if heading == outputFormatHeading {
			if insideOutputFormat {
				outputHeadings = append(outputHeadings, heading)
			} else if !foundOutputFormat {
				foundOutputFormat = true
				insideOutputFormat = true
			}
			continue
		}
		if insideOutputFormat && heading == guidelinesHeading {
			foundGuidelines = true
			insideOutputFormat = false
			continue
		}
		if insideOutputFormat {
			outputHeadings = append(outputHeadings, heading)
		}
	}
	return outputHeadings, foundOutputFormat, foundGuidelines
}

func matchingHeadings(allHeadings, expectedHeadings []string) []string {
	targetHeadings := make(map[string]struct{}, len(expectedHeadings))
	for _, heading := range expectedHeadings {
		targetHeadings[heading] = struct{}{}
	}

	matchedHeadings := make([]string, 0, len(expectedHeadings))
	for _, heading := range allHeadings {
		if _, isTargetHeading := targetHeadings[heading]; isTargetHeading {
			matchedHeadings = append(matchedHeadings, heading)
		}
	}
	return matchedHeadings
}

func assertHeadingSequence(t *testing.T, name string, got, want []string) {
	t.Helper()

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s headings = %q, want %q", name, got, want)
	}
}

func assertPromptContains(t *testing.T, prompt string, phrases []string) {
	t.Helper()

	prompt = strings.ToLower(prompt)
	for _, phrase := range phrases {
		if !strings.Contains(prompt, strings.ToLower(phrase)) {
			t.Errorf("prompt does not contain required rule %q", phrase)
		}
	}
}

func assertPromptOmitsPattern(t *testing.T, prompt, pattern string) {
	t.Helper()

	if match := regexp.MustCompile(pattern).FindString(prompt); match != "" {
		t.Errorf("prompt contains forbidden summary sentence rule %q", match)
	}
}

func assertPromptOmitsTerms(t *testing.T, prompt string, terms []string) {
	t.Helper()

	prompt = strings.ToLower(prompt)
	for _, term := range terms {
		if strings.Contains(prompt, strings.ToLower(term)) {
			t.Errorf("prompt contains forbidden term %q", term)
		}
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
