// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"qqmgr/internal"
	"qqmgr/internal/config"
	"qqmgr/internal/vm"

	"github.com/spf13/cobra"
)

var jsonOutput bool

var statusCmd = &cobra.Command{
	Use:   "status [vm-name]",
	Short: "Show virtual machine status",
	Long:  `Show the running status, ports, and socket information for a virtual machine.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vmName := args[0]

		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		// Create AppContext
		appCtx, err := internal.NewAppContext(cfg, configFile)
		if err != nil {
			fmt.Printf("Error creating app context: %v\n", err)
			return
		}
		defer appCtx.Close()

		// Resolve VM configuration
		vmEntry, err := appCtx.ResolveVM(vmName)
		if err != nil {
			fmt.Printf("Error resolving VM '%s': %v\n", vmName, err)
			return
		}

		// Debug: print VM configuration if debug flag is enabled
		if debugFlag {
			fmt.Fprintf(os.Stderr, "DEBUG: VM Vars: %+v\n", vmEntry.Vars)
		}

		// Create VM manager
		manager := vm.NewManager(vmEntry)

		// Get VM status with QMP-based checking
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := manager.GetStatus(ctx)
		if err != nil {
			fmt.Printf("Error getting VM status: %v\n", err)
			return
		}

		if jsonOutput {
			// JSON output
			result := map[string]interface{}{
				"name":          status.Name,
				"pid":           status.PID,
				"pid_file":      status.PIDFile,
				"running":       status.IsRunning,
				"alive":         status.IsAlive,
				"qmp_connected": status.QMPConnected,
				"ssh": map[string]interface{}{
					"port":   status.SSHPort,
					"config": status.SSHConfig,
				},
				"serial_file":    status.SerialFile,
				"qmp_socket":     status.QMPSocket,
				"monitor_socket": status.MonitorSocket,
				"qemu_stdout":    vmEntry.QemuStdoutPath(),
				"qemu_stderr":    vmEntry.QemuStderrPath(),
			}

			// Add status details if available
			if status.StatusDetails != nil {
				result["status_details"] = status.StatusDetails
			}

			jsonData, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				fmt.Printf("Error marshaling JSON: %v\n", err)
				return
			}
			fmt.Println(string(jsonData))
		} else {
			// Human-readable output
			fmt.Printf("Status for VM: %s\n", vmName)
			fmt.Printf("  Configured: yes\n")

			if status.IsRunning {
				if status.PID != nil {
					fmt.Printf("  Running: yes (PID: %d)\n", *status.PID)
				} else {
					fmt.Printf("  Running: yes\n")
				}

				if status.IsAlive {
					fmt.Printf("  Alive: yes (QMP responsive)\n")
				} else {
					fmt.Printf("  Alive: no (QMP not responsive)\n")
				}
			} else {
				fmt.Printf("  Running: no\n")
			}

			if status.QMPConnected {
				fmt.Printf("  QMP: connected\n")
			} else {
				fmt.Printf("  QMP: not connected\n")
			}

			fmt.Printf("  SSH Port: %v\n", status.SSHPort)
			fmt.Printf("  SSH Config: %s\n", vmEntry.SshConfigPath())
			fmt.Printf("  PID File: %s\n", status.PIDFile)
			fmt.Printf("  Serial File: %s\n", status.SerialFile)
			fmt.Printf("  QMP Socket: %s\n", status.QMPSocket)
			fmt.Printf("  Monitor Socket: %s\n", status.MonitorSocket)
			fmt.Printf("  QEMU Stdout: %s\n", vmEntry.QemuStdoutPath())
			fmt.Printf("  QEMU Stderr: %s\n", vmEntry.QemuStderrPath())

			// Show status details if available
			if status.StatusDetails != nil {
				if statusStr, ok := status.StatusDetails["status"].(string); ok {
					fmt.Printf("  VM Status: %s\n", statusStr)
				}
			}
		}
	},
}

func init() {
	statusCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.AddCommand(statusCmd)
}
