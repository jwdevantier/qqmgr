# qqmgr - Quick QEMU (VM) manager

qqmgr is written to help with:

1. managing (and reusing) VM configurations
2. managing and building VM images (raw/empty images and cloud-init customized images)
3. starting-, stopping and querying the status of VM's
   * supports scripting through optional JSON output
4. communicating with VM's
   * Supports `ssh`, `get` and `put` commands
   * `ssh` uses connection caching, making scripting with this feature faster
5. Convenient debugging of QEMU itself code
   * `qemu` command is a wrapper for launching a preconfigured instance of `gdb`
   * will automatically be configured with the QEMU binary and arguments from the VM configuration
   * further arguments to GDB can be passed by adding `-- <gdb args>`

This tool is basically written in reaction to the difficulty of remembering all the various command flags I wanted to pass when launching QEMU
and the difficulty of managing several almost, but not quite, same configurations.

Furthermore, this tool provides ways of configuring VM images using cloud-init. It can download (and cache) base images and packages
and generate cloud-init files from Go template files.

## Commands Overview

### VM Management
- `qqmgr start <vm-name>` - Start a configured VM
- `qqmgr stop <vm-name>` - Stop a running VM  
- `qqmgr list` - List configured VMs
- `qqmgr status <vm-name>` - Show VM status (supports JSON output)

### VM Communication
- `qqmgr ssh <vm-name> [command]` - SSH into VM (with connection caching)
- `qqmgr put <vm-name> <local-path> <remote-path>` - Upload files
- `qqmgr get <vm-name> <remote-path> <local-path>` - Download files

### VM Monitoring
- `qqmgr serial <vm-name>` - Connect to VM serial console
- `qqmgr stdout <vm-name>` - Monitor QEMU stdout
- `qqmgr stderr <vm-name>` - Monitor QEMU stderr

### Image Management
- `qqmgr img list` - List available images
- `qqmgr img build <image-name>` - Build VM images

### QEMU Debugging
- `qqmgr gdb <vm-name> [-- gdb-args]` - Debug QEMU with GDB

## SSH Configuration
Any keys in the `[ssh]` section inserted directly into the SSH configuration file generated for a given VM.
Note that config keys are the exact same as used in `~/.ssh/config`.

**NOTE:** Be sure to include `IdentityFile = "<path to ssh key>"` for passwordless login,
parts of `qqmgr` relies on it.

## VM Configuration

qqmgr uses TOML configuration files to define VMs with a template system for managing complex QEMU arguments.

### Basic VM Definition

```toml
[vm.myvm]
cmd = [
    "-machine q35,accel=kvm",
    "-cpu host -smp 2 -m 4096",
    "-netdev user,id=net0,hostfwd=tcp::{{.vm.ssh.port}}-:{{.vm.ssh.vm_port}}",
    "-drive id=boot,file=/path/to/image.qcow2,format=qcow2,if=virtio"
]

[vm.myvm.ssh]
port = 2222        # Required for SSH commands
vm_port = 22       # Optional, defaults to 22
```

### Global Variables

Define reusable variables in `[vars]`:

```toml
[vars]
q35_base = "-machine q35,accel=kvm -device virtio-rng-pci"
imgs_dir = "/data/images"

[vm.test]
cmd = [
    "{{.q35_base}}",                                    # Reference global vars
    "-drive id=boot,file={{.imgs_dir}}/test.qcow2,format=qcow2,if=virtio"
]
```

### VM-Specific Variables

```toml
# put VM-specific vars in [vm.<vm name>.vars] section
[vm.test.vars]
custom_device = "-device nvme,serial=test123"
# ... additional variables

[vm.test]
cmd = [
    "{{.q35_base}}",
    "{{.vm.custom_device}}"     # Reference with .vm prefix
]
```

### Special Variables

- `{{.vm.ssh.port}}`
    - required, HOST port to map VM SSH port to
    - from `[vm.<vm-name>.ssh].port` in config
- `{{.vm.ssh.vm_port}}`
    - optional, defaults to port 22
    - from `[vm.<vm-name>.ssh].vm_port` in config

- `{{.img.image-name}}` - Path to the image defined by `[img.<image name>]`
    - `{{index .img "<image-name>"}}` - if image name uses dashes or similar characters

## Image Building

### Raw Images
```toml
[img.test-disk]
builder = "raw" 
img_size = "1G"
```

### Cloud-Init Images
```toml
[img.fedora]
builder = "cloud-init"
img_size = "10G"
build_args = ["-m", "2048", "-smp", "2", "..."]

[img.fedora.base_img]
url = "https://example.com/fedora.qcow2"
sha256sum = "abc123..."

[[img.fedora.templates]]
template = "templates/user-data.tpl"
output = "user-data"

[img.fedora.env]
hostname = "test-vm"
# ... template variables
```

### Advanced Features
- `env_hook` - Dynamic variable generation via scripts
- `sources` - Include additional files in cloud-init ISO
- Template system with Go template syntax

## Debugging QEMU

qqmgr provides integrated GDB support for debugging QEMU itself during development:

```bash
# Launch VM with GDB attached to QEMU process
qqmgr gdb myvm

# Pass additional GDB arguments
qqmgr gdb myvm -- -ex "break nvme_admin_cmd" -ex "run"
```

The GDB integration automatically:
- Uses the QEMU binary from your config
- Applies the VM's QEMU arguments  
- Sets up the debugging environment

This makes it seamless to debug QEMU features while testing them with your configured VMs.
