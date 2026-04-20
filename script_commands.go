package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var scriptCommandName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func runScriptCommand(ctx context.Context, text string) (string, bool, error) {
	if !strings.HasPrefix(text, "/") {
		return "", false, nil
	}

	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(text, "/")))
	if len(fields) == 0 {
		return "", false, nil
	}

	commandName := fields[0]
	if !scriptCommandName.MatchString(commandName) {
		return "", false, nil
	}

	scriptPath := filepath.Join("scripts.d", commandName+".sh")
	info, err := os.Stat(scriptPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", true, fmt.Errorf("script /%s is unavailable: %w", commandName, err)
	}

	if info.IsDir() {
		return "", true, fmt.Errorf("script /%s points to a directory", commandName)
	}

	bashPath, err := exec.LookPath("bash")
	if err != nil {
		return "", true, fmt.Errorf("bash is not available for /%s: %w", commandName, err)
	}

	args := append([]string{scriptPath}, fields[1:]...)
	cmd := exec.CommandContext(ctx, bashPath, args...)
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if result == "" && err == nil {
		result = fmt.Sprintf("/%s completed with no output", commandName)
	}

	if err != nil {
		if result == "" {
			return "", true, fmt.Errorf("/%s failed: %w", commandName, err)
		}
		return "", true, fmt.Errorf("%s\n/%s failed: %v", result, commandName, err)
	}

	return result, true, nil
}
