package cmd

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	appconfig "github.com/igorrochap/commitgen/internal/config"
	"github.com/igorrochap/commitgen/internal/generator"
	"github.com/spf13/pflag"
)

func TestPushCommandIsRegistered(t *testing.T) {
	for _, command := range rootCmd.Commands() {
		if command == pushCmd {
			return
		}
	}
	t.Fatal("push command is not registered on the root command")
}

func TestPushCommandExposesGenerationFlags(t *testing.T) {
	for _, name := range []string{"context", "language", "model", "provider"} {
		if flag := pushCmd.Flags().Lookup(name); flag == nil {
			t.Fatalf("push command does not expose --%s", name)
		}
	}
}

func TestPushCommandPassesOptionsToGeneration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	withConfigPath(t, path)
	if err := appconfig.SaveFile(path, appconfig.Config{
		Language: "en",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKeys:  map[string]string{"openai": "sk-test"},
	}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	}))
	resetPushCommand(t)
	withTerminalStreams(t, true, true)
	rootCmd.SetArgs([]string{"push", "--context", "release notes", "--version=false"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	wantOptions := generator.Options{
		Context:  "release notes",
		Language: "en",
		Model:    "gpt-5.5",
		Provider: "openai",
		APIKey:   "sk-test",
	}
	wantErr := errors.New("generation failed")
	originalRunPushGeneration := runPushGeneration
	t.Cleanup(func() { runPushGeneration = originalRunPushGeneration })
	runPushGeneration = func(got generator.Options) error {
		if got != wantOptions {
			t.Errorf("generation options = %+v, want %+v", got, wantOptions)
		}
		return wantErr
	}

	if err := rootCmd.Execute(); !errors.Is(err, wantErr) {
		t.Fatalf("push command error = %v, want %v", err, wantErr)
	}
}

func resetPushCommand(t *testing.T) {
	t.Helper()
	pushCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if err := flag.Value.Set(flag.DefValue); err != nil {
			t.Fatalf("reset --%s: %v", flag.Name, err)
		}
		flag.Changed = false
	})
}
