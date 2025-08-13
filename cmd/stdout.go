// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"context"
	"fmt"
	"os"

	"qqmgr/internal"
	"qqmgr/internal/config"
	"qqmgr/internal/tail"
	"qqmgr/internal/vm"

	"github.com/spf13/cobra"
)

var (
	stdoutFollowFlag bool
	stdoutLinesFlag  int
)

var stdoutCmd = &cobra.Command{
	Use:   "stdout [vm-name]",
	Short: "Display QEMU stdout",
	Long: `Display QEMU stdout output. 
By default, shows the last 10 lines. Use --follow to continuously monitor output.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vmName := args[0]

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

		// Create VM manager
		manager := vm.NewManager(vmEntry)

		// Check if VM is running
		status, err := manager.GetStatus(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking VM status: %v\n", err)
			os.Exit(1)
		}

		if !status.IsRunning {
			fmt.Fprintf(os.Stderr, "VM '%s' is not running\n", vmName)
			os.Exit(1)
		}

		// Display stdout output
		if err := tail.DisplayFileOutput(vmEntry.QemuStdoutPath(), stdoutFollowFlag, stdoutLinesFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error displaying stdout output: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	stdoutCmd.Flags().BoolVarP(&stdoutFollowFlag, "follow", "f", false, "Follow the stdout output (like tail -f)")
	stdoutCmd.Flags().IntVarP(&stdoutLinesFlag, "lines", "n", 10, "Number of lines to show (default: 10)")
	rootCmd.AddCommand(stdoutCmd)
}
