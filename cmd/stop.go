// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"qqmgr/internal/config"
	"qqmgr/internal/vm"

	"github.com/spf13/cobra"
)

var forceFlag bool
var timeoutFlag int

var stopCmd = &cobra.Command{
	Use:   "stop [vm-name]",
	Short: "Stop a virtual machine",
	Long:  `Stop a virtual machine gracefully. If the VM doesn't stop within the timeout, it will be force-killed.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vmName := args[0]
		fmt.Printf("Stopping VM: %s\n", vmName)

		// Load configuration
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Resolve VM configuration
		vmEntry, err := cfg.ResolveVM(vmName, configFile)
		if err != nil {
			fmt.Printf("Error resolving VM '%s': %v\n", vmName, err)
			os.Exit(1)
		}

		// Create VM manager
		manager := vm.NewManager(vmEntry)

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutFlag)*time.Second)
		defer cancel()

		// Get initial status
		status, err := manager.GetStatus(ctx)
		if err != nil {
			fmt.Printf("Error getting VM status: %v\n", err)
			os.Exit(1)
		}

		if !status.IsRunning {
			fmt.Printf("VM '%s' is not running\n", vmName)
			return
		}

		if status.PID != nil {
			fmt.Printf("VM is running with PID: %d\n", *status.PID)
		} else {
			fmt.Printf("VM is running (PID not available)\n")
		}

		// Stop the VM
		fmt.Printf("Attempting to stop VM...\n")
		success, err := manager.Stop(ctx, time.Duration(timeoutFlag)*time.Second, forceFlag)
		if err != nil {
			fmt.Printf("Failed to stop VM: %v\n", err)
			os.Exit(1)
		}

		if success {
			fmt.Printf("VM '%s' stopped successfully\n", vmName)
		} else {
			fmt.Printf("Failed to stop VM '%s'\n", vmName)
			os.Exit(1)
		}
	},
}

func init() {
	stopCmd.Flags().BoolVar(&forceFlag, "force", true, "Force kill if graceful shutdown fails")
	stopCmd.Flags().IntVar(&timeoutFlag, "timeout", 20, "Timeout in seconds for graceful shutdown")
	rootCmd.AddCommand(stopCmd)
}
