// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Qemu QemuConfig             `toml:"qemu"`
	VMs  map[string]VMConfig    `toml:"vm"`
	Vars map[string]interface{} `toml:"vars"`
	SSH  map[string]interface{} `toml:"ssh"`
}

type QemuConfig struct {
	Bin string `toml:"bin"`
	Img string `toml:"img"`
}

type SSHConfig struct {
	Port    int64                  `toml:"port"`
	VMPort  int64                  `toml:"vm_port"`
	Options map[string]interface{} `toml:"-"` // All other SSH options
}

// UnmarshalTOML implements custom unmarshaling to capture all SSH options
func (s *SSHConfig) UnmarshalTOML(data interface{}) error {
	if data == nil {
		return nil
	}

	// Initialize Options map
	s.Options = make(map[string]interface{})

	// Handle map[string]interface{} from TOML
	if m, ok := data.(map[string]interface{}); ok {
		for k, v := range m {
			switch k {
			case "port":
				if port, ok := v.(int64); ok {
					s.Port = port
				}
			case "vm_port":
				if vmPort, ok := v.(int64); ok {
					s.VMPort = vmPort
				}
			default:
				// Store all other options
				s.Options[k] = v
			}
		}
	}

	return nil
}

type VMConfig struct {
	Cmd  []string               `toml:"cmd"`
	Vars map[string]interface{} `toml:"vars"`
	SSH  SSHConfig              `toml:"ssh"`
}

// VmEntry represents a resolved VM configuration with runtime information
type VmEntry struct {
	Name    string                 // VM name
	Cmd     []string               // Resolved command arguments
	Vars    map[string]interface{} // VM variables
	DataDir string                 // Runtime directory for this VM
}

// PidFilePath returns the path to the PID file
func (v *VmEntry) PidFilePath() string {
	absPath, _ := filepath.Abs(filepath.Join(v.DataDir, "pid"))
	return absPath
}

// SerialFilePath returns the path to the serial file
func (v *VmEntry) SerialFilePath() string {
	absPath, _ := filepath.Abs(filepath.Join(v.DataDir, "serial"))
	return absPath
}

// QmpSocketPath returns the path to the QMP socket
func (v *VmEntry) QmpSocketPath() string {
	absPath, _ := filepath.Abs(filepath.Join(v.DataDir, "qmp.socket"))
	return absPath
}

// MonitorSocketPath returns the path to the monitor socket
func (v *VmEntry) MonitorSocketPath() string {
	absPath, _ := filepath.Abs(filepath.Join(v.DataDir, "monitor.socket"))
	return absPath
}

// SshConfigPath returns the path to the SSH config file
func (v *VmEntry) SshConfigPath() string {
	absPath, _ := filepath.Abs(filepath.Join(v.DataDir, "ssh.conf"))
	return absPath
}

// GetAutoInjectedArgs returns the auto-injected QEMU arguments as specified in the design
func (v *VmEntry) GetAutoInjectedArgs() []string {
	return []string{
		"-pidfile", v.PidFilePath(),
		"-monitor",
		fmt.Sprintf("unix:%s,server,nowait", v.MonitorSocketPath()),
		"-serial",
		fmt.Sprintf("file:%s", v.SerialFilePath()),
		"-qmp",
		fmt.Sprintf("unix:%s,server,nowait", v.QmpSocketPath()),
	}
}

// GetFullCommand returns the complete command with auto-injected arguments
func (v *VmEntry) GetFullCommand() []string {
	var allArgs []string

	// Split each command part into individual arguments
	for _, cmdPart := range v.Cmd {
		args := strings.Fields(cmdPart)
		allArgs = append(allArgs, args...)
	}

	// Add auto-injected arguments
	allArgs = append(allArgs, v.GetAutoInjectedArgs()...)

	return allArgs
}

// FindConfigPath determines the configuration file path to use
// It checks in order: provided path, current directory, global location
func FindConfigPath(providedPath string) (string, error) {
	// If a path is provided, use it
	if providedPath != "" {
		if _, err := os.Stat(providedPath); err != nil {
			return "", fmt.Errorf("provided config file not found: %s", providedPath)
		}
		return providedPath, nil
	}

	// Try current directory first
	if _, err := os.Stat("qqmgr.toml"); err == nil {
		return "qqmgr.toml", nil
	}

	// Try global config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	globalPath := filepath.Join(homeDir, ".config", "qqmgr", "conf.toml")
	if _, err := os.Stat(globalPath); err == nil {
		return globalPath, nil
	}

	return "", fmt.Errorf("no configuration file found (looked for ./qqmgr.toml and %s)", globalPath)
}

