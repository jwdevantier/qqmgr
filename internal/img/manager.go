// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package img

import (
	"context"
	"fmt"
	"path/filepath"

	"qqmgr/internal/downloader"
	"qqmgr/internal/trace"
)

// Manager handles image building operations
type Manager struct {
	configDir  string
	runtimeDir string
	qemuBin    string
	qemuImg    string
	downloader *downloader.Downloader
	tracer     trace.Tracer
}

// NewManager creates a new image manager
func NewManager(configDir, runtimeDir, qemuBin, qemuImg string, tracer trace.Tracer) *Manager {
	downloadCacheDir := filepath.Join(runtimeDir, "download_cache")
	return &Manager{
		configDir:  configDir,
		runtimeDir: runtimeDir,
		qemuBin:    qemuBin,
		qemuImg:    qemuImg,
		downloader: downloader.NewDownloader(downloadCacheDir),
		tracer:     tracer,
	}
}

// CreateBuilder creates an appropriate image builder based on the configuration
func (m *Manager) CreateBuilder(config *ImageConfig, imgName string) (ImageBuilder, error) {
	// Determine state directory
	stateDir := filepath.Join(m.runtimeDir, "img."+imgName)

	switch config.Builder {
	case "raw":
		return NewRawImageBuilder(config, stateDir, m.qemuBin, m.qemuImg, m.tracer), nil
	case "cloud-init":
		templateProcessor := NewTemplateProcessor(m.configDir)
		return NewCloudInitImageBuilder(config, stateDir, m.qemuBin, m.qemuImg, m.downloader, templateProcessor, m.tracer), nil
	default:
		return nil, fmt.Errorf("unknown builder type: %s", config.Builder)
	}
}

// BuildImage builds a specific image
func (m *Manager) BuildImage(ctx context.Context, imgName string, config *ImageConfig) error {
	builder, err := m.CreateBuilder(config, imgName)
	if err != nil {
		return fmt.Errorf("failed to create builder: %w", err)
	}

	return builder.Build(ctx)
}

// GetImagePath returns the path to a built image
func (m *Manager) GetImagePath(imgName string, config *ImageConfig) (string, error) {
	builder, err := m.CreateBuilder(config, imgName)
	if err != nil {
		return "", fmt.Errorf("failed to create builder: %w", err)
	}

	return builder.GetImagePath(), nil
}
