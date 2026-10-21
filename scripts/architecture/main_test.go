package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrokenRule(t *testing.T) {
	tests := []struct {
		name     string
		importer string
		imported string
		wantRule string
	}{
		{
			name:     "internal package imports command",
			importer: modulePath + "/internal/updatecheck",
			imported: modulePath + "/cmd",
			wantRule: "R1",
		},
		{
			name:     "internal package imports command subpackage",
			importer: modulePath + "/internal/updatecheck",
			imported: modulePath + "/cmd/config",
			wantRule: "R1",
		},
		{
			name:     "leaf package imports module package",
			importer: modulePath + "/internal/config",
			imported: modulePath + "/internal/prompts",
			wantRule: "R2",
		},
		{
			name:     "generator imports disallowed module package",
			importer: modulePath + "/internal/generator",
			imported: modulePath + "/internal/config",
			wantRule: "R3",
		},
		{
			name:     "non-command package imports cobra",
			importer: modulePath + "/internal/generator",
			imported: "github.com/spf13/cobra",
			wantRule: "R4",
		},
		{
			name:     "non-terminal package imports charmbracelet",
			importer: modulePath + "/internal/generator",
			imported: "github.com/charmbracelet/bubbletea",
			wantRule: "R5",
		},
		{
			name:     "main imports disallowed module package",
			importer: modulePath,
			imported: modulePath + "/internal/config",
			wantRule: "R6",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rule, violatesRule := brokenRule(test.importer, test.imported)
			if !violatesRule {
				t.Fatal("expected an architecture violation")
			}
			if rule.id != test.wantRule {
				t.Fatalf("got rule %s, want %s", rule.id, test.wantRule)
			}
		})
	}
}

func TestBrokenRuleAllowsDeclaredDependencies(t *testing.T) {
	tests := []struct {
		name     string
		importer string
		imported string
	}{
		{
			name:     "loading imports terminal UI dependency",
			importer: modulePath + "/internal/loading",
			imported: "github.com/charmbracelet/bubbles",
		},
		{
			name:     "command imports cobra",
			importer: modulePath + "/cmd",
			imported: "github.com/spf13/cobra",
		},
		{
			name:     "generator imports declared module dependency",
			importer: modulePath + "/internal/generator",
			imported: modulePath + "/internal/prompts",
		},
		{
			name:     "main imports command",
			importer: modulePath,
			imported: modulePath + "/cmd",
		},
		{
			name:     "generator test imports its package",
			importer: modulePath + "/internal/generator",
			imported: modulePath + "/internal/generator",
		},
		{
			name:     "config test imports its package",
			importer: modulePath + "/internal/config",
			imported: modulePath + "/internal/config",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, violatesRule := brokenRule(test.importer, test.imported); violatesRule {
				t.Fatalf("unexpected violation: %s imports %s", test.importer, test.imported)
			}
		})
	}
}

