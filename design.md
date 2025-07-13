SPDX-License-Identifier: CC0-1.0
SPDX-FileCopyrightText: NONE

# qqmgr - Quick QEMU Manager Design Document

## Overview

A CLI tool for managing QEMU virtual machines in development contexts. Provides simple commands to start, stop, and manage VMs defined in TOML configuration files.

## Core Features

- **VM Lifecycle**: start, stop, status
- **SSH/SCP convenience**: ssh, put, get commands  
- **JSON output**: All commands support `--json` flag for scripting
- **Template-based config**: Variable substitution in VM commands

## Configuration

### Config File Locations & Runtime Directories

| Config Location | Runtime Directory |
|-----------------------------|-----------------------------------|
| `./qqmgr.toml`              | `./.qqmgr/`                       |
| `~/.config/qqmgr/conf.toml` | `~/.config/qqmgr/`                |
| Custom path via `-c` flag   | Directory containing config file  |

### Config Structure

```toml
[qemu]
bin = "qemu-system-x86_64"
img = "qemu-img"

[vars]
home = "/home/user"
data_dir = "/data"

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
vm_port = 22
```

**Note**: SSH configuration is required for all VMs. The `port` field is mandatory, while `vm_port` defaults to 22 if not specified.

### SSH Configuration

SSH configuration is generated from a combination of global and VM-specific options:

```toml
[ssh]
ServerAliveInterval = 300
ServerAliveCountMax = 3
UserKnownHostsFile = "/dev/null"
StrictHostKeyChecking = "no"

[vm.test-vm.ssh]
port = 2089
vm_port = 22
ServerAliveInterval = 60  # Override global setting
```

The SSH config generation:
1. Starts with global `[ssh]` options
2. Merges VM-specific `[vm.name.ssh]` options (excluding `port` and `vm_port`)
3. Generates a temporary SSH config file for each VM connection
4. Uses the generated config for SSH/SCP operations

### Template Resolution

1. **Global variables**: `{{.home}}` from `[vars]` section
2. **Namespaced variables**: `{{.foo.bar}}` from `[vars.foo]` sections  
3. **VM variables**: `{{.vm.ssh_host}}` from `[vm.name.vars]` section

## Runtime Management

### VM Runtime Directory Structure

For VM named "test-vm":
```
$RUNTIME_DIR/vm.test-vm/
├── pid                 # Process ID
├── monitor.socket      # Monitor socket
├── serial              # Serial output file
└── qmp.socket          # QMP socket
```

### Auto-Injected QEMU Arguments

qqmgr automatically appends these arguments to resolved VM commands:

```bash
-pidfile $RUNTIME_DIR/vm.$VM_NAME/pid
-monitor unix:$RUNTIME_DIR/vm.$VM_NAME/monitor.socket,server,nowait
-serial file:$RUNTIME_DIR/vm.$VM_NAME/serial
-qmp unix:$RUNTIME_DIR/vm.$VM_NAME/qmp.socket,server,nowait
```

## CLI Commands

### Basic Operations

```bash
qqmgr start <vm-name>     # Start VM
qqmgr stop <vm-name>      # Graceful stop, kill after timeout
qqmgr status <vm-name>    # Show running status, ports, sockets
qqmgr list               # List configured VMs
```

### SSH/SCP Operations

```bash
qqmgr ssh <vm-name>                    # Interactive SSH
qqmgr ssh <vm-name> "command"          # Run command via SSH
qqmgr put <vm-name> <local> <remote>   # Copy file to VM
qqmgr get <vm-name> <remote> <local>   # Copy file from VM
```

### Global Flags

```bash
qqmgr -c <config-file> <command>       # Use custom configuration file
```

### JSON Output

All commands support `--json` flag for machine-readable output:

```bash
qqmgr status vm-name --json
# {
#   "name": "vm-name",
#   "pid": null,
#   "pid_file": "/absolute/path/to/vm.vm-name/pid",
#   "ssh": {
#     "port": 2089
#   },
#   "serial_file": "/absolute/path/to/vm.vm-name/serial",
#   "qmp_file": "/absolute/path/to/vm.vm-name/qmp.socket",
#   "monitor_file": "/absolute/path/to/vm.vm-name/monitor.socket"
# }
```

## Implementation Notes

### Port Management
- SSH ports manually assigned in VM config via `[vm.name.ssh]` section
- Port validation occurs during config loading
- TODO: Consider auto-allocation with conflict detection

### Error Handling
- Graceful handling of stale PID files
- Socket cleanup on VM termination
- Config validation on load (SSH configuration required)

### Dependencies
- `github.com/BurntSushi/toml` v1.5.0 for config parsing
- `github.com/spf13/cobra` v1.9.1 for CLI framework
- Go 1.23.10

## Open Questions

1. **Stop timeout**: Configure by flag, default to 20 seconds
2. **SSH authentication**: Generate SSH config from Go template using global and VM-specific options
3. **Cleanup strategy**: Remove runtime files on stop or leave for debugging?
4. **Port conflicts**: Handle multiple VMs wanting same SSH port?
5. **Config validation**: Validate QEMU args, required fields, etc.?
