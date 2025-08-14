// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"qqmgr/internal"
	"qqmgr/internal/config"
	"qqmgr/internal/vm"
	"qqmgr/internal/vmutil"

	"github.com/spf13/cobra"
)

var gdbCmd = &cobra.Command{
	Use:   "gdb [vm-name] [-- gdb-flags...]",
	Short: "Debug QEMU with GDB",
	Long:  `Start GDB with the QEMU binary and VM arguments pre-configured. Additional GDB flags can be passed after --.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vmName := args[0]

		// Parse additional GDB flags (everything after the VM name)
		gdbFlags := args[1:]

		// Load configuration
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		// Create AppContext
		appCtx, err := internal.NewAppContext(cfg, configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating app context: %v\n", err)
			os.Exit(1)
		}
		defer appCtx.Close()

		// Resolve VM configuration
		vmEntry, err := appCtx.ResolveVM(vmName)
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
			fmt.Printf("Use 'gdb -p %d' to attach to the running process instead.\n", *status.PID)
			return
		}

		// Create runtime directory
		if err := os.MkdirAll(vmEntry.DataDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating runtime directory: %v\n", err)
			os.Exit(1)
		}

		// Delete existing stdout/stderr log files since we won't capture them
		vmutil.DeleteLogFiles(vmEntry)

		// Generate and launch GDB
		if err := launchGDB(appCtx.Config.Qemu.Bin, vmEntry, gdbFlags); err != nil {
			fmt.Fprintf(os.Stderr, "Error launching GDB: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(gdbCmd)
}

// launchGDB creates a temporary GDB commands file and launches GDB
func launchGDB(qemuBin string, vmEntry *config.VmEntry, gdbFlags []string) error {
	// Get the full command with auto-injected arguments
	fullCmd := vmEntry.GetFullCommand()

	// Create a temporary file for GDB commands
	tmpFile, err := os.CreateTemp("", "qqmgr-gdbcmds-*.gdb")
	if err != nil {
		return fmt.Errorf("failed to create temporary GDB commands file: %w", err)
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name()) // Clean up when we're done

	// Generate the GDB commands content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("file %s\n", qemuBin))
	content.WriteString(fmt.Sprintf("set args %s\n", strings.Join(fullCmd, " ")))
	content.WriteString("handle SIGUSR1 nostop noprint pass\n")
	content.WriteString("echo \\n=== Setup Complete ===\\n\n")
	content.WriteString("echo Type 'r' or 'run' to start the VM\\n")

	// Write to the temporary file
	if _, err := tmpFile.WriteString(content.String()); err != nil {
		return fmt.Errorf("failed to write GDB commands to temporary file: %w", err)
	}

	// Close the file before passing it to GDB
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary GDB commands file: %w", err)
	}

	// Build GDB command arguments
	gdbArgs := []string{"-x", tmpFile.Name()}
	gdbArgs = append(gdbArgs, gdbFlags...)

	// Launch GDB
	gdbCmd := exec.Command("gdb", gdbArgs...)
	gdbCmd.Stdin = os.Stdin
	gdbCmd.Stdout = os.Stdout
	gdbCmd.Stderr = os.Stderr

	return gdbCmd.Run()
}
