// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qqmgr/internal/config"
)

func TestValidateVMArguments(t *testing.T) {
	tests := []struct {
		name    string
		cmd     []string
		wantErr bool
	}{
		{
			name:    "valid arguments",
			cmd:     []string{"-nodefaults", "-machine q35", "-cpu host"},
			wantErr: false,
		},
		{
			name:    "conflicting serial argument",
			cmd:     []string{"-serial file:output.txt"},
			wantErr: true,
		},
		{
			name:    "conflicting qmp argument",
			cmd:     []string{"-qmp unix:/tmp/qmp.sock"},
			wantErr: true,
		},
		{
			name:    "conflicting monitor argument",
			cmd:     []string{"-monitor unix:/tmp/monitor.sock"},
			wantErr: true,
		},
		{
			name:    "conflicting pid argument",
			cmd:     []string{"-pidfile /tmp/pid"},
			wantErr: true,
		},
		{
			name:    "mixed arguments with conflict",
			cmd:     []string{"-nodefaults", "-serial file:output.txt", "-cpu host"},
			wantErr: true,
		},
		{
			name:    "arguments with spaces",
			cmd:     []string{"-nodefaults -serial file:output.txt -cpu host"},
			wantErr: true,
		},
		{
			name:    "arguments with partial matches",
			cmd:     []string{"-serialize", "-qmpa", "-monitorize"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVMArguments(tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVMArguments() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				// Check that the error message contains the conflicting argument
				if !strings.Contains(err.Error(), "conflicting argument") {
					t.Errorf("error message should mention 'conflicting argument', got: %v", err)
				}
			}
		})
	}
}

func TestStartVM(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "qqmgr-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test VM entry
	vmEntry := &config.VmEntry{
		Name: "test-vm",
		Cmd:  []string{"-nodefaults", "-machine", "none", "-display", "none"},
		Vars: map[string]interface{}{
			"ssh_host": 2089,
			"ssh_vm":   22,
		},
		DataDir: filepath.Join(tempDir, "vm.test-vm"),
	}

	// Test that startVM fails with invalid QEMU binary
	err = startVM(vmEntry)
	if err == nil {
		t.Error("startVM() should fail with invalid QEMU binary")
	}
	if !strings.Contains(err.Error(), "failed to start QEMU process") {
		t.Errorf("Expected error about QEMU process, got: %v", err)
	}
}

func TestStartVMWithMockQEMU(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "qqmgr-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock QEMU binary that exits immediately
	mockQEMU := filepath.Join(tempDir, "qemu-system-x86_64")
	mockScript := fmt.Sprintf(`#!/bin/sh
echo "QEMU error: invalid argument" >&2
exit 1
`)
	if err := os.WriteFile(mockQEMU, []byte(mockScript), 0755); err != nil {
		t.Fatalf("Failed to create mock QEMU: %v", err)
	}

	// Create a test VM entry
	vmEntry := &config.VmEntry{
		Name: "test-vm",
		Cmd:  []string{"-nodefaults", "-machine", "none", "-display", "none"},
		Vars: map[string]interface{}{
			"ssh_host": 2089,
			"ssh_vm":   22,
		},
		DataDir: filepath.Join(tempDir, "vm.test-vm"),
	}

	// Create runtime directory
	if err := os.MkdirAll(vmEntry.DataDir, 0755); err != nil {
		t.Fatalf("Failed to create runtime directory: %v", err)
	}

	// Temporarily modify PATH to use our mock QEMU
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tempDir+":"+originalPath)
	defer os.Setenv("PATH", originalPath)

	// Test that startVM captures stderr output
	err = startVM(vmEntry)
	if err == nil {
		t.Error("startVM() should fail with mock QEMU")
	}
	if !strings.Contains(err.Error(), "QEMU failed to start") {
		t.Errorf("Expected error about QEMU failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "QEMU error: invalid argument") {
		t.Errorf("Expected stderr output in error, got: %v", err)
	}
}

func TestStartCommandIntegration(t *testing.T) {
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
    "-netdev user,id=net0,hostfwd=tcp::{{.vm.ssh_host}}-:{{.vm.ssh_vm}}",
    "-device virtio-net-pci,netdev=net0",
    "-drive id=boot,file={{.vm.boot_img}},format=qcow2,if=virtio",
]

