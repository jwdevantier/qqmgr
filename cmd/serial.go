// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"qqmgr/internal"
	"qqmgr/internal/config"
	"qqmgr/internal/vm"

	"github.com/spf13/cobra"
)

var (
	followFlag bool
	linesFlag  int
)

var serialCmd = &cobra.Command{
	Use:   "serial [vm-name]",
	Short: "Display serial output from a virtual machine",
	Long: `Display serial output from a virtual machine. 
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

		// Display serial output
		if err := displaySerialOutput(vmEntry, followFlag, linesFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error displaying serial output: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	serialCmd.Flags().BoolVarP(&followFlag, "follow", "f", false, "Follow the serial output (like tail -f)")
	serialCmd.Flags().IntVarP(&linesFlag, "lines", "n", 10, "Number of lines to show (default: 10)")
	rootCmd.AddCommand(serialCmd)
}

// displaySerialOutput shows the serial output from the VM
func displaySerialOutput(vmEntry *config.VmEntry, follow bool, lines int) error {
	serialFile := vmEntry.SerialFilePath()

	// Check if serial file exists
	if _, err := os.Stat(serialFile); os.IsNotExist(err) {
		return fmt.Errorf("serial file not found: %s", serialFile)
	}

	if follow {
		return followSerialOutput(serialFile)
	} else {
		return showLastLines(serialFile, lines)
	}
}

// showLastLines displays the last N lines from the serial file
func showLastLines(serialFile string, lines int) error {
	file, err := os.Open(serialFile)
	if err != nil {
		return fmt.Errorf("failed to open serial file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading serial file: %w", err)
	}

	// Show last N lines
	start := 0
	if len(allLines) > lines {
		start = len(allLines) - lines
	}

	for i := start; i < len(allLines); i++ {
		fmt.Println(allLines[i])
	}

	return nil
}

// followSerialOutput continuously monitors the serial file for new output
func followSerialOutput(serialFile string) error {
	file, err := os.Open(serialFile)
	if err != nil {
		return fmt.Errorf("failed to open serial file: %w", err)
	}
	defer file.Close()

	// Seek to end of file to start from current position
	if _, err := file.Seek(0, 2); err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	// Create a buffered reader
	reader := bufio.NewReader(file)

	fmt.Printf("Following serial output from %s (Ctrl+C to stop)...\n", filepath.Base(serialFile))

	// Monitor for new output
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// Check if file was truncated (VM restarted)
			if strings.Contains(err.Error(), "bad file descriptor") ||
				strings.Contains(err.Error(), "no such file or directory") {
				// Try to reopen the file
				file.Close()
				time.Sleep(100 * time.Millisecond)

				file, err = os.Open(serialFile)
				if err != nil {
					return fmt.Errorf("failed to reopen serial file: %w", err)
				}

				reader = bufio.NewReader(file)
				continue
			}

			// For EOF, just wait a bit and continue
			if strings.Contains(err.Error(), "EOF") {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			return fmt.Errorf("error reading serial file: %w", err)
		}

		// Print the line without the trailing newline (ReadString includes it)
		fmt.Print(line)
	}
}
