// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package cmd

import (
	"fmt"

	"qqmgr/internal"
	"qqmgr/internal/config"

	"github.com/spf13/cobra"
)

var imgBuildCmd = &cobra.Command{
	Use:   "build [image-name]",
	Short: "Build a VM image",
	Long:  `Build a VM image using the specified builder.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		imgName := args[0]

		// Load configuration
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		// Create AppContext
		appCtx, err := internal.NewAppContext(cfg, configFile)
		if err != nil {
			fmt.Printf("Error creating app context: %v\n", err)
			return
		}
		defer appCtx.Close()

		// Build the image
		fmt.Printf("Building image '%s'...\n", imgName)
		if err := appCtx.BuildImage(imgName); err != nil {
			fmt.Printf("Error building image: %v\n", err)
			return
		}

		// Get the image path
		imagePath, err := appCtx.GetImagePath(imgName)
		if err != nil {
			fmt.Printf("Error getting image path: %v\n", err)
			return
		}

		fmt.Printf("Image built successfully: %s\n", imagePath)
	},
}

func init() {
	imgCmd.AddCommand(imgBuildCmd)
}
