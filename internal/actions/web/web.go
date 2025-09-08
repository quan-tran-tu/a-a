package web

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
)

func Search(ctx context.Context, query string) error {
	searchURL := fmt.Sprintf("https://www.google.com/search?q=%s", url.QueryEscape(query))
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.CommandContext(ctx, "xdg-open", searchURL)
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd", "/c", "start", searchURL)
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", searchURL)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}
	return nil
}
