// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get [vm-name] [remote-path] [local-path]",
	Short: "Copy a file from a virtual machine",
	Long:  `Copy a file from a virtual machine to the local system using SCP.`,
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		vmName := args[0]
		remotePath := args[1]
		localPath := args[2]

		// Load configuration and get VM status
		cfg, _, status, err := loadVMAndCheckStatus(vmName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Get SSH connection info
		sshConfigPath, sshPort, err := getSSHConnectionInfo(cfg, vmName, status)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Execute SCP command to download file
		if err := executeSCPGet(sshConfigPath, sshPort, remotePath, localPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing SCP: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully copied %s from VM %s to %s\n", remotePath, vmName, localPath)
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}

// executeSCPGet runs the SCP command to copy a file from VM to local
func executeSCPGet(sshConfigPath string, sshPort int64, remotePath, localPath string) error {
	// Build SCP command arguments
	args := []string{
		"-F", sshConfigPath, // Use generated SSH config
		"-P", fmt.Sprintf("%d", sshPort), // SCP port (capital P)
		fmt.Sprintf("localhost:%s", remotePath), // Remote file path
		localPath,                               // Local file path
	}

	// Create command
	scpCmd := exec.Command("scp", args...)

	// Set up stdin/stdout/stderr
	scpCmd.Stdin = os.Stdin
	scpCmd.Stdout = os.Stdout
	scpCmd.Stderr = os.Stderr

	// Execute SCP command
	return scpCmd.Run()
}
