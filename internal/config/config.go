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
	Qemu   QemuConfig             `toml:"qemu"`
	VMs    map[string]VMConfig    `toml:"vm"`
	Images map[string]ImageConfig `toml:"img"`
	Vars   map[string]interface{} `toml:"vars"`
	SSH    map[string]interface{} `toml:"ssh"`
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

// ImageConfig represents the configuration for an image
type ImageConfig struct {
	Builder   string                 `toml:"builder"` // Required: "raw" or "cloud-init"
	ImgSize   string                 `toml:"img_size"`
	BaseImg   *BaseImageConfig       `toml:"base_img,omitempty"`
	Env       map[string]interface{} `toml:"env,omitempty"`
	EnvHook   *EnvHookConfig         `toml:"env_hook,omitempty"`
	Templates []TemplateConfig       `toml:"templates,omitempty"`
	Sources   []SourceConfig         `toml:"sources,omitempty"`
	BuildArgs []string               `toml:"build_args,omitempty"`
}

// BaseImageConfig represents configuration for a base image
type BaseImageConfig struct {
	URL       string `toml:"url"`
	SHA256Sum string `toml:"sha256sum"`
}

// EnvHookConfig represents configuration for an environment hook
type EnvHookConfig struct {
	Interpreter string `toml:"interpreter"`
	Script      string `toml:"script"`
}

// TemplateConfig represents configuration for a template
type TemplateConfig struct {
	Template string `toml:"template"`
	Output   string `toml:"output"`
}

// SourceConfig represents configuration for an additional source
type SourceConfig struct {
	URL       string `toml:"url"`
	SHA256Sum string `toml:"sha256sum"`
	Filename  string `toml:"filename"`
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

// Get path to the global configuration file
func GlobalConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".config", "qqmgr", "conf.toml"), nil
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
	globalPath, err := GlobalConfigPath()
	if err == nil {
		if _, err := os.Stat(globalPath); err == nil {
			return globalPath, nil
		}
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

	// if using the global config file
	globalPath, err := GlobalConfigPath()
	if err == nil && globalPath == path {
		return filepath.Join(filepath.Dir(globalPath), "qqmgr"), nil
	}

	// otherwise, expect a directory (matching the config file name) under .qqmgr
	return filepath.Join(filepath.Dir(path), ".qqmgr", filepath.Base(configPath)), nil
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

	// Validate image configurations
	if err := config.validateImageConfig(); err != nil {
		return nil, fmt.Errorf("image configuration validation failed: %w", err)
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
func (c *Config) ResolveVM(vmName string, configPath string, imgMap map[string]interface{}) (*VmEntry, error) {
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

	// Add image map under "img" key
	data["img"] = imgMap

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

// ListImages returns a list of configured image names
func (c *Config) ListImages() []string {
	var images []string
	for name := range c.Images {
		images = append(images, name)
	}
	return images
}

// GetImage returns the configuration for a specific image
func (c *Config) GetImage(imgName string) (*ImageConfig, error) {
	img, exists := c.Images[imgName]
	if !exists {
		return nil, fmt.Errorf("image '%s' not found in configuration", imgName)
	}
	return &img, nil
}

// validateImageConfig ensures all images have proper configuration
func (c *Config) validateImageConfig() error {
	for imgName, img := range c.Images {
		if img.Builder == "" {
			return fmt.Errorf("image '%s' missing required builder configuration", imgName)
		}

		if img.Builder != "raw" && img.Builder != "cloud-init" {
			return fmt.Errorf("image '%s' has invalid builder type: %s (must be 'raw' or 'cloud-init')", imgName, img.Builder)
		}

		if img.ImgSize == "" {
			return fmt.Errorf("image '%s' missing required img_size configuration", imgName)
		}

		// For cloud-init images, require base image
		if img.Builder == "cloud-init" && img.BaseImg == nil {
			return fmt.Errorf("cloud-init image '%s' missing required base_img configuration", imgName)
		}
	}
	return nil
}
