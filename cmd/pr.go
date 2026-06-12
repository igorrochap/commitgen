package cmd

import (
	appconfig "github.com/igorrochap/commitgen/internal/config"
	"github.com/igorrochap/commitgen/internal/generator"
	"github.com/spf13/cobra"
)

var (
	prBase string
	prHead string
)

var prCmd = &cobra.Command{
	Use:          "pr",
	Short:        "Generate a Pull Request title and description",
	Long:         `Generate a Pull Request title and description based on the commits between two branches using an LLM.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := effectiveOptions(cmd)
		if err != nil {
			return err
		}

		base := prBase
		if base == "" {
			detected, err := generator.DefaultBase()
			if err != nil {
				return err
			}
			base = detected
		}

		head := prHead
		if head == "" {
			detected, err := generator.CurrentBranch()
			if err != nil {
				return err
			}
			head = detected
		}

		return generator.RunPR(generator.PROptions{
			Context:  opts.Context,
			Language: opts.Language,
			Model:    opts.Model,
			Base:     base,
			Head:     head,
		})
	},
}

func init() {
	prCmd.Flags().StringVar(&prBase, "base", "", "Base branch (default: main/master/develop)")
	prCmd.Flags().StringVar(&context, "context", "", "Additional context for generation")
	prCmd.Flags().StringVar(&prHead, "head", "", "Head branch (default: current branch)")
	prCmd.Flags().StringVar(&language, "language", appconfig.DefaultLanguage, "Output language")
	prCmd.Flags().StringVar(&model, "model", appconfig.DefaultModel, "Ollama model")
	rootCmd.AddCommand(prCmd)
}
