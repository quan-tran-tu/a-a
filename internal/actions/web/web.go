package web

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
)

// Search opens the default web browser with an encoded search query.
func Search(query string) error {
	searchURL := fmt.Sprintf("https://www.google.com/search?q=%s", url.QueryEscape(query))
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", searchURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", searchURL)
	case "darwin":
		cmd = exec.Command("open", searchURL)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}
	return nil
}
