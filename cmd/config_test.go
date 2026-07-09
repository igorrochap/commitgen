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
	if err := cmd.Flags().Set("provider", "openai"); err != nil {
		t.Fatalf("Set(provider) error = %v", err)
	}
	if err := cmd.Flags().Set("api-key", "sk-test"); err != nil {
		t.Fatalf("Set(api-key) error = %v", err)
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
	if cfg.Provider != "openai" {
		t.Fatalf("Provider = %q, want openai", cfg.Provider)
	}
	if cfg.APIKeys["openai"] != "sk-test" {
		t.Fatalf("APIKeys[openai] = %q, want sk-test", cfg.APIKeys["openai"])
	}
}

func TestConfigSetPreservesExistingValueOnPartialUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{Language: "en", Model: "llama3.2", Provider: "ollama"}); err != nil {
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
	if err := appconfig.SaveFile(path, appconfig.Config{
		Language: "pt-BR",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKeys: map[string]string{
			"openai": "sk-test",
		},
	}); err != nil {
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
	if opts.Model != "gpt-5.5" {
		t.Fatalf("Model = %q, want gpt-5.5", opts.Model)
	}
	if opts.Provider != "openai" {
		t.Fatalf("Provider = %q, want openai", opts.Provider)
	}
	if opts.APIKey != "sk-test" {
		t.Fatalf("APIKey = %q, want sk-test", opts.APIKey)
	}
}

func TestEffectiveOptionsFlagsOverrideSavedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{
		Language: "pt-BR",
		Model:    "llama3.2",
		Provider: "ollama",
		APIKeys: map[string]string{
			"gemini": "gemini-key",
		},
	}); err != nil {
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
	if err := cmd.Flags().Set("provider", "gemini"); err != nil {
		t.Fatalf("Set(provider) error = %v", err)
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
	if opts.Provider != "gemini" {
		t.Fatalf("Provider = %q, want gemini", opts.Provider)
	}
	if opts.APIKey != "gemini-key" {
		t.Fatalf("APIKey = %q, want gemini-key", opts.APIKey)
	}
}

func TestEffectiveOptionsRejectsHostedProviderWithoutConfiguredAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{Language: "en", Model: "gpt-5.5", Provider: "openai"}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	cmd := newRootTestCommand()

	if _, err := effectiveOptions(cmd); err == nil {
		t.Fatal("effectiveOptions() error = nil, want missing api key error")
	}
}

func TestAPIKeyFlagOnlyExistsOnConfigSet(t *testing.T) {
	if flag := rootCmd.Flags().Lookup("api-key"); flag != nil {
		t.Fatal("root command exposes api-key flag, want config-only")
	}
	if flag := prCmd.Flags().Lookup("api-key"); flag != nil {
		t.Fatal("pr command exposes api-key flag, want config-only")
	}
	if flag := configSetCmd.Flags().Lookup("api-key"); flag == nil {
		t.Fatal("config set command does not expose api-key flag")
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
	configProvider = ""
	configAPIKey = ""
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.Flags().StringVar(&configLanguage, "language", "", "Default commit language")
	cmd.Flags().StringVar(&configModel, "model", "", "Default Ollama model")
	cmd.Flags().StringVar(&configProvider, "provider", "", "Default LLM provider")
	cmd.Flags().StringVar(&configAPIKey, "api-key", "", "Provider API key")
	return cmd
}

func newRootTestCommand() *cobra.Command {
	context = ""
	language = appconfig.DefaultLanguage
	model = appconfig.DefaultModel
	provider = appconfig.DefaultProvider
	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&context, "context", "", "Additional context for generation")
	cmd.Flags().StringVar(&language, "language", appconfig.DefaultLanguage, "Commit language")
	cmd.Flags().StringVar(&model, "model", appconfig.DefaultModel, "Ollama model")
	cmd.Flags().StringVar(&provider, "provider", appconfig.DefaultProvider, "LLM provider")
	return cmd
}
