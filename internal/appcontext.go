// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"qqmgr/internal/config"
	"qqmgr/internal/img"
	"qqmgr/internal/trace"
)

// AppContext holds the configuration and runtime context for VM operations
type AppContext struct {
	Config     *config.Config
	ConfigPath string
	ImgManager *img.Manager
	Tracer     trace.Tracer
}

// NewAppContext creates a new AppContext with the given configuration and paths
func NewAppContext(cfg *config.Config, configPath string) (*AppContext, error) {
	// Get runtime directory
	runtimeDir, err := config.GetRuntimeDir(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to determine runtime directory: %w", err)
	}

	// Set up tracing
	var tracer trace.Tracer
	if traceEnv := os.Getenv("QQMGR_TRACE"); traceEnv != "" {
		// Create trace file in runtime directory
		tracePath := filepath.Join(runtimeDir, "trace.log")
		patterns := []string{traceEnv} // Use the env var as pattern

		tracer, err = trace.NewTraceLoggerWithFile(patterns, tracePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create tracer: %w", err)
		}
	} else {
		tracer = trace.NewNoOpTracer()
	}

	// Get config directory for image manager
	configDir := filepath.Dir(configPath)
	if configPath == "qqmgr.toml" {
		configDir = "."
	}

	// Create image manager
	imgManager := img.NewManager(configDir, runtimeDir, cfg.Qemu.Bin, cfg.Qemu.Img, tracer)

	return &AppContext{
		Config:     cfg,
		ConfigPath: configPath,
		ImgManager: imgManager,
		Tracer:     tracer,
	}, nil
}

// ResolveVM resolves template variables in VM configuration and returns a VmEntry
func (ctx *AppContext) ResolveVM(vmName string) (*config.VmEntry, error) {
	// Build image map for template resolution
	imgMap := make(map[string]interface{})
	if len(ctx.Config.Images) > 0 {
		// Get path for each image
		for imgName, imgConfig := range ctx.Config.Images {
			imgPath, err := ctx.ImgManager.GetImagePath(imgName, &imgConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve image path for '%s': %w", imgName, err)
			}
			imgMap[imgName] = imgPath
		}
	}

	// Call the Config's ResolveVM method with the image map
	return ctx.Config.ResolveVM(vmName, ctx.ConfigPath, imgMap)
}

// GetImagePath returns the path to a specific image
func (ctx *AppContext) GetImagePath(imgName string) (string, error) {
	imgConfig, err := ctx.Config.GetImage(imgName)
	if err != nil {
		return "", err
	}
	return ctx.ImgManager.GetImagePath(imgName, imgConfig)
}

// BuildImage builds a specific image
func (ctx *AppContext) BuildImage(imgName string) error {
	imgConfig, err := ctx.Config.GetImage(imgName)
	if err != nil {
		return err
	}
	return ctx.ImgManager.BuildImage(context.Background(), imgName, imgConfig)
}

func (ctx *AppContext) Close() {
	ctx.Tracer.Close()
}
