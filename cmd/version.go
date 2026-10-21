package cmd

import (
	"fmt"

	"github.com/igorrochap/commitgen/internal/updatecheck"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show commitgen version",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return printVersion(cmd)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func printVersion(cmd *cobra.Command) error {
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "commitgen %s\n", updatecheck.CurrentVersion())
	return err
}
