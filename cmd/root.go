// Package cmd provides the commitgen command-line interface.
package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	appconfig "github.com/igorrochap/commitgen/internal/config"
	"github.com/igorrochap/commitgen/internal/generator"
	"github.com/igorrochap/commitgen/internal/updatecheck"

	"github.com/spf13/cobra"
)

var (
	context  string
	language string
	model    string
	provider string
	version  bool
)

var configPath = appconfig.Path

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "commitgen",
	Short:        "Generate commits based on changes made in the project",
	Long:         `Commit generator helps you to generate commits using the conventional commit pattern. It uses an LLM to generate the commit for you to review`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		if version {
			return
		}
		notifyUpdate(cmd)
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		if version {
			return printVersion(cmd)
		}

		opts, err := effectiveOptions(cmd)
		if err != nil {
			return err
		}
		return generator.Run(opts)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&version, "version", "v", false, "Show commitgen version")
	registerGenerationFlags(rootCmd, "Commit language", "LLM model")
}

func registerGenerationFlags(cmd *cobra.Command, languageHelp, modelHelp string) {
	cmd.Flags().StringVar(&context, "context", "", "Additional context for generation")
	cmd.Flags().StringVar(&language, "language", appconfig.DefaultLanguage, languageHelp)
	cmd.Flags().StringVar(&model, "model", appconfig.DefaultModel, modelHelp)
	cmd.Flags().StringVar(&provider, "provider", appconfig.DefaultProvider, "LLM provider (ollama, openai, anthropic, gemini)")
}

func effectiveOptions(cmd *cobra.Command) (generator.Options, error) {
	path, err := configPath()
	if err != nil {
		return generator.Options{}, err
	}

	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		return generator.Options{}, err
	}

	if cmd.Flags().Changed("language") {
		cfg.Language = language
	}
	if cmd.Flags().Changed("model") {
		cfg.Model = model
	}
	if cmd.Flags().Changed("provider") {
		cfg.Provider = strings.ToLower(strings.TrimSpace(provider))
	}
	if !appconfig.IsSupportedProvider(cfg.Provider) {
		return generator.Options{}, fmt.Errorf("provider %s not supported", cfg.Provider)
	}

	selectedAPIKey := cfg.APIKeys[cfg.Provider]
	if cfg.Provider != appconfig.DefaultProvider && strings.TrimSpace(selectedAPIKey) == "" {
		return generator.Options{}, fmt.Errorf("api key for provider %s is not configured; run `commitgen config set --provider %s --api-key <key>`", cfg.Provider, cfg.Provider)
	}

	return generator.Options{
		Context:  context,
		Language: cfg.Language,
		Model:    cfg.Model,
		Provider: cfg.Provider,
		APIKey:   selectedAPIKey,
	}, nil
}

func notifyUpdate(cmd *cobra.Command) {
	result, err := updatecheck.CheckWithTimeout(750 * time.Millisecond)
	if err != nil || !result.Newer {
		return
	}

	// The update notice is advisory, so a failed write must not block the command.
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "A new commitgen version is available: %s -> %s\nRun `commitgen update` to upgrade.\n\n", result.Current, result.Latest)
}
