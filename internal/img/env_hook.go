// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package img

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnvHookExecutor executes environment hooks
type EnvHookExecutor struct{}

// NewEnvHookExecutor creates a new environment hook executor
func NewEnvHookExecutor() *EnvHookExecutor {
	return &EnvHookExecutor{}
}

// Execute runs an environment hook and returns the processed environment
func (e *EnvHookExecutor) Execute(
	hook *EnvHookConfig,
	configDir string,
	env map[string]interface{},
) (map[string]interface{}, error) {
	// Prepare the script path
	scriptPath := filepath.Join(configDir, hook.Script)

	// Create command
	var cmd *exec.Cmd
	if hook.Interpreter != "" {
		cmd = exec.Command(hook.Interpreter, scriptPath)
	} else {
		cmd = exec.Command(scriptPath)
	}

	// Set up stdin with JSON input
	inputData, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal environment: %w", err)
	}

	cmd.Stdin = bytes.NewReader(inputData)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("hook execution failed: %s, %w", stderr.String(), err)
	}

	// Parse the last line of stdout as JSON
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("hook produced no output")
	}

	lastLine := strings.TrimSpace(lines[len(lines)-1])
	if lastLine == "" {
		return nil, fmt.Errorf("hook produced empty last line")
	}

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(lastLine), &result); err != nil {
		return nil, fmt.Errorf("failed to parse hook output as JSON: %w", err)
	}

	return result, nil
}
