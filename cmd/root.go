// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configFile string
	debugFlag  bool
)

var rootCmd = &cobra.Command{
	Use:   "qqmgr",
	Short: "Quick QEMU Manager - A CLI tool for managing QEMU virtual machines",
	Long: `qqmgr is a CLI tool for managing QEMU virtual machines in development contexts.
It provides simple commands to start, stop, and manage VMs defined in configuration files.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Configuration file path (default: ./qqmgr.toml or ~/.config/qqmgr/conf.toml)")
	rootCmd.PersistentFlags().BoolVarP(&debugFlag, "debug", "d", false, "Enable debug output")
}
