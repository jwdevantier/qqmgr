// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package img

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os/exec"
	"path/filepath"
	"qqmgr/internal/trace"
)

// RawImageBuilder creates raw disk images
type RawImageBuilder struct {
	*BaseImageBuilder
}

// NewRawImageBuilder creates a new raw image builder
func NewRawImageBuilder(config *ImageConfig, stateDir, qemuBin, qemuImg string, tracer trace.Tracer) *RawImageBuilder {
	return &RawImageBuilder{
		BaseImageBuilder: NewBaseImageBuilder(config, stateDir, qemuBin, qemuImg, tracer),
	}
}

// Build creates a raw image using qemu-img
func (r *RawImageBuilder) Build(ctx context.Context) error {
	if err := r.ensureStateDir(); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Calculate manifest for this build
	manifest, err := r.calculateManifest()
	if err != nil {
		return fmt.Errorf("failed to calculate manifest: %w", err)
	}

	// Check if we need to rebuild
	changed, err := r.manifestChanged(manifest)
	if err != nil {
		return fmt.Errorf("failed to check manifest: %w", err)
	}

	if !changed {
		// Image is up to date
		return nil
	}

	// Create the raw image
	if err := r.createRawImage(); err != nil {
		return fmt.Errorf("failed to create raw image: %w", err)
	}

	// Save the manifest
	if err := r.saveManifest(manifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	return nil
}

// GetImagePath returns the path to the created image
func (r *RawImageBuilder) GetImagePath() string {
	return filepath.Join(r.stateDir, "image.img")
}

// GetManifest returns the current manifest for this image
func (r *RawImageBuilder) GetManifest() (map[string]string, error) {
	return r.calculateManifest()
}

// calculateManifest calculates the manifest for this raw image build
func (r *RawImageBuilder) calculateManifest() (map[string]string, error) {
	// For raw images, the manifest includes:
	// - Image size
	// - Builder type and version
	// - qemu-img version (if available)

	manifest := map[string]string{
		"img_size": r.config.ImgSize,
		"builder":  "raw",
		"version":  "1.0", // Could be made configurable
	}

	// Try to get qemu-img version for more precise caching
	if r.qemuImg != "" {
		cmd := exec.Command(r.qemuImg, "--version")
		if output, err := cmd.Output(); err == nil {
			// Hash the version string
			hash := sha256.Sum256(output)
			manifest["qemu_img_version"] = fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes
		}
	}

	return manifest, nil
}

// createRawImage creates the raw image using qemu-img
func (r *RawImageBuilder) createRawImage() error {
	imagePath := r.GetImagePath()

	cmd := exec.Command(r.qemuImg, "create", "-f", "raw", imagePath, r.config.ImgSize)
	// Don't set cmd.Dir since we're using absolute paths

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img failed: %s, %w", string(output), err)
	}

	return nil
}
