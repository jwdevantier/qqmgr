// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package tail

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ShowLastLines displays the last N lines from a file
func ShowLastLines(filePath string, lines int) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
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

// FollowFileOutput continuously monitors a file for new output
func FollowFileOutput(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Seek to end of file to start from current position
	if _, err := file.Seek(0, 2); err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	// Create a buffered reader
	reader := bufio.NewReader(file)

	fmt.Printf("Following output from %s (Ctrl+C to stop)...\n", filepath.Base(filePath))

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

				file, err = os.Open(filePath)
				if err != nil {
					return fmt.Errorf("failed to reopen file: %w", err)
				}

				reader = bufio.NewReader(file)
				continue
			}

			// For EOF, just wait a bit and continue
			if strings.Contains(err.Error(), "EOF") {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			return fmt.Errorf("error reading file: %w", err)
		}

		// Print the line without the trailing newline (ReadString includes it)
		fmt.Print(line)
	}
}

// DisplayFileOutput shows file output either as last N lines or following mode
func DisplayFileOutput(filePath string, follow bool, lines int) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	if follow {
		return FollowFileOutput(filePath)
	} else {
		return ShowLastLines(filePath, lines)
	}
}
