//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func pickExcelFileNative() (string, error) {
	return "", nil
}

func showFatalError(title, msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, msg)
}

func launchDesktopGUI(url string, _ func()) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", url)
		} else {
			return nil
		}
	}
	return cmd.Start()
}

func hasNativePicker() bool {
	return false
}
