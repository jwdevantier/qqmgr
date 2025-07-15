// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package img

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"qqmgr/internal/trace"
)

// ImageBuilder defines the interface for image builders
type ImageBuilder interface {
	Build(ctx context.Context) error
	GetImagePath() string
	GetStateDir() string
	GetManifest() (map[string]string, error) // Returns input hashes for caching
}

// BaseImageBuilder provides common functionality for image builders
type BaseImageBuilder struct {
	config   *ImageConfig
	stateDir string
	qemuBin  string
	qemuImg  string
	tracer   trace.Tracer
}

// NewBaseImageBuilder creates a new base image builder
func NewBaseImageBuilder(config *ImageConfig, stateDir, qemuBin, qemuImg string, tracer trace.Tracer) *BaseImageBuilder {
	return &BaseImageBuilder{
		config:   config,
		stateDir: stateDir,
		qemuBin:  qemuBin,
		qemuImg:  qemuImg,
		tracer:   tracer,
	}
}

// initStateDir resolves stateDir to an absolute path and ensures it exists
func (b *BaseImageBuilder) initStateDir() error {
	absPath, err := filepath.Abs(b.stateDir)
	if err != nil {
		return err
	}
	b.stateDir = absPath
	if err := os.MkdirAll(b.stateDir, 0755); err != nil {
		return err
	}
	return nil
}

// GetStateDir returns the state directory for this image
func (b *BaseImageBuilder) GetStateDir() string {
	return b.stateDir
}

// ensureStateDir ensures the state directory exists and is absolute
func (b *BaseImageBuilder) ensureStateDir() error {
	return b.initStateDir()
}

// getManifestPath returns the path to the manifest file
func (b *BaseImageBuilder) getManifestPath() string {
	return filepath.Join(b.stateDir, "manifest.json")
}

// loadManifest loads the stored manifest from disk
func (b *BaseImageBuilder) loadManifest() (map[string]string, error) {
	manifestPath := b.getManifestPath()
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, nil // No manifest exists yet
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest map[string]string
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return manifest, nil
}

// saveManifest saves the manifest to disk
func (b *BaseImageBuilder) saveManifest(manifest map[string]string) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	manifestPath := b.getManifestPath()
	return os.WriteFile(manifestPath, data, 0644)
}

// manifestChanged checks if the current manifest differs from the stored one
func (b *BaseImageBuilder) manifestChanged(currentManifest map[string]string) (bool, error) {
	storedManifest, err := b.loadManifest()
	if err != nil {
		return true, err // Consider changed if we can't load stored manifest
	}

	if storedManifest == nil {
		return true, nil // No stored manifest, consider changed
	}

	// Compare manifests
	if len(currentManifest) != len(storedManifest) {
		return true, nil
	}

	for k, v := range currentManifest {
		if storedManifest[k] != v {
			return true, nil
		}
	}

	return false, nil
}
