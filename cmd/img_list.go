// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"encoding/json"
	"fmt"

	"qqmgr/internal/config"

	"github.com/spf13/cobra"
)

var imgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured images",
	Long:  `List all images defined in the configuration file.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		if jsonOutput {
			// JSON output
			images := cfg.ListImages()
			result := make([]map[string]interface{}, len(images))
			for i, name := range images {
				img, err := cfg.GetImage(name)
				if err != nil {
					continue
				}
				result[i] = map[string]interface{}{
					"name":     name,
					"builder":  img.Builder,
					"img_size": img.ImgSize,
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
			fmt.Println("Configured Images:")
			images := cfg.ListImages()
			if len(images) == 0 {
				fmt.Println("  No images configured")
			} else {
				for _, name := range images {
					img, err := cfg.GetImage(name)
					if err != nil {
						fmt.Printf("  %s (error: %v)\n", name, err)
						continue
					}
					fmt.Printf("  %s\t%s\t%s\n", name, img.Builder, img.ImgSize)
				}
			}
		}
	},
}

func init() {
	imgListCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	imgCmd.AddCommand(imgListCmd)
}