func TestFindViolationsIncludesTestFilesAndSkipsNonPackageDirectories(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/updatecheck/updatecheck_test.go", `package updatecheck_test

import _ "github.com/igorrochap/commitgen/cmd"
`)
	writeGoFile(t, root, "internal/generator/generator_test.go", `package generator_test

import _ "github.com/igorrochap/commitgen/internal/generator"
`)
	writeGoFile(t, root, "internal/config/config_test.go", `package config_test

import _ "github.com/igorrochap/commitgen/internal/config"
`)
	writeGoFile(t, root, ".hidden/ignored.go", `package ignored

import _ "github.com/spf13/cobra"
`)
	writeGoFile(t, root, "_fixtures/ignored.go", `package ignored

import _ "github.com/spf13/cobra"
`)
	writeGoFile(t, root, "testdata/ignored.go", `package ignored

import _ "github.com/spf13/cobra"
`)
	writeGoFile(t, root, "vendor/example/ignored.go", `package ignored

import _ "github.com/spf13/cobra"
`)

	violations, err := findViolations(root)
	if err != nil {
		t.Fatalf("find violations: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(violations))
	}
	if violations[0].file != "internal/updatecheck/updatecheck_test.go" {
		t.Fatalf("got file %s", violations[0].file)
	}
	if violations[0].rule.id != "R1" {
		t.Fatalf("got rule %s, want R1", violations[0].rule.id)
	}
}

func TestFindViolationsSortsResultsByFileRuleAndImport(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/config/config.go", `package config

import _ "github.com/igorrochap/commitgen/internal/prompts"
`)
	writeGoFile(t, root, "internal/generator/imports.go", `package generator

import (
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/charmbracelet/bubbles"
	_ "github.com/spf13/cobra"
)
`)

	violations, err := findViolations(root)
	if err != nil {
		t.Fatalf("find violations: %v", err)
	}
	want := []struct {
		file     string
		imported string
		ruleID   string
	}{
		{
			file:     "internal/config/config.go",
			imported: modulePath + "/internal/prompts",
			ruleID:   "R2",
		},
		{
			file:     "internal/generator/imports.go",
			imported: "github.com/spf13/cobra",
			ruleID:   "R4",
		},
		{
			file:     "internal/generator/imports.go",
			imported: "github.com/charmbracelet/bubbles",
			ruleID:   "R5",
		},
		{
			file:     "internal/generator/imports.go",
			imported: "github.com/charmbracelet/bubbletea",
			ruleID:   "R5",
		},
	}
	if len(violations) != len(want) {
		t.Fatalf("got %d violations, want %d", len(violations), len(want))
	}
	for index, expected := range want {
		got := violations[index]
		if got.file != expected.file || got.imported != expected.imported || got.rule.id != expected.ruleID {
			t.Errorf("violation %d = (%s, %s, %s), want (%s, %s, %s)",
				index,
				got.file,
				got.imported,
				got.rule.id,
				expected.file,
				expected.imported,
				expected.ruleID,
			)
		}
	}
}

func TestFindViolationsReportsInvalidGoFile(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "invalid.go", "package invalid\n\nimport (\n")

	_, err := findViolations(root)
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if !strings.Contains(err.Error(), "parse imports in") {
		t.Fatalf("got error %q, want import parse context", err)
	}
}

func TestRunPrintsEveryViolation(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/config/config.go", `package config

import _ "github.com/igorrochap/commitgen/internal/prompts"
`)
	writeGoFile(t, root, "internal/generator/generator.go", `package generator

import _ "github.com/igorrochap/commitgen/internal/config"
`)
	changeWorkingDirectory(t, root)

	var exitCode int
	output := captureStdout(t, func() {
		exitCode = run()
	})
	if exitCode != 1 {
		t.Fatalf("violating tree returned exit code %d, want 1", exitCode)
	}
	if strings.Count(output, "FAIL ") != 2 {
		t.Fatalf("got output %q, want both violations", output)
	}
	if !strings.Contains(output, "R2") || !strings.Contains(output, "R3") {
		t.Fatalf("output %q does not name both broken rules", output)
	}
}

func TestFindViolationsReturnsWalkError(t *testing.T) {
	missingRoot := filepath.Join(t.TempDir(), "missing")
	if _, err := findViolations(missingRoot); err == nil {
		t.Fatal("expected a walk error")
	}
}

func TestSkipDirectory(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "vendor", want: true},
		{name: "testdata", want: true},
		{name: ".hidden", want: true},
		{name: "_generated", want: true},
		{name: "internal", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := skipDirectory(test.name); got != test.want {
				t.Fatalf("got %t, want %t", got, test.want)
			}
		})
	}
}

func TestPackagePath(t *testing.T) {
	if got := packagePath("."); got != modulePath {
		t.Fatalf("got %s, want %s", got, modulePath)
	}
	if got := packagePath(filepath.Join("internal", "config")); got != modulePath+"/internal/config" {
		t.Fatalf("got %s, want %s", got, modulePath+"/internal/config")
	}
}

func TestRunReportsPassAndViolation(t *testing.T) {
	cleanRoot := t.TempDir()
	changeWorkingDirectory(t, cleanRoot)
	if exitCode := run(); exitCode != 0 {
		t.Fatalf("clean tree returned exit code %d, want 0", exitCode)
	}

	violationRoot := t.TempDir()
	writeGoFile(t, violationRoot, "internal/config/config.go", `package config

import _ "github.com/igorrochap/commitgen/internal/prompts"
`)
	changeWorkingDirectory(t, violationRoot)
	if exitCode := run(); exitCode != 1 {
		t.Fatalf("violating tree returned exit code %d, want 1", exitCode)
	}

	invalidRoot := t.TempDir()
	writeGoFile(t, invalidRoot, "invalid.go", "package invalid\n\nimport (\n")
	changeWorkingDirectory(t, invalidRoot)
	if exitCode := run(); exitCode != 1 {
		t.Fatalf("invalid tree returned exit code %d, want 1", exitCode)
	}
}

func writeGoFile(t *testing.T, root, relativePath, contents string) {
	t.Helper()

	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %s: %v", relativePath, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", relativePath, err)
	}
}

func changeWorkingDirectory(t *testing.T, directory string) {
	t.Helper()

	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatalf("change working directory to %s: %v", directory, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Errorf("restore working directory to %s: %v", previousDirectory, err)
		}
	})
}

func captureStdout(t *testing.T, action func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create output pipe: %v", err)
	}
	previousStdout := os.Stdout
	os.Stdout = writer
	action()
	if err := writer.Close(); err != nil {
		t.Fatalf("close output pipe: %v", err)
	}
	os.Stdout = previousStdout
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read output pipe: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close output reader: %v", err)
	}
	return string(output)
}
