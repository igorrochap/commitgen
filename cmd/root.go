/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"time"

	appconfig "github.com/igorrochap/commitgen/internal/config"
	"github.com/igorrochap/commitgen/internal/generator"
	"github.com/igorrochap/commitgen/internal/updatecheck"

	"github.com/spf13/cobra"
)

var (
	language string
	model    string
	version  bool
)

var configPath = appconfig.Path

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "commitgen",
	Short:        "Generate commits based on changes made in the project",
	Long:         `Commit generator helps you to generate commits using the conventional commit pattern. It uses an LLM to generate the commit for you to review`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if version {
			return
		}
		notifyUpdate(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if version {
			printVersion(cmd)
			return nil
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
	rootCmd.Flags().StringVar(&language, "language", appconfig.DefaultLanguage, "Commit language")
	rootCmd.Flags().StringVar(&model, "model", appconfig.DefaultModel, "Ollama model")
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

	return generator.Options{Language: cfg.Language, Model: cfg.Model}, nil
}

func notifyUpdate(cmd *cobra.Command) {
	result, err := updatecheck.CheckWithTimeout(750 * time.Millisecond)
	if err != nil || !result.Newer {
		return
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "A new commitgen version is available: %s -> %s\nRun `commitgen update` to upgrade.\n\n", result.Current, result.Latest)
}
