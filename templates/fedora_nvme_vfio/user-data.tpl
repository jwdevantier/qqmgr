#cloud-config
users:
  - name: root
    lock_passwd: false
    hashed_passwd: {{.root_password_hash}}
    ssh_authorized_keys:
      - {{.ssh_public_key}}

disable_root: false
ssh_pwauth: true

packages:
  - gcc
  - g++
  - perl
  - meson
  - ninja-build
  - lspci
  - vim

write_files:
  - path: /root/setup_libvfn.sh
    permissions: '0755'
    content: |
      #!/bin/bash
      set -ex
      cd /tmp
      tar xvzf libvfn.tgz
      cd libvfn-5.1.0
      meson setup build -Dlibnvme=disabled
      ninja -C build
      meson test -C build "libvfn:"
      meson install -C build
  - path: /etc/modules-load.d/vfio.conf
    content: |
      vfio
      vfio_iommu_type1
      vfio_pci
  - path: /etc/modprobe.d/blacklist-nvme.conf
    content: |
      blacklist nvme
  - path: /etc/dracut.conf.d/vfio.conf
    content: |
      add_drivers+=" vfio vfio_iommu_type1 vfio_pci "
  - path: /etc/ld.so.conf.d/lvfn.conf
    content: |
      /usr/local/lib64

runcmd:
  - mkdir -p /tmp/cidata
  - mount LABEL=cidata /tmp/cidata
  - cp /tmp/cidata/libvfn.tgz /tmp/
  - /root/setup_libvfn.sh
  - ldconfig
  - sed -i 's/\(GRUB_CMDLINE_LINUX_DEFAULT=".*\)"/\1 intel_iommu=on vfio-pci.ids=1b36:0010 modprobe.blacklist=nvme"/' /etc/default/grub
  - grub2-mkconfig -o /boot/grub2/grub.cfg
  - dracut -f --kver `uname -r`

power_state:
  mode: poweroff
  timeout: 300
  condition: True 