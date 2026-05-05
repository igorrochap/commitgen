package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPrintVersion(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	printVersion(cmd)

	got := out.String()
	if !strings.HasPrefix(got, "commitgen ") {
		t.Fatalf("printVersion() = %q, want commitgen prefix", got)
	}
}

func TestVersionFlagSkipsGeneration(t *testing.T) {
	tests := []string{"--version", "-v"}

	for _, flag := range tests {
		t.Run(flag, func(t *testing.T) {
			var out bytes.Buffer
			called := false
			cmd := newVersionFlagTestCommand(&out, &called)
			cmd.SetArgs([]string{flag})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if called {
				t.Fatal("generation path was called")
			}
			if !strings.HasPrefix(out.String(), "commitgen ") {
				t.Fatalf("output = %q, want commitgen prefix", out.String())
			}
		})
	}
}

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{Use: "commitgen"}
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})
	cmd.AddCommand(&cobra.Command{
		Use: "version",
		Run: func(cmd *cobra.Command, args []string) {
			printVersion(cmd)
		},
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.HasPrefix(out.String(), "commitgen ") {
		t.Fatalf("output = %q, want commitgen prefix", out.String())
	}
}

func newVersionFlagTestCommand(out *bytes.Buffer, called *bool) *cobra.Command {
	version = false
	cmd := &cobra.Command{
		Use: "commitgen",
		RunE: func(cmd *cobra.Command, args []string) error {
			if version {
				printVersion(cmd)
				return nil
			}
			*called = true
			return nil
		},
	}
	cmd.SetOut(out)
	cmd.PersistentFlags().BoolVarP(&version, "version", "v", false, "Show commitgen version")
	return cmd
}
