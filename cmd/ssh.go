// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"qqmgr/internal"
	"qqmgr/internal/config"
	"qqmgr/internal/vm"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh [vm-name] [command]",
	Short: "Connect to a virtual machine via SSH",
	Long:  `Connect to a virtual machine via SSH. If a command is provided, it will be executed on the VM.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vmName := args[0]
		var command string
		if len(args) > 1 {
			command = args[1]
		}

		// Load configuration and get VM status
		cfg, _, status, err := loadVMAndCheckStatus(vmName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Generate SSH config file
		sshConfigPath, err := internal.GenerateSSHConfig(cfg, vmName, configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating SSH config: %v\n", err)
			os.Exit(1)
		}

		// Get SSH port from VM configuration
		sshPort, ok := status.SSHPort.(int64)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: SSH port not configured for VM '%s'\n", vmName)
			os.Exit(1)
		}

		// Execute SSH command
		if err := executeSSH(sshConfigPath, sshPort, command); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing SSH: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(sshCmd)
}

// loadVMAndCheckStatus loads configuration, resolves VM, and checks if it's running
func loadVMAndCheckStatus(vmName string) (*config.Config, *config.VmEntry, *vm.Status, error) {
	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading configuration: %w", err)
	}

	// Resolve VM configuration
	vmEntry, err := cfg.ResolveVM(vmName, configFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolving VM configuration: %w", err)
	}

	// Create VM manager
	manager := vm.NewManager(vmEntry)

	// Check if VM is running
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := manager.GetStatus(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("checking VM status: %w", err)
	}

	if !status.IsRunning {
		return nil, nil, nil, fmt.Errorf("VM '%s' is not running", vmName)
	}

	return cfg, vmEntry, status, nil
}

// getSSHConnectionInfo returns SSH config path and port for a VM
func getSSHConnectionInfo(cfg *config.Config, vmName string, status *vm.Status) (string, int64, error) {
	// Generate SSH config file
	sshConfigPath, err := internal.GenerateSSHConfig(cfg, vmName, configFile)
	if err != nil {
		return "", 0, fmt.Errorf("generating SSH config: %w", err)
	}

	// Get SSH port from VM configuration
	sshPort, ok := status.SSHPort.(int64)
	if !ok {
		return "", 0, fmt.Errorf("SSH port not configured for VM '%s'", vmName)
	}

	return sshConfigPath, sshPort, nil
}

// executeSSH runs the SSH command with the generated config
func executeSSH(sshConfigPath string, sshPort int64, command string) error {
	// Build SSH command arguments
	args := []string{
		"-F", sshConfigPath, // Use generated SSH config
		"-p", fmt.Sprintf("%d", sshPort), // SSH port
		"localhost", // Connect to localhost (port forwarding)
	}

	// Add command if provided
	if command != "" {
		args = append(args, command)
	}

	// Create command
	sshCmd := exec.Command("ssh", args...)

	// Set up stdin/stdout/stderr for interactive session
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	// Execute SSH command
	return sshCmd.Run()
}
