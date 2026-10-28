package cmd

import (
	"bytes"
	"errors"
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

func TestConfigSetRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		flag  string
		value string
	}{
		{name: "empty language", flag: "language", value: " "},
		{name: "empty model", flag: "model", value: " "},
		{name: "empty provider", flag: "provider", value: " "},
		{name: "unsupported provider", flag: "provider", value: "custom"},
		{name: "empty API key", flag: "api-key", value: " "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withConfigPath(t, filepath.Join(t.TempDir(), "config.json"))
			cmd := newConfigSetTestCommand()
			if err := cmd.Flags().Set(test.flag, test.value); err != nil {
				t.Fatalf("Set(%s) error = %v", test.flag, err)
			}
			if err := runConfigSet(cmd, nil); err == nil {
				t.Fatalf("runConfigSet() error = nil, want invalid %s error", test.flag)
			}
		})
	}
}

func TestConfigSetPropagatesConfigPathError(t *testing.T) {
	pathErr := errors.New("config path unavailable")
	withConfigPathError(t, pathErr)
	cmd := newConfigSetTestCommand()
	if err := cmd.Flags().Set("language", "en"); err != nil {
		t.Fatalf("Set(language) error = %v", err)
	}

	if err := runConfigSet(cmd, nil); !errors.Is(err, pathErr) {
		t.Fatalf("runConfigSet() error = %v, want %v", err, pathErr)
	}
}

func TestConfigSetPropagatesLoadError(t *testing.T) {
	withConfigPath(t, t.TempDir())
	cmd := newConfigSetTestCommand()
	if err := cmd.Flags().Set("language", "en"); err != nil {
		t.Fatalf("Set(language) error = %v", err)
	}

	if err := runConfigSet(cmd, nil); err == nil {
		t.Fatal("runConfigSet() error = nil, want config read error")
	}
}

func TestConfigSetPropagatesOutputError(t *testing.T) {
	withConfigPath(t, filepath.Join(t.TempDir(), "config.json"))
	cmd := newConfigSetTestCommand()
	cmd.SetOut(commandErrorWriter{})
	if err := cmd.Flags().Set("model", "gpt-5.5"); err != nil {
		t.Fatalf("Set(model) error = %v", err)
	}

	if err := runConfigSet(cmd, nil); err == nil {
		t.Fatal("runConfigSet() error = nil, want output error")
	}
}

func TestConfigSetSavesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	cmd := newConfigSetTestCommand()
	setFlags(t, cmd, map[string]string{
		"language": "pt-BR",
		"model":    "llama3.2",
		"provider": "openai",
		"api-key":  "sk-test",
	})

	if err := runConfigSet(cmd, nil); err != nil {
		t.Fatalf("runConfigSet() error = %v", err)
	}

	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	assertStringValue(t, "Language", cfg.Language, "pt-BR")
	assertStringValue(t, "Model", cfg.Model, "llama3.2")
	assertStringValue(t, "Provider", cfg.Provider, "openai")
	assertStringValue(t, "APIKeys[openai]", cfg.APIKeys["openai"], "sk-test")
}

func TestConfigSetStoresAPIKeyUnderSelectedProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{
		Language: "en",
		Model:    "gemini-3.5-flash-lite",
		Provider: "gemini",
		APIKeys: map[string]string{
			"gemini": "gemini-key",
		},
	}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	cmd := newConfigSetTestCommand()
	setFlags(t, cmd, map[string]string{
		"provider": "openai",
		"api-key":  "openai-key",
	})
	if err := runConfigSet(cmd, nil); err != nil {
		t.Fatalf("runConfigSet() error = %v", err)
	}

	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	assertStringValue(t, "Provider", cfg.Provider, "openai")
	assertStringValue(t, "APIKeys[openai]", cfg.APIKeys["openai"], "openai-key")
	assertStringValue(t, "APIKeys[gemini]", cfg.APIKeys["gemini"], "gemini-key")
}

