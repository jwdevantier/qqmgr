// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"qqmgr/internal/config"
)

// GenerateSSHConfig generates an SSH config file for a specific VM
func GenerateSSHConfig(cfg *config.Config, vmName string, configPath string) (string, error) {
	vm, exists := cfg.VMs[vmName]
	if !exists {
		return "", fmt.Errorf("VM '%s' not found in configuration", vmName)
	}

	// Get the VM entry to determine the config file path
	vmEntry, err := cfg.ResolveVM(vmName, configPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve VM: %w", err)
	}

	sshConfigPath := vmEntry.SshConfigPath()

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(sshConfigPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create SSH config directory: %w", err)
	}

	// Create the SSH config file
	file, err := os.Create(sshConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to create SSH config file: %w", err)
	}
	defer file.Close()

	// Create control directory for SSH control sockets
	controlDir := filepath.Join(filepath.Dir(sshConfigPath), "ssh")
	if err := os.MkdirAll(controlDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create SSH control directory: %w", err)
	}

	// Write global SSH options with ControlPath fix
	for key, value := range cfg.SSH {
		if key == "ControlPath" {
			// Convert relative ControlPath to absolute path
			if strValue, ok := value.(string); ok {
				if !filepath.IsAbs(strValue) {
					// Replace relative path with absolute path in control directory
					controlPath := filepath.Join(controlDir, filepath.Base(strValue))
					fmt.Fprintf(file, "%s %s\n", key, controlPath)
				} else {
					fmt.Fprintf(file, "%s %s\n", key, strValue)
				}
			} else {
				fmt.Fprintf(file, "%s %v\n", key, value)
			}
		} else {
			if strValue, ok := value.(string); ok {
				fmt.Fprintf(file, "%s %s\n", key, strValue)
			} else {
				fmt.Fprintf(file, "%s %v\n", key, value)
			}
		}
	}

	// Write VM-specific SSH options (excluding port and vm_port)
	for key, value := range vm.SSH.Options {
		// Skip lowercase options (port, vm_port)
		if len(key) > 0 && key[0] >= 'a' && key[0] <= 'z' {
			continue
		}
		if strValue, ok := value.(string); ok {
			fmt.Fprintf(file, "%s %s\n", key, strValue)
		} else {
			fmt.Fprintf(file, "%s %v\n", key, value)
		}
	}

	return sshConfigPath, nil
}

// GetSSHOptions returns all SSH options for a VM (global + VM-specific)
func GetSSHOptions(cfg *config.Config, vmName string) (map[string]interface{}, error) {
	vm, exists := cfg.VMs[vmName]
	if !exists {
		return nil, fmt.Errorf("VM '%s' not found in configuration", vmName)
	}

	// Start with global options
	options := make(map[string]interface{})
	for k, v := range cfg.SSH {
		options[k] = v
	}

	// Add VM-specific options (excluding port and vm_port)
	for k, v := range vm.SSH.Options {
		// Skip lowercase options (port, vm_port)
		if len(k) > 0 && k[0] >= 'a' && k[0] <= 'z' {
			continue
		}
		options[k] = v
	}

	return options, nil
}
