package apps

import (
	"fmt"
	"os/exec"
	"runtime"
)

func Open(appName string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", appName)
	case "darwin":
		cmd = exec.Command("open", "-a", appName)
	case "linux":
		cmd = exec.Command(appName)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to open application '%s': %w", appName, err)
	}

	return nil
}