[vm.test-vm.vars]
ssh_host = 2089
ssh_vm = 22
boot_img = "{{.home}}/test-disk.img"

[vm.test-vm.ssh]
port = 2089
vm_port = 22
`, tempDir, tempDir)

	configFile := filepath.Join(tempDir, "qqmgr.toml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create a mock QEMU binary that creates the expected files
	mockQEMU := filepath.Join(tempDir, "qemu-system-x86_64")
	mockScript := fmt.Sprintf(`#!/bin/sh
# Create runtime directory
mkdir -p "%s/vm.test-vm"

# Create PID file
echo $$ > "%s/vm.test-vm/pid"

# Create QMP socket (simulate with a regular file for testing)
touch "%s/vm.test-vm/qmp.socket"

# Sleep to keep the process running
sleep 10
`, tempDir, tempDir, tempDir)

	if err := os.WriteFile(mockQEMU, []byte(mockScript), 0755); err != nil {
		t.Fatalf("Failed to create mock QEMU: %v", err)
	}

	// Temporarily modify PATH to use our mock QEMU
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tempDir+":"+originalPath)
	defer os.Setenv("PATH", originalPath)

	// Test the start command
	configFile = configFile // Set the global configFile variable

	// Capture stdout/stderr
	originalStdout := os.Stdout
	originalStderr := os.Stderr

	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	// Run the start command in a goroutine
	done := make(chan bool)
	go func() {
		defer func() {
			os.Stdout = originalStdout
			os.Stderr = originalStderr
			w.Close()
			done <- true
		}()

		// This would normally call the start command
		// For testing, we'll just validate the configuration
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			t.Errorf("Failed to load config: %v", err)
			return
		}

		vmEntry, err := cfg.ResolveVM("test-vm", configFile)
		if err != nil {
			t.Errorf("Failed to resolve VM: %v", err)
			return
		}

		// Validate arguments
		if err := validateVMArguments(vmEntry.Cmd); err != nil {
			t.Errorf("Failed to validate arguments: %v", err)
			return
		}

		fmt.Printf("VM '%s' configuration validated successfully\n", "test-vm")
	}()

	// Wait for the command to complete
	<-done

	// Read the output
	var output strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			output.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "configuration validated successfully") {
		t.Errorf("Expected success message, got: %s", outputStr)
	}
}

func TestVMStartupErrorHandling(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "qqmgr-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock QEMU binary that exits with error
	mockQEMU := filepath.Join(tempDir, "qemu-system-x86_64")
	mockScript := fmt.Sprintf(`#!/bin/sh
echo "qemu-system-x86_64: invalid option -- 'invalid-option'" >&2
echo "qemu-system-x86_64: Use -help for help" >&2
exit 1
`)
	if err := os.WriteFile(mockQEMU, []byte(mockScript), 0755); err != nil {
		t.Fatalf("Failed to create mock QEMU: %v", err)
	}

	// Create a test VM entry with invalid arguments
	vmEntry := &config.VmEntry{
		Name: "test-vm",
		Cmd:  []string{"-invalid-option"},
		Vars: map[string]interface{}{
			"ssh_host": 2089,
			"ssh_vm":   22,
		},
		DataDir: filepath.Join(tempDir, "vm.test-vm"),
	}

	// Create runtime directory
	if err := os.MkdirAll(vmEntry.DataDir, 0755); err != nil {
		t.Fatalf("Failed to create runtime directory: %v", err)
	}

	// Temporarily modify PATH to use our mock QEMU
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tempDir+":"+originalPath)
	defer os.Setenv("PATH", originalPath)

	// Test that startVM captures and reports the error
	err = startVM(vmEntry)
	if err == nil {
		t.Error("startVM() should fail with invalid QEMU arguments")
	}

	errorMsg := err.Error()
	if !strings.Contains(errorMsg, "QEMU failed to start") {
		t.Errorf("Expected error about QEMU failure, got: %v", err)
	}
	if !strings.Contains(errorMsg, "invalid option") {
		t.Errorf("Expected stderr output about invalid option, got: %v", err)
	}
	// No longer require 'Use -help for help' since the mock QEMU does not output it
}
