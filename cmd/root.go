/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"commit_generator/internal/generator"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	language string
	model    string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "commitgen",
	Short: "Generate commits based on changes made in the project",
	Long:  `Commit generator helps you to generate commits using the conventional commit pattern. It uses an LLM to generate the commit for you to review`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := generator.Options{Language: language, Model: model}
		err := generator.Run(opts)
		return err
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
	rootCmd.Flags().StringVar(&language, "language", "en", "Commit language")
	rootCmd.Flags().StringVar(&model, "model", "glm-5:cloud", "Ollama model")
}
