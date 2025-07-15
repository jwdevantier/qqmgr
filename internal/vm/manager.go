// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package vm

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"qqmgr/internal"
	"qqmgr/internal/config"
	"syscall"
)

// Manager provides VM management functionality
type Manager struct {
	vmEntry *config.VmEntry
}

// NewManager creates a new VM manager for the given VM entry
func NewManager(vmEntry *config.VmEntry) *Manager {
	return &Manager{
		vmEntry: vmEntry,
	}
}

// Status represents the current status of a VM
type Status struct {
	Name          string                 `json:"name"`
	PID           *int                   `json:"pid,omitempty"`
	PIDFile       string                 `json:"pid_file"`
	IsRunning     bool                   `json:"running"`
	IsAlive       bool                   `json:"alive"`
	SSHPort       interface{}            `json:"ssh_port"`
	SSHConfig     string                 `json:"ssh_config"`
	SerialFile    string                 `json:"serial_file"`
	QMPSocket     string                 `json:"qmp_socket"`
	MonitorSocket string                 `json:"monitor_socket"`
	QMPConnected  bool                   `json:"qmp_connected"`
	StatusDetails map[string]interface{} `json:"status_details,omitempty"`
}

// GetStatus returns the current status of the VM
func (m *Manager) GetStatus(ctx context.Context) (*Status, error) {
	status := &Status{
		Name:          m.vmEntry.Name,
		PIDFile:       m.vmEntry.PidFilePath(),
		SSHPort:       m.getSSHPort(),
		SSHConfig:     m.vmEntry.SshConfigPath(),
		SerialFile:    m.vmEntry.SerialFilePath(),
		QMPSocket:     m.vmEntry.QmpSocketPath(),
		MonitorSocket: m.vmEntry.MonitorSocketPath(),
	}

	// Read PID file
	pid, err := m.readPIDFile()
	if err != nil {
		return nil, fmt.Errorf("failed to read PID file: %w", err)
	}
	status.PID = pid

	// Check if VM is alive via QMP
	alive, connected, statusDetails, err := m.checkQMPStatus(ctx)
	if err != nil {
		// QMP check failed, but we can still report PID-based status
		status.IsAlive = false
		status.QMPConnected = false
		status.IsRunning = pid != nil && m.isProcessRunning(pid)
	} else {
		status.IsAlive = alive
		status.QMPConnected = connected
		status.IsRunning = alive // QMP is the authoritative source
		status.StatusDetails = statusDetails
	}

	return status, nil
}

// IsAlive checks if the VM is alive using QMP
func (m *Manager) IsAlive(ctx context.Context) (bool, error) {
	alive, _, _, err := m.checkQMPStatus(ctx)
	return alive, err
}

// Stop gracefully shuts down the VM
func (m *Manager) Stop(ctx context.Context, timeout time.Duration, forceAfterTimeout bool) (bool, error) {
	// First check if VM is running
	status, err := m.GetStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get VM status: %w", err)
	}

	if !status.IsRunning {
		// VM is not running, clean up any stale files
		if err := m.cleanupRuntimeFiles(); err != nil {
			return false, fmt.Errorf("failed to cleanup runtime files: %w", err)
		}
		return true, nil
	}

	// Create QMP client
	qmpClient := internal.NewQMPClient(m.vmEntry.QmpSocketPath())

	// Try to connect to QMP
	if err := qmpClient.Connect(ctx); err != nil {
		// QMP connection failed, fall back to force kill
		if status.PID != nil {
			if err := m.forceKillPID(*status.PID); err != nil {
				return false, fmt.Errorf("failed to force kill PID %d: %w", *status.PID, err)
			}
		}
	} else {
		defer qmpClient.Close()

		// Attempt graceful shutdown via QMP
		success, err := qmpClient.Shutdown(ctx, 1*time.Second, timeout, forceAfterTimeout)
		if err != nil {
			// QMP shutdown failed, fall back to force kill
			if status.PID != nil {
				if err := m.forceKillPID(*status.PID); err != nil {
					return false, fmt.Errorf("failed to force kill PID %d: %w", *status.PID, err)
				}
			}
		} else if !success && forceAfterTimeout {
			// Graceful shutdown timed out, force kill
			if status.PID != nil {
				if err := m.forceKillPID(*status.PID); err != nil {
					return false, fmt.Errorf("failed to force kill PID %d: %w", *status.PID, err)
				}
			}
		}
	}

	// Clean up runtime files
	if err := m.cleanupRuntimeFiles(); err != nil {
		return false, fmt.Errorf("failed to cleanup runtime files: %w", err)
	}

	return true, nil
}

// readPIDFile reads and validates the PID from the PID file
func (m *Manager) readPIDFile() (*int, error) {
	data, err := os.ReadFile(m.vmEntry.PidFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No PID file means VM is not running
		}
		return nil, fmt.Errorf("failed to read PID file: %w", err)
	}

	pidStr := string(data)
	if pidStr == "" {
		return nil, nil // Empty PID file
	}

	// Trim whitespace
	pidStr = strings.TrimSpace(pidStr)

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid PID in file: %w", err)
	}

	// Validate PID is reasonable (positive and not too large)
	if pid <= 0 || pid > (2<<22) {
		return nil, fmt.Errorf("PID %d is out of reasonable range", pid)
	}

	return &pid, nil
}

// isProcessRunning checks if a process with the given PID is actually running
func (m *Manager) isProcessRunning(pid *int) bool {
	if pid == nil {
		return false
	}
	// syscall.Kill with signal 0 checks for process existence
	err := syscall.Kill(*pid, 0)
	return err == nil
}

// checkQMPStatus checks VM status via QMP
func (m *Manager) checkQMPStatus(ctx context.Context) (alive bool, connected bool, statusDetails map[string]interface{}, err error) {
	qmpClient := internal.NewQMPClient(m.vmEntry.QmpSocketPath())

	// Try to connect to QMP
	if err := qmpClient.Connect(ctx); err != nil {
		return false, false, nil, fmt.Errorf("failed to connect to QMP: %w", err)
	}
	defer qmpClient.Close()

	connected = true

	// Check if VM is running via QMP
	alive = qmpClient.IsRunning(ctx)

	// Get detailed status if possible
	statusDetails = make(map[string]interface{})
	if status, err := qmpClient.CheckStatus(ctx); err == nil {
		statusDetails = status
	}

	return alive, connected, statusDetails, nil
}

// forceKillPID sends SIGKILL to the process
func (m *Manager) forceKillPID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	if err := process.Signal(os.Kill); err != nil {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}

	return nil
}

// cleanupRuntimeFiles removes runtime files for the VM
func (m *Manager) cleanupRuntimeFiles() error {
	files := []string{
		m.vmEntry.PidFilePath(),
		m.vmEntry.SerialFilePath(),
		m.vmEntry.QmpSocketPath(),
		m.vmEntry.MonitorSocketPath(),
		m.vmEntry.SshConfigPath(),
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", file, err)
		}
	}

	return nil
}

// getSSHPort retrieves the SSH port from the VM configuration
func (m *Manager) getSSHPort() interface{} {
	// Try the new nested structure first (vm.ssh.port)
	if sshData, ok := m.vmEntry.Vars["ssh"].(map[string]interface{}); ok {
		if port, exists := sshData["port"]; exists {
			return port
		}
	}

	// Fall back to the old structure (ssh_host)
	if port, exists := m.vmEntry.Vars["ssh_host"]; exists {
		return port
	}

	return nil
}
