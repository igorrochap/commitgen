package cmd

import (
	"fmt"
	"strings"

	appconfig "github.com/igorrochap/commitgen/internal/config"
	"github.com/igorrochap/commitgen/internal/prompts"
	"github.com/spf13/cobra"
)

var (
	configLanguage string
	configModel    string
	configProvider string
	configAPIKey   string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage commitgen defaults",
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set default language or model",
	RunE:  runConfigSet,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current defaults",
	RunE:  runConfigShow,
}

func init() {
	configSetCmd.Flags().StringVar(&configLanguage, "language", "", "Default commit language")
	configSetCmd.Flags().StringVar(&configModel, "model", "", "Default LLM model")
	configSetCmd.Flags().StringVar(&configProvider, "provider", "", "Default LLM provider (ollama, openai, anthropic, gemini)")
	configSetCmd.Flags().StringVar(&configAPIKey, "api-key", "", "Provider API key")

	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	languageChanged := cmd.Flags().Changed("language")
	modelChanged := cmd.Flags().Changed("model")
	providerChanged := cmd.Flags().Changed("provider")
	apiKeyChanged := cmd.Flags().Changed("api-key")
	if !languageChanged && !modelChanged && !providerChanged && !apiKeyChanged {
		return fmt.Errorf("provide --language, --model, --provider, --api-key, or a combination")
	}

	path, err := configPath()
	if err != nil {
		return err
	}
	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		return err
	}

	if languageChanged {
		if strings.TrimSpace(configLanguage) == "" {
			return fmt.Errorf("language cannot be empty")
		}
		if !prompts.IsSupported(configLanguage) {
			return fmt.Errorf("language %s not supported", configLanguage)
		}
		cfg.Language = configLanguage
	}

	if modelChanged {
		configModel = strings.TrimSpace(configModel)
		if configModel == "" {
			return fmt.Errorf("model cannot be empty")
		}
		cfg.Model = configModel
	}

	if providerChanged {
		configProvider = strings.ToLower(strings.TrimSpace(configProvider))
		if configProvider == "" {
			return fmt.Errorf("provider cannot be empty")
		}
		if !appconfig.IsSupportedProvider(configProvider) {
			return fmt.Errorf("provider %s not supported", configProvider)
		}
		cfg.Provider = configProvider
	}

	if apiKeyChanged {
		configAPIKey = strings.TrimSpace(configAPIKey)
		if configAPIKey == "" {
			return fmt.Errorf("api key cannot be empty")
		}
		if cfg.APIKeys == nil {
			cfg.APIKeys = map[string]string{}
		}
		cfg.APIKeys[cfg.Provider] = configAPIKey
	}

	if err := appconfig.SaveFile(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Defaults updated: language=%s model=%s provider=%s\n", cfg.Language, cfg.Model, cfg.Provider)
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		return err
	}

	apiKeyStatus := "unset"
	if cfg.APIKeys[cfg.Provider] != "" {
		apiKeyStatus = "set"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "language=%s\nmodel=%s\nprovider=%s\napi_key=%s\nconfig=%s\n", cfg.Language, cfg.Model, cfg.Provider, apiKeyStatus, path)
	return nil
}
