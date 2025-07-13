// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"encoding/json"
	"fmt"

	"qqmgr/internal/config"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured virtual machines",
	Long:  `List all virtual machines defined in the configuration file.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		if jsonOutput {
			// JSON output
			vms := cfg.ListVMs()
			result := make([]map[string]interface{}, len(vms))
			for i, name := range vms {
				result[i] = map[string]interface{}{
					"name":       name,
					"configured": true,
					"running":    false, // TODO: Check actual running status
				}
			}

			jsonData, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				fmt.Printf("Error marshaling JSON: %v\n", err)
				return
			}
			fmt.Println(string(jsonData))
		} else {
			// Human-readable output
			fmt.Println("Configured VMs:")
			vms := cfg.ListVMs()
			if len(vms) == 0 {
				fmt.Println("  No VMs configured")
			} else {
				for _, name := range vms {
					fmt.Printf("  %s\n", name)
				}
			}
		}
	},
}

func init() {
	listCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.AddCommand(listCmd)
}
