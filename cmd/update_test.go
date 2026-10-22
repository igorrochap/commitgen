package cmd

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateCommandRunsGoInstallAndPrintsInstallLocation(t *testing.T) {
	binDir := t.TempDir()
	gopath := filepath.Join(t.TempDir(), "go path")
	logPath := filepath.Join(t.TempDir(), "go-args")
	goExecutable := filepath.Join(binDir, "go")
	script := "#!/bin/sh\ncase \"$1\" in\n" +
		"install) printf '%s\\n' \"$*\" > \"$COMMITGEN_UPDATE_LOG\" ;;\n" +
		"env) printf '%s\\n' \"$COMMITGEN_UPDATE_GOPATH\" ;;\n" +
		"*) exit 1 ;;\nesac\n"
	if err := os.WriteFile(goExecutable, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("COMMITGEN_UPDATE_LOG", logPath)
	t.Setenv("COMMITGEN_UPDATE_GOPATH", gopath)
	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return updateResponse(`{"Version":"v1.1.0"}`), nil
	}))

	output := captureCommandStdout(t, func() {
		rootCmd.SetArgs([]string{"update"})
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("update command error = %v", err)
		}
	})
	newBinary := filepath.Join(gopath, "bin", "commitgen")
	if !strings.Contains(output, "Updating commitgen...\nInstalled to "+newBinary+"\n") {
		t.Fatalf("update output = %q, want install status and path", output)
	}
	if !strings.Contains(output, "To replace it, run:\n  sudo cp "+newBinary+" ") {
		t.Fatalf("update output = %q, want active binary replacement instructions", output)
	}
	args, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := strings.TrimSpace(string(args)), "install "+modulePath+"@latest"; got != want {
		t.Fatalf("go install args = %q, want %q", got, want)
	}
}

func TestUpdateCommandReportsMissingGoToolchain(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	withUpdateTransport(t, commandRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return updateResponse(`{"Version":"v1.1.0"}`), nil
	}))
	rootCmd.SetArgs([]string{"update"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	if err := rootCmd.Execute(); err == nil || !strings.Contains(err.Error(), "go toolchain not found in PATH") {
		t.Fatalf("update command error = %v, want missing toolchain error", err)
	}
}
