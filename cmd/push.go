package cmd

import (
	"github.com/igorrochap/commitgen/internal/generator"
	"github.com/spf13/cobra"
)

var runPushGeneration = generator.RunPush

var pushCmd = &cobra.Command{
	Use:          "push",
	Short:        "Generate a commit and push it to the remote",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		opts, err := effectiveOptions(cmd, resolvedAgentMode)
		if err != nil {
			return err
		}
		return runPushGeneration(opts)
	},
}

func init() {
	registerGenerationFlags(pushCmd, "Commit language", "LLM model")
	rootCmd.AddCommand(pushCmd)
}
