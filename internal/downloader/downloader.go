// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package downloader

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Downloader handles downloading files with checksum verification and global caching
type Downloader struct {
	cacheDir string // Global cache directory shared across all images
}

// NewDownloader creates a new downloader with the specified cache directory
func NewDownloader(cacheDir string) *Downloader {
	return &Downloader{
		cacheDir: cacheDir,
	}
}

// GetCachedPath returns the path where a file with the given checksum should be cached
func (d *Downloader) GetCachedPath(sha256sum string) string {
	return filepath.Join(d.cacheDir, sha256sum)
}

// IsCached checks if a file exists in the global cache and has the matching checksum
func (d *Downloader) IsCached(sha256sum string) bool {
	cachedPath := d.GetCachedPath(sha256sum)
	if _, err := os.Stat(cachedPath); err != nil {
		return false
	}

	actualHash, err := calculateFileChecksum(cachedPath)
	if err != nil {
		return false
	}

	return actualHash == sha256sum
}

// Download downloads a file from the given URL and verifies its checksum
func (d *Downloader) Download(url, expectedSHA256 string) (string, error) {
	// Check if file already exists in global cache
	if d.IsCached(expectedSHA256) {
		return d.GetCachedPath(expectedSHA256), nil
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Download to temporary file first
	tempPath := d.GetCachedPath(expectedSHA256) + ".tmp"

	// Download the file
	if err := d.downloadFile(url, tempPath); err != nil {
		// Clean up temp file on error
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to download %s: %w", url, err)
	}

	// Verify checksum
	actualHash, err := calculateFileChecksum(tempPath)
	if err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if actualHash != expectedSHA256 {
		os.Remove(tempPath)
		return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", url, expectedSHA256, actualHash)
	}

	// Move to final location
	finalPath := d.GetCachedPath(expectedSHA256)
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to move downloaded file: %w", err)
	}

	return finalPath, nil
}

// downloadFile downloads a file from URL to the specified path
func (d *Downloader) downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// calculateFileChecksum calculates the SHA256 checksum of a file
func calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
