package cmd

import (
	"fmt"

	"github.com/igorrochap/commitgen/internal/updatecheck"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show commitgen version",
	Run: func(cmd *cobra.Command, args []string) {
		printVersion(cmd)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func printVersion(cmd *cobra.Command) {
	fmt.Fprintf(cmd.OutOrStdout(), "commitgen %s\n", updatecheck.CurrentVersion())
}
