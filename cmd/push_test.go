package cmd

import "testing"

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