func TestConfigShowDisplaysAPIKeyStatus(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		wantStatus string
	}{
		{name: "configured", apiKey: "gemini-key", wantStatus: "set"},
		{name: "missing", wantStatus: "unset"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			withConfigPath(t, path)
			cfg := appconfig.Config{
				Language: "pt-BR",
				Model:    "gemini-3.5-flash-lite",
				Provider: "gemini",
			}
			if test.apiKey != "" {
				cfg.APIKeys = map[string]string{"gemini": test.apiKey}
			}
			if err := appconfig.SaveFile(path, cfg); err != nil {
				t.Fatalf("SaveFile() error = %v", err)
			}

			var output bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&output)
			if err := runConfigShow(cmd, nil); err != nil {
				t.Fatalf("runConfigShow() error = %v", err)
			}

			want := "language=pt-BR\nmodel=gemini-3.5-flash-lite\nprovider=gemini\n" +
				"api_key=" + test.wantStatus + "\nconfig=" + path + "\n"
			if got := output.String(); got != want {
				t.Fatalf("runConfigShow() output = %q, want %q", got, want)
			}
		})
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

	opts, err := effectiveOptions(cmd, false)
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
	setFlags(t, cmd, map[string]string{
		"context":  "fix CI failure",
		"language": "en",
		"model":    "gemma3",
		"provider": "gemini",
	})

	opts, err := effectiveOptions(cmd, false)
	if err != nil {
		t.Fatalf("effectiveOptions() error = %v", err)
	}

	assertStringValue(t, "Language", opts.Language, "en")
	assertStringValue(t, "Context", opts.Context, "fix CI failure")
	assertStringValue(t, "Model", opts.Model, "gemma3")
	assertStringValue(t, "Provider", opts.Provider, "gemini")
	assertStringValue(t, "APIKey", opts.APIKey, "gemini-key")
}

func TestEffectiveOptionsRejectsHostedProviderWithoutConfiguredAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{Language: "en", Model: "gpt-5.5", Provider: "openai"}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	cmd := newRootTestCommand()

	if _, err := effectiveOptions(cmd, false); err == nil {
		t.Fatal("effectiveOptions() error = nil, want missing api key error")
	}
}

func TestEffectiveOptionsRejectsUnsupportedSavedProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{Language: "en", Model: "model", Provider: "custom"}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	if _, err := effectiveOptions(newRootTestCommand(), false); err == nil {
		t.Fatal("effectiveOptions() error = nil, want unsupported-provider error")
	}
}

func TestEffectiveOptionsPropagatesConfigErrors(t *testing.T) {
	t.Run("path error", func(t *testing.T) {
		pathErr := errors.New("config path unavailable")
		withConfigPathError(t, pathErr)
		if _, err := effectiveOptions(newRootTestCommand(), false); !errors.Is(err, pathErr) {
			t.Fatalf("effectiveOptions() error = %v, want %v", err, pathErr)
		}
	})

	t.Run("load error", func(t *testing.T) {
		withConfigPath(t, t.TempDir())
		if _, err := effectiveOptions(newRootTestCommand(), false); err == nil {
			t.Fatal("effectiveOptions() error = nil, want config read error")
		}
	})
}

func TestConfigShowPropagatesOutputError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Defaults()); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	cmd := &cobra.Command{}
	cmd.SetOut(commandErrorWriter{})

	if err := runConfigShow(cmd, nil); err == nil {
		t.Fatal("runConfigShow() error = nil, want output error")
	}
}

func TestConfigShowPropagatesStorageErrors(t *testing.T) {
	t.Run("path error", func(t *testing.T) {
		pathErr := errors.New("config path unavailable")
		withConfigPathError(t, pathErr)
		if err := runConfigShow(&cobra.Command{}, nil); !errors.Is(err, pathErr) {
			t.Fatalf("runConfigShow() error = %v, want %v", err, pathErr)
		}
	})

	t.Run("load error", func(t *testing.T) {
		withConfigPath(t, t.TempDir())
		if err := runConfigShow(&cobra.Command{}, nil); err == nil {
			t.Fatal("runConfigShow() error = nil, want config read error")
		}
	})
}

func TestUpdateConfigAPIKeyInitializesMissingMap(t *testing.T) {
	cmd := newConfigSetTestCommand()
	if err := cmd.Flags().Set("api-key", "openai-key"); err != nil {
		t.Fatalf("Set(api-key) error = %v", err)
	}
	cfg := appconfig.Config{Provider: "openai"}

	if err := updateConfigAPIKey(cmd, &cfg); err != nil {
		t.Fatalf("updateConfigAPIKey() error = %v", err)
	}
	if got := cfg.APIKeys["openai"]; got != "openai-key" {
		t.Fatalf("APIKeys[openai] = %q, want openai-key", got)
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

func withConfigPathError(t *testing.T, pathErr error) {
	t.Helper()
	original := configPath
	configPath = func() (string, error) {
		return "", pathErr
	}
	t.Cleanup(func() {
		configPath = original
	})
}

func setFlags(t *testing.T, cmd *cobra.Command, values map[string]string) {
	t.Helper()
	for name, value := range values {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}
}

func assertStringValue(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

type commandErrorWriter struct{}

func (commandErrorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
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
