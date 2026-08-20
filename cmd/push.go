package cmd

import (
	appconfig "github.com/igorrochap/commitgen/internal/config"
	"github.com/igorrochap/commitgen/internal/generator"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:          "push",
	Short:        "Generate a commit and push it to the remote",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := effectiveOptions(cmd)
		if err != nil {
			return err
		}
		return generator.RunPush(opts)
	},
}

func init() {
	pushCmd.Flags().StringVar(&context, "context", "", "Additional context for generation")
	pushCmd.Flags().StringVar(&language, "language", appconfig.DefaultLanguage, "Commit language")
	pushCmd.Flags().StringVar(&model, "model", appconfig.DefaultModel, "LLM model")
	pushCmd.Flags().StringVar(&provider, "provider", appconfig.DefaultProvider, "LLM provider (ollama, openai, anthropic, gemini)")
	rootCmd.AddCommand(pushCmd)
}
