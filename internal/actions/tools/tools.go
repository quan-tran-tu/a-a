package tools

import (
	"fmt"
	"os/exec"
)

func HandleGit(subcommand string, args []string, path string) error {
	// Provide a security whitelist for git subcommand
	switch subcommand {
	case "clone", "commit", "push", "pull", "init", "status", "add":
	default:
		return fmt.Errorf("git subcommand '%s' is not allowed", subcommand)
	}

	cmdArgs := append([]string{subcommand}, args...)
	cmd := exec.Command("git", cmdArgs...)

	// If a specific path is provided, run the command there.
	if path != "" {
		cmd.Dir = path
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git command failed: %s\n%s", err, string(output))
	}

	fmt.Printf("Git command successful:\n%s\n", string(output))
	return nil
}
