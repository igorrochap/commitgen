// Package main implements the repository's architecture dependency checker.
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const modulePath = "github.com/igorrochap/commitgen"

type architectureRule struct {
	id         string
	constraint string
	rationale  string
}

type violation struct {
	file     string
	imported string
	rule     architectureRule
}

var architectureRules = map[string]architectureRule{
	// R1 keeps internal packages reusable without depending on command wiring.
	"R1": {
		id:         "R1",
		constraint: "internal packages cannot import cmd",
		rationale:  "keeps internal packages independent from the CLI entry point",
	},
	// R2 keeps the leaf packages reusable and independent from module packages.
	"R2": {
		id:         "R2",
		constraint: "leaf packages cannot import other module packages",
		rationale:  "keeps foundational packages independent and easy to reuse",
	},
	// R3 limits generator orchestration to its three collaborators.
	"R3": {
		id:         "R3",
		constraint: "generator can import only loading, prompts, and selection",
		rationale:  "keeps generation focused on its declared collaborators",
	},
	// R4 keeps command-line framework details at the command boundary.
	"R4": {
		id:         "R4",
		constraint: "only cmd can import github.com/spf13/cobra",
		rationale:  "keeps Cobra-specific wiring at the CLI boundary",
	},
	// R5 confines terminal UI dependencies to packages designed for terminal interaction.
	"R5": {
		id:         "R5",
		constraint: "only loading and selection can import github.com/charmbracelet/*",
		rationale:  "keeps terminal UI code behind packages that can isolate TTY behavior",
	},
	// R6 keeps the executable entry point as a small delegate to the command package.
	"R6": {
		id:         "R6",
		constraint: "main can import only cmd from this module",
		rationale:  "keeps application wiring out of the executable entry point",
	},
}

func main() {
	os.Exit(run())
}

func run() int {
	violations, err := findViolations(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL architecture checker: %v\n", err)
		return 1
	}

	if len(violations) == 0 {
		fmt.Println("PASS package dependency rules R1-R6")
		return 0
	}

	for _, violation := range violations {
		fmt.Printf("FAIL %s imports %s: %s %s; %s\n",
			violation.file,
			violation.imported,
			violation.rule.id,
			violation.rule.constraint,
			violation.rule.rationale,
		)
	}
	return 1
}

func findViolations(root string) ([]violation, error) {
	var violations []violation
	fileSet := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		fileViolations, err := collectWalkedFileViolations(root, path, entry, walkErr, fileSet)
		if err != nil {
			return err
		}
		violations = append(violations, fileViolations...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].file != violations[j].file {
			return violations[i].file < violations[j].file
		}
		if violations[i].rule.id != violations[j].rule.id {
			return violations[i].rule.id < violations[j].rule.id
		}
		return violations[i].imported < violations[j].imported
	})

	return violations, nil
}

func collectWalkedFileViolations(
	root string,
	path string,
	entry fs.DirEntry,
	walkErr error,
	fileSet *token.FileSet,
) ([]violation, error) {
	if walkErr != nil {
		return nil, walkErr
	}
	if entry.IsDir() && path != root && skipDirectory(entry.Name()) {
		return nil, filepath.SkipDir
	}
	if entry.IsDir() || filepath.Ext(path) != ".go" {
		return nil, nil
	}

	return checkFile(root, path, fileSet)
}

func checkFile(root, path string, fileSet *token.FileSet) ([]violation, error) {
	parsedFile, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parse imports in %s: %w", path, err)
	}

	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return nil, fmt.Errorf("make path relative for %s: %w", path, err)
	}
	importer := packagePath(filepath.Dir(relativePath))
	var violations []violation
	for _, importSpec := range parsedFile.Imports {
		imported, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("read import in %s: %w", path, err)
		}

		rule, violatesRule := brokenRule(importer, imported)
		if !violatesRule {
			continue
		}

		violations = append(violations, violation{
			file:     filepath.ToSlash(relativePath),
			imported: imported,
			rule:     rule,
		})
	}

	return violations, nil
}

func skipDirectory(name string) bool {
	return name == "vendor" || name == "testdata" ||
		strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func packagePath(directory string) string {
	if directory == "." {
		return modulePath
	}
	return modulePath + "/" + filepath.ToSlash(directory)
}

func brokenRule(importer, imported string) (architectureRule, bool) {
	commandPackage := modulePath + "/cmd"
	rule, violatesRule := brokenInternalRule(importer, imported)
	if violatesRule {
		return rule, true
	}

	rule, violatesRule = brokenExternalRule(importer, imported)
	if violatesRule {
		return rule, true
	}

	if importer == modulePath && isModuleImport(imported) && imported != commandPackage {
		return architectureRules["R6"], true
	}

	return architectureRule{}, false
}

func brokenInternalRule(importer, imported string) (architectureRule, bool) {
	if strings.HasPrefix(importer, modulePath+"/internal/") && isCommandPackage(imported) {
		return architectureRules["R1"], true
	}

	if isLeafPackage(importer) && isModuleImport(imported) && imported != importer {
		return architectureRules["R2"], true
	}

	if importer == modulePath+"/internal/generator" && isModuleImport(imported) && imported != importer &&
		!isGeneratorDependency(imported) {
		return architectureRules["R3"], true
	}

	return architectureRule{}, false
}

func brokenExternalRule(importer, imported string) (architectureRule, bool) {
	commandPackage := modulePath + "/cmd"
	loadingPackage := modulePath + "/internal/loading"
	selectionPackage := modulePath + "/internal/selection"

	if imported == "github.com/spf13/cobra" && importer != commandPackage {
		return architectureRules["R4"], true
	}

	if strings.HasPrefix(imported, "github.com/charmbracelet/") &&
		importer != loadingPackage && importer != selectionPackage {
		return architectureRules["R5"], true
	}

	return architectureRule{}, false
}

func isLeafPackage(importer string) bool {
	switch importer {
	case modulePath + "/internal/config",
		modulePath + "/internal/prompts",
		modulePath + "/internal/loading",
		modulePath + "/internal/selection",
		modulePath + "/internal/updatecheck":
		return true
	default:
		return false
	}
}

func isModuleImport(imported string) bool {
	return imported == modulePath || strings.HasPrefix(imported, modulePath+"/")
}

func isCommandPackage(imported string) bool {
	return imported == modulePath+"/cmd" || strings.HasPrefix(imported, modulePath+"/cmd/")
}

func isGeneratorDependency(imported string) bool {
	return imported == modulePath+"/internal/loading" ||
		imported == modulePath+"/internal/prompts" ||
		imported == modulePath+"/internal/selection"
}
