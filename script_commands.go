package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var scriptCommandName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type scriptRunner struct {
	path string
	bin  string
	args []string
}

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

	runner, err := resolveScriptRunner(commandName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", true, err
	}

	args := append(runner.args, fields[1:]...)
	cmd := exec.CommandContext(ctx, runner.bin, args...)
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

func resolveScriptRunner(commandName string) (scriptRunner, error) {
	for _, candidate := range scriptCandidates(commandName) {
		info, err := os.Stat(candidate.path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return scriptRunner{}, fmt.Errorf("script /%s is unavailable: %w", commandName, err)
		}

		if info.IsDir() {
			return scriptRunner{}, fmt.Errorf("script /%s points to a directory", commandName)
		}

		bin, err := exec.LookPath(candidate.bin)
		if err != nil {
			return scriptRunner{}, fmt.Errorf("%s is not available for /%s: %w", candidate.bin, commandName, err)
		}

		return scriptRunner{
			path: candidate.path,
			bin:  bin,
			args: append(candidate.args, candidate.path),
		}, nil
	}

	return scriptRunner{}, os.ErrNotExist
}

func scriptCandidates(commandName string) []scriptRunner {
	baseDir := "scripts.d"
	ps1 := scriptRunner{
		path: filepath.Join(baseDir, commandName+".ps1"),
		bin:  "powershell",
		args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File"},
	}
	sh := scriptRunner{
		path: filepath.Join(baseDir, commandName+".sh"),
		bin:  "bash",
		args: nil,
	}

	if runtime.GOOS == "windows" {
		return []scriptRunner{ps1, sh}
	}

	return []scriptRunner{sh, ps1}
}
