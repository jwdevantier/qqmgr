// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"github.com/spf13/cobra"
)

var imgCmd = &cobra.Command{
	Use:   "img",
	Short: "Manage VM images",
	Long:  `Manage VM images including building and listing configured images.`,
}

func init() {
	rootCmd.AddCommand(imgCmd)
}
