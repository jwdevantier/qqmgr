// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var putCmd = &cobra.Command{
	Use:   "put [vm-name] [local-path] [remote-path]",
	Short: "Copy a file to a virtual machine",
	Long:  `Copy a local file to a virtual machine using SCP.`,
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		vmName := args[0]
		localPath := args[1]
		remotePath := args[2]

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

		// Execute SCP command to upload file
		if err := executeSCPPut(sshConfigPath, sshPort, localPath, remotePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing SCP: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully copied %s to %s on VM %s\n", localPath, remotePath, vmName)
	},
}

func init() {
	rootCmd.AddCommand(putCmd)
}

// executeSCPPut runs the SCP command to copy a file from local to VM
func executeSCPPut(sshConfigPath string, sshPort int64, localPath, remotePath string) error {
	// Build SCP command arguments
	args := []string{
		"-F", sshConfigPath, // Use generated SSH config
		"-P", fmt.Sprintf("%d", sshPort), // SCP port (capital P)
		localPath,                               // Local file path
		fmt.Sprintf("localhost:%s", remotePath), // Remote file path
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