// LoadConfig loads configuration from the determined path
func LoadConfig(configPath string) (*Config, error) {
	path, err := FindConfigPath(configPath)
	if err != nil {
		return nil, err
	}
	return LoadFromFile(path)
}

// GetRuntimeDir determines the runtime directory based on config file location
func GetRuntimeDir(configPath string) (string, error) {
	path, err := FindConfigPath(configPath)
	if err != nil {
		return "", err
	}

	// If config is in current directory, use ./.qqmgr/
	if path == "qqmgr.toml" {
		return ".qqmgr", nil
	}

	// If config is global, use ~/.config/qqmgr/
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	globalPath := filepath.Join(homeDir, ".config", "qqmgr", "conf.toml")
	if path == globalPath {
		return filepath.Join(homeDir, ".config", "qqmgr"), nil
	}

	// For custom config files, use the directory containing the config file
	return filepath.Dir(path), nil
}

// LoadFromFile loads configuration from a specific file path
func LoadFromFile(path string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, fmt.Errorf("failed to decode config file %s: %w", path, err)
	}

	// Validate SSH configuration for all VMs
	if err := config.validateSSHConfig(); err != nil {
		return nil, fmt.Errorf("SSH configuration validation failed: %w", err)
	}

	return &config, nil
}

// validateSSHConfig ensures all VMs have proper SSH configuration
func (c *Config) validateSSHConfig() error {
	for vmName, vm := range c.VMs {
		if vm.SSH.Port == 0 {
			return fmt.Errorf("VM '%s' missing required SSH port configuration", vmName)
		}
		if vm.SSH.VMPort == 0 {
			// Set default VM port if not specified
			vm.SSH.VMPort = 22
		}

		// Initialize Options map if not present
		if vm.SSH.Options == nil {
			vm.SSH.Options = make(map[string]interface{})
		}

		c.VMs[vmName] = vm
	}
	return nil
}

// ResolveVM resolves template variables in VM configuration and returns a VmEntry
func (c *Config) ResolveVM(vmName string, configPath string) (*VmEntry, error) {
	vm, exists := c.VMs[vmName]
	if !exists {
		return nil, fmt.Errorf("VM '%s' not found in configuration", vmName)
	}

	// Get runtime directory
	runtimeDir, err := GetRuntimeDir(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to determine runtime directory: %w", err)
	}

	// Build the template data
	data := make(map[string]interface{})

	// Add global vars at root level
	for k, v := range c.Vars {
		data[k] = v
	}

	// Create VM data structure with vars and ssh
	vmData := make(map[string]interface{})
	if vm.Vars != nil {
		for k, v := range vm.Vars {
			vmData[k] = v
		}
	}

	// Add SSH configuration under "vm.ssh" key
	vmData["ssh"] = map[string]interface{}{
		"port":    vm.SSH.Port,
		"vm_port": vm.SSH.VMPort,
	}

	// Add VM data under "vm" key
	data["vm"] = vmData

	var resolved []string
	for _, cmdPart := range vm.Cmd {
		// First pass: resolve VM variables
		tmpl := template.New("cmd")
		tmpl, err := tmpl.Parse(cmdPart)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template in command: %w", err)
		}

		var buf bytes.Buffer
		err = tmpl.Execute(&buf, data)
		if err != nil {
			return nil, fmt.Errorf("failed to execute template: %w", err)
		}

		// Second pass: resolve any remaining global variables
		intermediate := buf.String()
		tmpl2 := template.New("cmd2")
		tmpl2, err = tmpl2.Parse(intermediate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse intermediate template: %w", err)
		}

		var finalBuf bytes.Buffer
		err = tmpl2.Execute(&finalBuf, data)
		if err != nil {
			return nil, fmt.Errorf("failed to execute final template: %w", err)
		}

		resolved = append(resolved, finalBuf.String())
	}

	// Create VM-specific runtime directory
	vmDataDir := filepath.Join(runtimeDir, "vm."+vmName)

	return &VmEntry{
		Name:    vmName,
		Cmd:     resolved,
		Vars:    vmData, // Store the resolved VM data including SSH
		DataDir: vmDataDir,
	}, nil
}

// ListVMs returns a list of configured VM names
func (c *Config) ListVMs() []string {
	var vms []string
	for name := range c.VMs {
		vms = append(vms, name)
	}
	return vms
}
