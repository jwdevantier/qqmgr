// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"qqmgr/internal/config"
	"qqmgr/internal/vm"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [vm-name]",
	Short: "Start a virtual machine",
	Long:  `Start a virtual machine by name. The VM must be defined in the configuration file.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vmName := args[0]

		// Load configuration
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		// Resolve VM configuration
		vmEntry, err := cfg.ResolveVM(vmName, configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving VM configuration: %v\n", err)
			os.Exit(1)
		}

		// Validate arguments to prevent conflicts with auto-injected args
		if err := validateVMArguments(vmEntry.Cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error validating VM arguments: %v\n", err)
			os.Exit(1)
		}

		// Create VM manager
		manager := vm.NewManager(vmEntry)

		// Check if VM is already running
		status, err := manager.GetStatus(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking VM status: %v\n", err)
			os.Exit(1)
		}

		if status.IsRunning {
			fmt.Printf("VM '%s' is already running (PID: %d)\n", vmName, *status.PID)
			return
		}

		// Create runtime directory
		if err := os.MkdirAll(vmEntry.DataDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating runtime directory: %v\n", err)
			os.Exit(1)
		}

		// Start the VM
		if err := startVM(vmEntry); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting VM: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("VM '%s' started successfully\n", vmName)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

// validateVMArguments checks that the user hasn't specified arguments that conflict with auto-injected ones
func validateVMArguments(cmd []string) error {
	conflictingArgs := []string{"-serial", "-qmp", "-monitor", "-pidfile"}

	for _, arg := range cmd {
		// Split the argument in case it contains multiple options
		parts := strings.Fields(arg)
		for _, part := range parts {
			for _, conflicting := range conflictingArgs {
				// Check for exact match or argument with value (e.g., -serial file:output.txt)
				if part == conflicting || strings.HasPrefix(part, conflicting+" ") || strings.HasPrefix(part, conflicting+"=") {
					return fmt.Errorf("conflicting argument '%s' found in VM command. These arguments are auto-injected by qqmgr: %v", part, conflictingArgs)
				}
			}
		}
	}

	return nil
}

// startVM starts the QEMU process with proper error handling
func startVM(vmEntry *config.VmEntry) error {
	// Get the full command with auto-injected arguments
	fullCmd := vmEntry.GetFullCommand()

	// Find QEMU binary from config or use default
	qemuBin := "qemu-system-x86_64" // default
	if configFile != "" {
		if cfg, err := config.LoadFromFile(configFile); err == nil && cfg.Qemu.Bin != "" {
			qemuBin = cfg.Qemu.Bin
		}
	}

	// Print debug information if debug flag is enabled
	if debugFlag {
		fmt.Fprintf(os.Stderr, "DEBUG: QEMU binary: %s\n", qemuBin)
		fmt.Fprintf(os.Stderr, "DEBUG: Full QEMU command:\n")
		fmt.Fprintf(os.Stderr, "  %s %s\n", qemuBin, strings.Join(fullCmd, " "))
		fmt.Fprintf(os.Stderr, "DEBUG: Command arguments:\n")
		for i, arg := range fullCmd {
			fmt.Fprintf(os.Stderr, "  [%d] %s\n", i, arg)
		}
	}

	// Build the command
	cmd := exec.Command(qemuBin, fullCmd...)

	// Set up stderr capture for error reporting
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start QEMU process: %w", err)
	}

	// Create a channel to capture stderr output
	stderrCh := make(chan string)
	go func() {
		defer close(stderrCh)
		buffer := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(buffer)
			if n > 0 {
				stderrCh <- string(buffer[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for the process to either start successfully or fail
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Wait for either process completion or successful startup
	select {
	case err := <-done:
		// Process exited - this usually means an error
		stderrOutput := ""
		for output := range stderrCh {
			stderrOutput += output
		}

		if stderrOutput != "" {
			return fmt.Errorf("QEMU failed to start:\n%s", stderrOutput)
		}
		return fmt.Errorf("QEMU process exited unexpectedly: %w", err)

	case <-time.After(5 * time.Second):
		// Check if process is still running and QMP socket is available
		if cmd.Process == nil {
			return fmt.Errorf("QEMU process failed to start")
		}

		// Check if QMP socket is created (indicates successful startup)
		if _, err := os.Stat(vmEntry.QmpSocketPath()); err == nil {
			// Success! Process is running and QMP socket is available
			return nil
		}

		// Give it a bit more time for socket creation
		time.Sleep(1 * time.Second)
		if _, err := os.Stat(vmEntry.QmpSocketPath()); err == nil {
			return nil
		}

		// Still no socket, check if process is still running
		if cmd.Process == nil {
			return fmt.Errorf("QEMU process failed to start")
		}

		// Collect any stderr output for debugging
		stderrOutput := ""
		for {
			select {
			case output := <-stderrCh:
				stderrOutput += output
			default:
				goto checkProcess
			}
		}
	checkProcess:

		// Check if process is still running
		if err := cmd.Process.Signal(os.Signal(nil)); err != nil {
			// Process is not running
			if stderrOutput != "" {
				return fmt.Errorf("QEMU failed to start:\n%s", stderrOutput)
			}
			return fmt.Errorf("QEMU process failed to start")
		}

		// Process is running but no QMP socket - this might be normal for some VMs
		// that don't use QMP, so we'll consider it a success
		return nil
	}
}
