// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package internal

import (
	"os"
	"path/filepath"
	"qqmgr/internal/config"
	"strings"
	"testing"
)

func TestSSHConfigGeneration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	testConfigContent := `[qemu]
bin = "qemu-system-x86_64"

[ssh]
ServerAliveInterval = 300
ServerAliveCountMax = 3
UserKnownHostsFile = "/dev/null"
StrictHostKeyChecking = "no"

[vm.test-vm]
cmd = ["-nodefaults"]

[vm.test-vm.ssh]
port = 2089
vm_port = 22
ServerAliveInterval = 60
Compression = "yes"`

	testFile := filepath.Join(tempDir, "test.toml")
	err := os.WriteFile(testFile, []byte(testConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config, err := config.LoadFromFile(testFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test SSH config generation
	sshConfigPath, err := GenerateSSHConfig(config, "test-vm", testFile)
	if err != nil {
		t.Fatalf("Failed to generate SSH config: %v", err)
	}
	defer os.Remove(sshConfigPath)

	// Read the generated config
	configData, err := os.ReadFile(sshConfigPath)
	if err != nil {
		t.Fatalf("Failed to read generated SSH config: %v", err)
	}

	configContent := string(configData)

	// Check that global options are present
	if !strings.Contains(configContent, "ServerAliveInterval 300") {
		t.Error("Expected global ServerAliveInterval to be present")
	}
	if !strings.Contains(configContent, "ServerAliveCountMax 3") {
		t.Error("Expected global ServerAliveCountMax to be present")
	}
	if !strings.Contains(configContent, "UserKnownHostsFile /dev/null") {
		t.Error("Expected global UserKnownHostsFile to be present")
	}
	if !strings.Contains(configContent, "StrictHostKeyChecking no") {
		t.Error("Expected global StrictHostKeyChecking to be present")
	}

	// Check that VM-specific options override global ones
	if !strings.Contains(configContent, "ServerAliveInterval 60") {
		t.Error("Expected VM-specific ServerAliveInterval to override global")
	}
	if !strings.Contains(configContent, "Compression yes") {
		t.Error("Expected VM-specific Compression option to be present")
	}

	// Check that port and vm_port are not included
	if strings.Contains(configContent, "port") {
		t.Error("Expected port to be excluded from SSH config")
	}
	if strings.Contains(configContent, "vm_port") {
		t.Error("Expected vm_port to be excluded from SSH config")
	}
}

func TestGetSSHOptions(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	testConfigContent := `[qemu]
bin = "qemu-system-x86_64"

[ssh]
ServerAliveInterval = 300
ServerAliveCountMax = 3

[vm.test-vm]
cmd = ["-nodefaults"]

[vm.test-vm.ssh]
port = 2089
vm_port = 22
ServerAliveInterval = 60
Compression = "yes"`

	testFile := filepath.Join(tempDir, "test.toml")
	err := os.WriteFile(testFile, []byte(testConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config, err := config.LoadFromFile(testFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	options, err := GetSSHOptions(config, "test-vm")
	if err != nil {
		t.Fatalf("Failed to get SSH options: %v", err)
	}

	// Check that global options are present
	if options["ServerAliveCountMax"] != int64(3) {
		t.Errorf("Expected ServerAliveCountMax to be 3, got %v", options["ServerAliveCountMax"])
	}

	// Check that VM-specific options override global ones
	if options["ServerAliveInterval"] != int64(60) {
		t.Errorf("Expected ServerAliveInterval to be overridden to 60, got %v", options["ServerAliveInterval"])
	}
	if options["Compression"] != "yes" {
		t.Errorf("Expected Compression to be 'yes', got %v", options["Compression"])
	}

	// Check that port and vm_port are not included
	if _, exists := options["port"]; exists {
		t.Error("Expected port to be excluded from SSH options")
	}
	if _, exists := options["vm_port"]; exists {
		t.Error("Expected vm_port to be excluded from SSH options")
	}
}
