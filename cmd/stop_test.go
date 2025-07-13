// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"qqmgr/internal/config"
	"qqmgr/internal/vm"
)

// TestStopCommandLogic tests the core logic of the stop command
func TestStopCommandLogic(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "qqmgr-stop-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock VM entry
	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		Cmd:     []string{"qemu-system-x86_64", "-hda", "test.img"},
		Vars:    map[string]interface{}{"ssh_host": 2222},
		DataDir: tmpDir,
	}

	// Create VM manager
	manager := vm.NewManager(vmEntry)

	// Test 1: VM not running (no PID file)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := manager.GetStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if status.IsRunning {
		t.Error("Expected VM to not be running when no PID file exists")
	}

	// Test 2: Create PID file and check status
	pidValue := 12345
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte(strconv.Itoa(pidValue)), 0644); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	status, err = manager.GetStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if status.PID == nil {
		t.Error("Expected PID when PID file exists")
	} else if *status.PID != pidValue {
		t.Errorf("Expected PID %d, got %d", pidValue, *status.PID)
	}

	// Test 3: Test stop functionality when VM is not running
	success, err := manager.Stop(ctx, 10*time.Second, true)
	if err != nil {
		t.Fatalf("Failed to stop VM: %v", err)
	}

	if !success {
		t.Error("Expected stop to succeed when VM is not running")
	}
}

// TestStopCommandFlags tests the command flags
func TestStopCommandFlags(t *testing.T) {
	// Test default values
	if forceFlag != true {
		t.Errorf("Expected forceFlag to be true by default, got %v", forceFlag)
	}
	if timeoutFlag != 20 {
		t.Errorf("Expected timeoutFlag to be 20 by default, got %d", timeoutFlag)
	}
}

// TestStopCommandIntegration tests the full stop command integration
func TestStopCommandIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "qqmgr-stop-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock VM entry
	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		Cmd:     []string{"qemu-system-x86_64", "-hda", "test.img"},
		Vars:    map[string]interface{}{"ssh_host": 2222},
		DataDir: tmpDir,
	}

	// Create VM manager
	manager := vm.NewManager(vmEntry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test stopping when VM is not running
	success, err := manager.Stop(ctx, 10*time.Second, true)
	if err != nil {
		t.Fatalf("Failed to stop VM: %v", err)
	}

	if !success {
		t.Error("Expected stop to succeed when VM is not running")
	}

	// Verify that runtime files are cleaned up
	pidFile := vmEntry.PidFilePath()
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file should have been cleaned up")
	}
}

// TestStopCommandWithPIDFile tests stop command with PID file present
func TestStopCommandWithPIDFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "qqmgr-stop-pid-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock VM entry
	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		Cmd:     []string{"qemu-system-x86_64", "-hda", "test.img"},
		Vars:    map[string]interface{}{"ssh_host": 2222},
		DataDir: tmpDir,
	}

	// Create PID file with current process PID (which exists)
	currentPID := os.Getpid()
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte(strconv.Itoa(currentPID)), 0644); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	// Create VM manager
	manager := vm.NewManager(vmEntry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test status with PID file
	status, err := manager.GetStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if status.PID == nil {
		t.Error("Expected PID when PID file exists")
	} else if *status.PID != currentPID {
		t.Errorf("Expected PID %d, got %d", currentPID, *status.PID)
	}

	// Note: We can't actually test the full stop functionality here because
	// we can't kill the current process, but we can test that the status
	// detection works correctly
}

// TestStopCommandEdgeCases tests edge cases for the stop command
func TestStopCommandEdgeCases(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "qqmgr-stop-edge-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock VM entry
	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		DataDir: tmpDir,
	}

	// Create VM manager
	manager := vm.NewManager(vmEntry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with empty PID file
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write empty PID file: %v", err)
	}

	status, err := manager.GetStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if status.PID != nil {
		t.Error("Expected nil PID for empty file")
	}

	// Test with invalid PID file
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte("not-a-number"), 0644); err != nil {
		t.Fatalf("Failed to write invalid PID file: %v", err)
	}

	status, err = manager.GetStatus(ctx)
	if err == nil {
		t.Error("Expected error for invalid PID file")
	}
}

// BenchmarkStopCommandStatus benchmarks status checking
func BenchmarkStopCommandStatus(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "qqmgr-stop-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		DataDir: tmpDir,
	}

	manager := vm.NewManager(vmEntry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.GetStatus(ctx)
		if err != nil {
			b.Fatalf("Failed to get status: %v", err)
		}
	}
}
