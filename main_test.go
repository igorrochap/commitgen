package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestMainExecutesVersionCommand(t *testing.T) {
	previousArgs := os.Args
	previousStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	os.Args = []string{"commitgen", "--version"}
	os.Stdout = writer
	t.Cleanup(func() {
		os.Args = previousArgs
		os.Stdout = previousStdout
	})

	main()

	if err := writer.Close(); err != nil {
		t.Fatalf("Close(stdout) error = %v", err)
	}
	os.Stdout = previousStdout
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close(reader) error = %v", err)
	}
	if !strings.HasPrefix(string(output), "commitgen ") {
		t.Fatalf("main() output = %q, want version output", output)
	}
}
