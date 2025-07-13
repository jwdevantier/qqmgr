// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFindConfigPath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		providedPath string
		setupFiles   map[string]string
		wantPath     string
		wantErr      bool
	}{
		{
			name:         "provided path exists",
			providedPath: filepath.Join(tempDir, "test.toml"),
			setupFiles: map[string]string{
				"test.toml": `[qemu]
bin = "qemu-system-x86_64"
img = "qemu-img"`,
			},
			wantPath: filepath.Join(tempDir, "test.toml"),
			wantErr:  false,
		},
		{
			name:         "provided path does not exist",
			providedPath: filepath.Join(tempDir, "nonexistent.toml"),
			wantErr:      true,
		},
		{
			name: "local config exists",
			setupFiles: map[string]string{
				"qqmgr.toml": `[qemu]
bin = "qemu-system-x86_64"
img = "qemu-img"`,
			},
			wantPath: "qqmgr.toml",
			wantErr:  false,
		},
		{
			name:       "no config files exist",
			setupFiles: map[string]string{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files
			for filename, content := range tt.setupFiles {
				var filePath string
				if filename == "qqmgr.toml" {
					filePath = filename // Use current directory for local config
				} else {
					filePath = filepath.Join(tempDir, filename)
				}
				err := os.WriteFile(filePath, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", filePath, err)
				}
			}

			// Clean up qqmgr.toml for the 'no config files exist' test
			if tt.name == "no config files exist" {
				_ = os.Remove("qqmgr.toml")
			}

			gotPath, err := FindConfigPath(tt.providedPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindConfigPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && gotPath != tt.wantPath {
				t.Errorf("FindConfigPath() = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    *Config
		wantErr bool
	}{
		{
			name: "valid config",
			content: `[qemu]
bin = "qemu-system-x86_64"
img = "qemu-img"

[vars]
home = "/home/user"

[ssh]
ServerAliveInterval = 300
ServerAliveCountMax = 3
UserKnownHostsFile = "/dev/null"
StrictHostKeyChecking = "no"

[vm.test-vm]
cmd = [
    "-nodefaults",
    "-machine q35,accel=kvm",
    "-cpu host -smp 2 -m 4096"
]

[vm.test-vm.vars]
ssh_host = 2089
ssh_vm = 22
boot_img = "{{.home}}/path/to/disk.img"

[vm.test-vm.ssh]
port = 2089
vm_port = 22
ServerAliveInterval = 60
Compression = "yes"`,
			want: &Config{
				Qemu: QemuConfig{
					Bin: "qemu-system-x86_64",
					Img: "qemu-img",
				},
				Vars: map[string]interface{}{
					"home": "/home/user",
				},
				VMs: map[string]VMConfig{
					"test-vm": {
						Cmd: []string{
							"-nodefaults",
							"-machine q35,accel=kvm",
							"-cpu host -smp 2 -m 4096",
						},
						Vars: map[string]interface{}{
							"ssh_host": int64(2089),
							"ssh_vm":   int64(22),
							"boot_img": "{{.home}}/path/to/disk.img",
						},
						SSH: SSHConfig{
							Port:   2089,
							VMPort: 22,
							Options: map[string]interface{}{
								"ServerAliveInterval": int64(60),
								"Compression":         "yes",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid TOML",
			content: `[qemu
bin = "qemu-system-x86_64"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tempDir, "test.toml")
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			got, err := LoadFromFile(testFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Check Qemu, Vars, and VMs fields
			if !reflect.DeepEqual(got.Qemu, tt.want.Qemu) {
				t.Errorf("Qemu = %+v, want %+v", got.Qemu, tt.want.Qemu)
			}
			if !reflect.DeepEqual(got.Vars, tt.want.Vars) {
				t.Errorf("Vars = %+v, want %+v", got.Vars, tt.want.Vars)
			}
			if len(got.VMs) != len(tt.want.VMs) {
				t.Errorf("VMs length = %d, want %d", len(got.VMs), len(tt.want.VMs))
			}
			for name, wantVM := range tt.want.VMs {
				gotVM, ok := got.VMs[name]
				if !ok {
					t.Errorf("VM %s missing in loaded config", name)
					continue
				}
				if !reflect.DeepEqual(gotVM.Cmd, wantVM.Cmd) {
					t.Errorf("VM %s Cmd = %+v, want %+v", name, gotVM.Cmd, wantVM.Cmd)
				}
				if !reflect.DeepEqual(gotVM.Vars, wantVM.Vars) {
					t.Errorf("VM %s Vars = %+v, want %+v", name, gotVM.Vars, wantVM.Vars)
				}
				// Check SSH fields
				if gotVM.SSH.Port != wantVM.SSH.Port {
					t.Errorf("VM %s SSH.Port = %v, want %v", name, gotVM.SSH.Port, wantVM.SSH.Port)
				}
				if gotVM.SSH.VMPort != wantVM.SSH.VMPort {
					t.Errorf("VM %s SSH.VMPort = %v, want %v", name, gotVM.SSH.VMPort, wantVM.SSH.VMPort)
				}
				// Check SSH options: only upper-case keys should be present
				for k, v := range wantVM.SSH.Options {
					if gotVal, ok := gotVM.SSH.Options[k]; !ok || !reflect.DeepEqual(gotVal, v) {
						t.Errorf("VM %s SSH.Options[%q] = %v, want %v", name, k, gotVal, v)
					}
				}
				for k := range gotVM.SSH.Options {
					if len(k) > 0 && k[0] >= 'a' && k[0] <= 'z' {
						t.Errorf("VM %s SSH.Options should not contain lower-case key %q", name, k)
					}
				}
			}
			// Check global SSH options (should be present in got.SSH)
			if got.SSH == nil {
				t.Errorf("Global SSH config missing")
			} else {
				for _, key := range []string{"ServerAliveInterval", "ServerAliveCountMax", "UserKnownHostsFile", "StrictHostKeyChecking"} {
					if _, ok := got.SSH[key]; !ok {
						t.Errorf("Global SSH option %q missing", key)
					}
				}
			}
		})
	}
}

func TestResolveVM(t *testing.T) {
	// Create a temporary config file for testing
	tempDir := t.TempDir()
	testConfigFile := filepath.Join(tempDir, "test-config.toml")
	testConfigContent := `[qemu]
bin = "qemu-system-x86_64"
img = "qemu-img"

[vars]
home = "/home/user"

[vm.test-vm]
cmd = [
    "-nodefaults -machine q35,accel=kvm,kernel-irqchip=split",
    "-cpu host -smp 2 -m 4096",
    "-netdev user,id=net0,hostfwd=tcp::{{.vm.ssh_host}}-:{{.vm.ssh_vm}}",
    "-device virtio-net-pci,netdev=net0",
    "-drive id=boot,file={{.vm.boot_img}},format=qcow2,if=virtio",
]

[vm.test-vm.vars]
ssh_host = 2089
ssh_vm = 22
boot_img = "{{.home}}/path/to/disk.img"

[vm.test-vm.ssh]
port = 2089
vm_port = 22`

	err := os.WriteFile(testConfigFile, []byte(testConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	config := &Config{
		Vars: map[string]interface{}{
			"home": "/home/user",
		},
		VMs: map[string]VMConfig{
			"test-vm": {
				Cmd: []string{
					"-nodefaults -machine q35,accel=kvm,kernel-irqchip=split",
					"-cpu host -smp 2 -m 4096",
					"-netdev user,id=net0,hostfwd=tcp::{{.vm.ssh_host}}-:{{.vm.ssh_vm}}",
					"-device virtio-net-pci,netdev=net0",
					"-drive id=boot,file={{.vm.boot_img}},format=qcow2,if=virtio",
				},
				Vars: map[string]interface{}{
					"ssh_host": int64(2089),
					"ssh_vm":   int64(22),
					"boot_img": "{{.home}}/path/to/disk.img",
				},
			},
		},
	}

	tests := []struct {
		name       string
		vmName     string
		configPath string
		wantCmd    []string
		wantErr    bool
	}{
		{
			name:       "valid VM with template resolution",
			vmName:     "test-vm",
			configPath: testConfigFile,
			wantCmd: []string{
				"-nodefaults -machine q35,accel=kvm,kernel-irqchip=split",
				"-cpu host -smp 2 -m 4096",
				"-netdev user,id=net0,hostfwd=tcp::2089-:22",
				"-device virtio-net-pci,netdev=net0",
				"-drive id=boot,file=/home/user/path/to/disk.img,format=qcow2,if=virtio",
			},
			wantErr: false,
		},
		{
			name:       "VM not found",
			vmName:     "nonexistent-vm",
			configPath: testConfigFile,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.ResolveVM(tt.vmName, tt.configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveVM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Name != tt.vmName {
					t.Errorf("ResolveVM() name = %v, want %v", got.Name, tt.vmName)
				}
				if !reflect.DeepEqual(got.Cmd, tt.wantCmd) {
					t.Errorf("ResolveVM() cmd = %v, want %v", got.Cmd, tt.wantCmd)
				}
				expectedDataDir := filepath.Join(tempDir, "vm.test-vm")
				if got.DataDir != expectedDataDir {
					t.Errorf("ResolveVM() dataDir = %v, want %v", got.DataDir, expectedDataDir)
				}
			}
		})
	}
}

func TestListVMs(t *testing.T) {
	config := &Config{
		VMs: map[string]VMConfig{
			"vm1": {},
			"vm2": {},
			"vm3": {},
		},
	}

	got := config.ListVMs()
	expected := []string{"vm1", "vm2", "vm3"}

	// Sort both slices for comparison since map iteration order is not guaranteed
	if len(got) != len(expected) {
		t.Errorf("ListVMs() returned %d VMs, expected %d", len(got), len(expected))
	}

	// Check that all expected VMs are present
	for _, expectedVM := range expected {
		found := false
		for _, gotVM := range got {
			if gotVM == expectedVM {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ListVMs() missing expected VM: %s", expectedVM)
		}
	}
}

func TestVmEntryMethods(t *testing.T) {
	entry := &VmEntry{
		Name:    "test-vm",
		Cmd:     []string{"-nodefaults", "-machine q35"},
		Vars:    map[string]interface{}{"ssh_host": int64(2089)},
		DataDir: ".qqmgr/vm.test-vm",
	}

	// Get current working directory for absolute path construction
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{
			name:     "PidFilePath",
			method:   entry.PidFilePath,
			expected: filepath.Join(cwd, ".qqmgr", "vm.test-vm", "pid"),
		},
		{
			name:     "SerialFilePath",
			method:   entry.SerialFilePath,
			expected: filepath.Join(cwd, ".qqmgr", "vm.test-vm", "serial"),
		},
		{
			name:     "QmpSocketPath",
			method:   entry.QmpSocketPath,
			expected: filepath.Join(cwd, ".qqmgr", "vm.test-vm", "qmp.socket"),
		},
		{
			name:     "MonitorSocketPath",
			method:   entry.MonitorSocketPath,
			expected: filepath.Join(cwd, ".qqmgr", "vm.test-vm", "monitor.socket"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method()
			if got != tt.expected {
				t.Errorf("%s() = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestVmEntryGetAutoInjectedArgs(t *testing.T) {
	entry := &VmEntry{
		Name:    "test-vm",
		DataDir: ".qqmgr/vm.test-vm",
	}

	// Get current working directory for absolute path construction
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	args := entry.GetAutoInjectedArgs()
	expected := []string{
		fmt.Sprintf("-pidfile %s", filepath.Join(cwd, ".qqmgr", "vm.test-vm", "pid")),
		fmt.Sprintf("-monitor unix:%s,server,nowait", filepath.Join(cwd, ".qqmgr", "vm.test-vm", "monitor.socket")),
		fmt.Sprintf("-serial file:%s", filepath.Join(cwd, ".qqmgr", "vm.test-vm", "serial")),
		fmt.Sprintf("-qmp unix:%s,server,nowait", filepath.Join(cwd, ".qqmgr", "vm.test-vm", "qmp.socket")),
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("GetAutoInjectedArgs() = %v, want %v", args, expected)
	}
}

func TestVmEntryGetFullCommand(t *testing.T) {
	entry := &VmEntry{
		Name:    "test-vm",
		Cmd:     []string{"-nodefaults", "-machine q35"},
		DataDir: ".qqmgr/vm.test-vm",
	}

	// Get current working directory for absolute path construction
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	fullCmd := entry.GetFullCommand()
	expected := []string{
		"-nodefaults",
		"-machine q35",
		fmt.Sprintf("-pidfile %s", filepath.Join(cwd, ".qqmgr", "vm.test-vm", "pid")),
		fmt.Sprintf("-monitor unix:%s,server,nowait", filepath.Join(cwd, ".qqmgr", "vm.test-vm", "monitor.socket")),
		fmt.Sprintf("-serial file:%s", filepath.Join(cwd, ".qqmgr", "vm.test-vm", "serial")),
		fmt.Sprintf("-qmp unix:%s,server,nowait", filepath.Join(cwd, ".qqmgr", "vm.test-vm", "qmp.socket")),
	}

	if !reflect.DeepEqual(fullCmd, expected) {
		t.Errorf("GetFullCommand() = %v, want %v", fullCmd, expected)
	}
}

func TestSSHConfigValidation(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		wantErr  bool
		errorMsg string
	}{
		{
			name: "valid SSH config",
			content: `[qemu]
bin = "qemu-system-x86_64"

[vm.test-vm]
cmd = ["-nodefaults"]

[vm.test-vm.ssh]
port = 2089
vm_port = 22`,
			wantErr: false,
		},
		{
			name: "missing SSH port",
			content: `[qemu]
bin = "qemu-system-x86_64"

[vm.test-vm]
cmd = ["-nodefaults"]`,
			wantErr:  true,
			errorMsg: "VM 'test-vm' missing required SSH port configuration",
		},
		{
			name: "missing VM port (should default to 22)",
			content: `[qemu]
bin = "qemu-system-x86_64"

[vm.test-vm]
cmd = ["-nodefaults"]

[vm.test-vm.ssh]
port = 2089`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tempDir, "test.toml")
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			got, err := LoadFromFile(testFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadFromFile() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("LoadFromFile() error = %v, want to contain %v", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("LoadFromFile() unexpected error: %v", err)
					return
				}

				// Check that VM port defaults to 22 if not specified
				if vm, exists := got.VMs["test-vm"]; exists {
					if vm.SSH.VMPort != 22 {
						t.Errorf("Expected VM port to default to 22, got %d", vm.SSH.VMPort)
					}
				}
			}
		})
	}
}
