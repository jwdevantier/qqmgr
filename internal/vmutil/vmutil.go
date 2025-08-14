// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package vmutil

import (
	"os"
	"qqmgr/internal/config"
)

// DeleteLogFiles removes existing stdout/stderr log files for a VM
func DeleteLogFiles(vmEntry *config.VmEntry) {
	_ = os.Remove(vmEntry.QemuStdoutPath())
	_ = os.Remove(vmEntry.QemuStderrPath())
}
