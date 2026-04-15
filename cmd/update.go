package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const modulePath = "github.com/igorrochap/commit-generator"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update commitgen to the latest version",
	Long:  "Downloads and installs the latest version of commitgen via `go install`.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("go"); err != nil {
			return fmt.Errorf("go toolchain not found in PATH; install Go to use `commitgen update`")
		}

		currentPath, _ := os.Executable()

		fmt.Println("Updating commitgen...")
		install := exec.Command("go", "install", modulePath+"@latest")
		install.Stdout = os.Stdout
		install.Stderr = os.Stderr
		if err := install.Run(); err != nil {
			return fmt.Errorf("go install failed: %w", err)
		}

		gopathOut, err := exec.Command("go", "env", "GOPATH").Output()
		if err != nil {
			return fmt.Errorf("could not resolve GOPATH: %w", err)
		}
		gopath := strings.TrimSpace(string(gopathOut))
		newBinary := gopath + "/bin/commitgen"

		fmt.Printf("Installed to %s\n", newBinary)

		if currentPath != "" && currentPath != newBinary {
			fmt.Printf("\nHeads up: your active binary lives at %s\n", currentPath)
			fmt.Printf("To replace it, run:\n  sudo cp %s %s\n", newBinary, currentPath)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
