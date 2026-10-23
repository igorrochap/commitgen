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

	if err := printVersion(cmd); err != nil {
		t.Fatalf("printVersion() error = %v", err)
	}

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
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printVersion(cmd)
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			if version {
				return printVersion(cmd)
			}
			*called = true
			return nil
		},
	}
	cmd.SetOut(out)
	cmd.PersistentFlags().BoolVarP(&version, "version", "v", false, "Show commitgen version")
	return cmd
}
