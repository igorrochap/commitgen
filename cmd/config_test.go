package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	appconfig "github.com/igorrochap/commitgen/internal/config"
	"github.com/spf13/cobra"
)

func TestConfigSetRejectsEmptyInvocation(t *testing.T) {
	withConfigPath(t, filepath.Join(t.TempDir(), "config.json"))
	cmd := newConfigSetTestCommand()

	if err := runConfigSet(cmd, nil); err == nil {
		t.Fatal("runConfigSet() error = nil, want error")
	}
}

func TestConfigSetRejectsUnsupportedLanguage(t *testing.T) {
	withConfigPath(t, filepath.Join(t.TempDir(), "config.json"))
	cmd := newConfigSetTestCommand()
	if err := cmd.Flags().Set("language", "fr"); err != nil {
		t.Fatalf("Set(language) error = %v", err)
	}

	if err := runConfigSet(cmd, nil); err == nil {
		t.Fatal("runConfigSet() error = nil, want error")
	}
}

func TestConfigSetSavesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	cmd := newConfigSetTestCommand()
	if err := cmd.Flags().Set("language", "pt-BR"); err != nil {
		t.Fatalf("Set(language) error = %v", err)
	}
	if err := cmd.Flags().Set("model", "llama3.2"); err != nil {
		t.Fatalf("Set(model) error = %v", err)
	}

	if err := runConfigSet(cmd, nil); err != nil {
		t.Fatalf("runConfigSet() error = %v", err)
	}

	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if cfg.Language != "pt-BR" {
		t.Fatalf("Language = %q, want pt-BR", cfg.Language)
	}
	if cfg.Model != "llama3.2" {
		t.Fatalf("Model = %q, want llama3.2", cfg.Model)
	}
}

func TestConfigSetPreservesExistingValueOnPartialUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{Language: "en", Model: "llama3.2"}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	cmd := newConfigSetTestCommand()
	if err := cmd.Flags().Set("language", "pt-BR"); err != nil {
		t.Fatalf("Set(language) error = %v", err)
	}

	if err := runConfigSet(cmd, nil); err != nil {
		t.Fatalf("runConfigSet() error = %v", err)
	}

	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if cfg.Language != "pt-BR" {
		t.Fatalf("Language = %q, want pt-BR", cfg.Language)
	}
	if cfg.Model != "llama3.2" {
		t.Fatalf("Model = %q, want llama3.2", cfg.Model)
	}
}

func TestEffectiveOptionsUsesSavedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{Language: "pt-BR", Model: "llama3.2"}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	cmd := newRootTestCommand()

	opts, err := effectiveOptions(cmd)
	if err != nil {
		t.Fatalf("effectiveOptions() error = %v", err)
	}

	if opts.Language != "pt-BR" {
		t.Fatalf("Language = %q, want pt-BR", opts.Language)
	}
	if opts.Model != "llama3.2" {
		t.Fatalf("Model = %q, want llama3.2", opts.Model)
	}
}

func TestEffectiveOptionsFlagsOverrideSavedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{Language: "pt-BR", Model: "llama3.2"}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	cmd := newRootTestCommand()
	if err := cmd.Flags().Set("context", "fix CI failure"); err != nil {
		t.Fatalf("Set(context) error = %v", err)
	}
	if err := cmd.Flags().Set("language", "en"); err != nil {
		t.Fatalf("Set(language) error = %v", err)
	}
	if err := cmd.Flags().Set("model", "gemma3"); err != nil {
		t.Fatalf("Set(model) error = %v", err)
	}

	opts, err := effectiveOptions(cmd)
	if err != nil {
		t.Fatalf("effectiveOptions() error = %v", err)
	}

	if opts.Language != "en" {
		t.Fatalf("Language = %q, want en", opts.Language)
	}
	if opts.Context != "fix CI failure" {
		t.Fatalf("Context = %q, want fix CI failure", opts.Context)
	}
	if opts.Model != "gemma3" {
		t.Fatalf("Model = %q, want gemma3", opts.Model)
	}
}

func withConfigPath(t *testing.T, path string) {
	t.Helper()
	original := configPath
	configPath = func() (string, error) {
		return path, nil
	}
	t.Cleanup(func() {
		configPath = original
	})
}

func newConfigSetTestCommand() *cobra.Command {
	configLanguage = ""
	configModel = ""
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.Flags().StringVar(&configLanguage, "language", "", "Default commit language")
	cmd.Flags().StringVar(&configModel, "model", "", "Default Ollama model")
	return cmd
}

func newRootTestCommand() *cobra.Command {
	context = ""
	language = appconfig.DefaultLanguage
	model = appconfig.DefaultModel
	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&context, "context", "", "Additional context for generation")
	cmd.Flags().StringVar(&language, "language", appconfig.DefaultLanguage, "Commit language")
	cmd.Flags().StringVar(&model, "model", appconfig.DefaultModel, "Ollama model")
	return cmd
}
