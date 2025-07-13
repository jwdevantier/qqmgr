// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"qqmgr/internal/config"
)

func TestShowLastLines(t *testing.T) {
	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "serial-test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write some test lines
	testLines := []string{
		"Line 1",
		"Line 2",
		"Line 3",
		"Line 4",
		"Line 5",
		"Line 6",
		"Line 7",
		"Line 8",
		"Line 9",
		"Line 10",
		"Line 11",
		"Line 12",
	}

	for _, line := range testLines {
		if _, err := tempFile.WriteString(line + "\n"); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
	}
	tempFile.Close()

	// Test showing last 5 lines
	err = showLastLines(tempFile.Name(), 5)
	if err != nil {
		t.Fatalf("showLastLines() failed: %v", err)
	}
}

func TestShowLastLinesWithFewerLines(t *testing.T) {
	// Create a temporary file with fewer lines than requested
	tempFile, err := os.CreateTemp("", "serial-test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write only 3 lines
	testLines := []string{
		"Line 1",
		"Line 2",
		"Line 3",
	}

	for _, line := range testLines {
		if _, err := tempFile.WriteString(line + "\n"); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
	}
	tempFile.Close()

	// Test showing last 10 lines (should show all 3)
	err = showLastLines(tempFile.Name(), 10)
	if err != nil {
		t.Fatalf("showLastLines() failed: %v", err)
	}
}

func TestShowLastLinesWithEmptyFile(t *testing.T) {
	// Create an empty temporary file
	tempFile, err := os.CreateTemp("", "serial-test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Test showing last 5 lines from empty file
	err = showLastLines(tempFile.Name(), 5)
	if err != nil {
		t.Fatalf("showLastLines() failed: %v", err)
	}
}

func TestShowLastLinesWithNonexistentFile(t *testing.T) {
	// Test with a file that doesn't exist
	err := showLastLines("/nonexistent/file", 5)
	if err == nil {
		t.Error("showLastLines() should fail with nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to open serial file") {
		t.Errorf("Expected error about opening file, got: %v", err)
	}
}

func TestFollowSerialOutput(t *testing.T) {
	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "serial-test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write initial content
	if _, err := tempFile.WriteString("Initial line\n"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// Start following in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- followSerialOutput(tempFile.Name())
	}()

	// Wait a bit for the follow to start
	time.Sleep(100 * time.Millisecond)

	// Add more content to the file
	file, err := os.OpenFile(tempFile.Name(), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file for appending: %v", err)
	}

	if _, err := file.WriteString("New line 1\n"); err != nil {
		t.Fatalf("Failed to write new line: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if _, err := file.WriteString("New line 2\n"); err != nil {
		t.Fatalf("Failed to write new line: %v", err)
	}
	file.Close()

	// Wait a bit more for the follow to process
	time.Sleep(100 * time.Millisecond)

	// The follow should still be running, so we'll just check that it didn't error immediately
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("followSerialOutput() failed unexpectedly: %v", err)
		}
	default:
		// This is expected - the follow should still be running
	}
}

func TestDisplaySerialOutput(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "qqmgr-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test VM entry
	vmEntry := &config.VmEntry{
		Name: "test-vm",
		Cmd:  []string{"-nodefaults", "-machine", "none"},
		Vars: map[string]interface{}{
			"ssh_host": 2089,
			"ssh_vm":   22,
		},
		DataDir: filepath.Join(tempDir, "vm.test-vm"),
	}

	// Create the serial file
	serialFile := vmEntry.SerialFilePath()
	if err := os.MkdirAll(filepath.Dir(serialFile), 0755); err != nil {
		t.Fatalf("Failed to create serial file directory: %v", err)
	}

	// Write some test content
	if err := os.WriteFile(serialFile, []byte("Test line 1\nTest line 2\n"), 0644); err != nil {
		t.Fatalf("Failed to write serial file: %v", err)
	}

	// Test displaying last lines
	err = displaySerialOutput(vmEntry, false, 5)
	if err != nil {
		t.Fatalf("displaySerialOutput() failed: %v", err)
	}
}

func TestDisplaySerialOutputWithNonexistentFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "qqmgr-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test VM entry
	vmEntry := &config.VmEntry{
		Name: "test-vm",
		Cmd:  []string{"-nodefaults", "-machine", "none"},
		Vars: map[string]interface{}{
			"ssh_host": 2089,
			"ssh_vm":   22,
		},
		DataDir: filepath.Join(tempDir, "vm.test-vm"),
	}

	// Test with nonexistent serial file
	err = displaySerialOutput(vmEntry, false, 5)
	if err == nil {
		t.Error("displaySerialOutput() should fail with nonexistent serial file")
	}
	if !strings.Contains(err.Error(), "serial file not found") {
		t.Errorf("Expected error about serial file not found, got: %v", err)
	}
}

func TestSerialCommandIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "qqmgr-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test configuration file
	configContent := fmt.Sprintf(`
[qemu]
bin = "qemu-system-x86_64"
img = "qemu-img"

[vars]
home = "%s"
data_dir = "%s"

[vm.test-vm]
cmd = [
    "-nodefaults -machine q35,accel=kvm,kernel-irqchip=split",
    "-cpu host -smp 2 -m 4096",
]

[vm.test-vm.vars]
ssh_host = 2089
ssh_vm = 22

[vm.test-vm.ssh]
port = 2089
vm_port = 22
`, tempDir, tempDir)

	configFile := filepath.Join(tempDir, "qqmgr.toml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create a mock VM with serial file
	vmEntry := &config.VmEntry{
		Name: "test-vm",
		Cmd:  []string{"-nodefaults", "-machine", "none"},
		Vars: map[string]interface{}{
			"ssh_host": 2089,
			"ssh_vm":   22,
		},
		DataDir: filepath.Join(tempDir, "vm.test-vm"),
	}

	// Create the serial file
	serialFile := vmEntry.SerialFilePath()
	if err := os.MkdirAll(filepath.Dir(serialFile), 0755); err != nil {
		t.Fatalf("Failed to create serial file directory: %v", err)
	}

	// Write some test content
	testContent := "Boot sequence started\nKernel loaded\nSystem ready\n"
	if err := os.WriteFile(serialFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write serial file: %v", err)
	}

	// Create a mock PID file to simulate running VM
	pidFile := vmEntry.PidFilePath()
	if err := os.WriteFile(pidFile, []byte("12345"), 0644); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	// Test the serial command functionality
	// We'll test the displaySerialOutput function directly since it's the core functionality
	err = displaySerialOutput(vmEntry, false, 2)
	if err != nil {
		t.Fatalf("displaySerialOutput() failed: %v", err)
	}
}
