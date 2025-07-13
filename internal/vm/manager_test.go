// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package vm

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"qqmgr/internal/config"
)

// TestManagerReadPIDFile tests PID file reading with validation
func TestManagerReadPIDFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vm-manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		DataDir: tmpDir,
	}

	manager := NewManager(vmEntry)

	// Test 1: No PID file
	pid, err := manager.readPIDFile()
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}
	if pid != nil {
		t.Error("Expected nil PID when no PID file exists")
	}

	// Test 2: Empty PID file
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write empty PID file: %v", err)
	}

	pid, err = manager.readPIDFile()
	if err != nil {
		t.Fatalf("Failed to read empty PID file: %v", err)
	}
	if pid != nil {
		t.Error("Expected nil PID for empty file")
	}

	// Test 3: Valid PID file
	validPID := 12345
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte(strconv.Itoa(validPID)), 0644); err != nil {
		t.Fatalf("Failed to write valid PID file: %v", err)
	}

	pid, err = manager.readPIDFile()
	if err != nil {
		t.Fatalf("Failed to read valid PID file: %v", err)
	}
	if pid == nil {
		t.Error("Expected PID when valid PID file exists")
	} else if *pid != validPID {
		t.Errorf("Expected PID %d, got %d", validPID, *pid)
	}

	// Test 4: PID file with whitespace
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte("  12345  "), 0644); err != nil {
		t.Fatalf("Failed to write PID file with whitespace: %v", err)
	}

	pid, err = manager.readPIDFile()
	if err != nil {
		t.Fatalf("Failed to read PID file with whitespace: %v", err)
	}
	if pid == nil {
		t.Error("Expected PID when PID file with whitespace exists")
	} else if *pid != validPID {
		t.Errorf("Expected PID %d, got %d", validPID, *pid)
	}

	// Test 5: Invalid PID file (non-numeric)
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte("not-a-number"), 0644); err != nil {
		t.Fatalf("Failed to write invalid PID file: %v", err)
	}

	pid, err = manager.readPIDFile()
	if err == nil {
		t.Error("Expected error for invalid PID")
	}
	if pid != nil {
		t.Error("Expected nil PID for invalid file")
	}

	// Test 6: PID out of range (negative)
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte("-1"), 0644); err != nil {
		t.Fatalf("Failed to write negative PID file: %v", err)
	}

	pid, err = manager.readPIDFile()
	if err == nil {
		t.Error("Expected error for negative PID")
	}
	if pid != nil {
		t.Error("Expected nil PID for negative PID")
	}

	// Test 7: PID out of range (too large)
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte("9999999"), 0644); err != nil {
		t.Fatalf("Failed to write large PID file: %v", err)
	}

	pid, err = manager.readPIDFile()
	if err == nil {
		t.Error("Expected error for too large PID")
	}
	if pid != nil {
		t.Error("Expected nil PID for too large PID")
	}
}

// TestManagerIsProcessRunning tests process running detection
func TestManagerIsProcessRunning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vm-manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		DataDir: tmpDir,
	}

	manager := NewManager(vmEntry)

	// Test with nil PID
	if manager.isProcessRunning(nil) {
		t.Error("Expected false for nil PID")
	}

	// Test with non-existent PID
	nonExistentPID := 999999
	if manager.isProcessRunning(&nonExistentPID) {
		t.Error("Expected false for non-existent PID")
	}

	// Test with current process PID
	currentPID := os.Getpid()
	if !manager.isProcessRunning(&currentPID) {
		t.Error("Expected true for current process PID")
	}
}

// TestManagerGetStatus tests status retrieval
func TestManagerGetStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vm-manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		DataDir: tmpDir,
		Vars:    map[string]interface{}{"ssh_host": 2222},
	}

	manager := NewManager(vmEntry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test status when VM is not running
	status, err := manager.GetStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if status.Name != "test-vm" {
		t.Errorf("Expected name 'test-vm', got '%s'", status.Name)
	}

	if status.IsRunning {
		t.Error("Expected VM to not be running")
	}

	if status.IsAlive {
		t.Error("Expected VM to not be alive")
	}

	if status.QMPConnected {
		t.Error("Expected QMP to not be connected")
	}

	if status.PID != nil {
		t.Error("Expected nil PID when VM is not running")
	}

	// Verify file paths
	expectedPIDFile := vmEntry.PidFilePath()
	if status.PIDFile != expectedPIDFile {
		t.Errorf("Expected PID file '%s', got '%s'", expectedPIDFile, status.PIDFile)
	}

	expectedQMPSocket := vmEntry.QmpSocketPath()
	if status.QMPSocket != expectedQMPSocket {
		t.Errorf("Expected QMP socket '%s', got '%s'", expectedQMPSocket, status.QMPSocket)
	}
}

// TestManagerIsAlive tests QMP-based alive checking
func TestManagerIsAlive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vm-manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		DataDir: tmpDir,
	}

	manager := NewManager(vmEntry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test when QMP socket doesn't exist
	alive, err := manager.IsAlive(ctx)
	if err == nil {
		t.Error("Expected error when QMP socket doesn't exist")
	}
	if alive {
		t.Error("Expected VM to not be alive when QMP socket doesn't exist")
	}
}

// TestManagerStop tests VM stopping functionality
func TestManagerStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vm-manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		DataDir: tmpDir,
	}

	manager := NewManager(vmEntry)

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
}

// TestManagerCleanupRuntimeFiles tests runtime file cleanup
func TestManagerCleanupRuntimeFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vm-manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		DataDir: tmpDir,
	}

	manager := NewManager(vmEntry)

	// Create some mock files
	files := []string{
		vmEntry.PidFilePath(),
		vmEntry.SerialFilePath(),
		vmEntry.QmpSocketPath(),
		vmEntry.MonitorSocketPath(),
		vmEntry.SshConfigPath(),
	}

	for _, file := range files {
		// Create directory if needed
		dir := filepath.Dir(file)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		// Create file
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}

	// Test cleanup
	if err := manager.cleanupRuntimeFiles(); err != nil {
		t.Fatalf("Failed to cleanup runtime files: %v", err)
	}

	// Verify files were removed
	for _, file := range files {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("File %s should have been removed", file)
		}
	}

	// Test cleanup of non-existent files (should not error)
	if err := manager.cleanupRuntimeFiles(); err != nil {
		t.Fatalf("Cleanup of non-existent files should not error: %v", err)
	}
}

// TestManagerForceKillPID tests force kill functionality
func TestManagerForceKillPID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vm-manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		DataDir: tmpDir,
	}

	manager := NewManager(vmEntry)

	// Test with non-existent PID (should not error)
	err = manager.forceKillPID(999999)
	if err != nil {
		// On some systems, this might error, which is acceptable
		t.Logf("Force kill of non-existent PID returned error (expected on some systems): %v", err)
	}
}

// BenchmarkManagerReadPIDFile benchmarks PID file reading
func BenchmarkManagerReadPIDFile(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "vm-manager-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vmEntry := &config.VmEntry{
		Name:    "test-vm",
		DataDir: tmpDir,
	}

	manager := NewManager(vmEntry)

	// Create PID file
	if err := os.WriteFile(vmEntry.PidFilePath(), []byte("12345"), 0644); err != nil {
		b.Fatalf("Failed to write PID file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.readPIDFile()
		if err != nil {
			b.Fatalf("Failed to read PID file: %v", err)
		}
	}
}
