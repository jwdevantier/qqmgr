// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package img

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"qqmgr/internal/downloader"
	"qqmgr/internal/trace"
)

// CloudInitImageBuilder creates cloud-init images
type CloudInitImageBuilder struct {
	*BaseImageBuilder
	downloader        *downloader.Downloader
	templateProcessor *TemplateProcessor
	envHookExecutor   *EnvHookExecutor
}

// NewCloudInitImageBuilder creates a new cloud-init image builder
func NewCloudInitImageBuilder(
	config *ImageConfig,
	stateDir, qemuBin, qemuImg string,
	downloader *downloader.Downloader,
	templateProcessor *TemplateProcessor,
	tracer trace.Tracer,
) *CloudInitImageBuilder {
	return &CloudInitImageBuilder{
		BaseImageBuilder:  NewBaseImageBuilder(config, stateDir, qemuBin, qemuImg, tracer),
		downloader:        downloader,
		templateProcessor: templateProcessor,
		envHookExecutor:   NewEnvHookExecutor(),
	}
}

// Build creates a cloud-init image through the multi-stage process
func (c *CloudInitImageBuilder) Build(ctx context.Context) error {
	c.tracer.Trace("cloud-init", "Starting cloud-init image build", "stateDir", c.stateDir)

	if err := c.ensureStateDir(); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Stage 1: Download base image
	c.tracer.Trace("cloud-init", "Stage 1: Downloading base image")
	if err := c.downloadBaseImage(); err != nil {
		return fmt.Errorf("failed to download base image: %w", err)
	}

	// Stage 2: Prepare base image (resize and create overlay)
	c.tracer.Trace("cloud-init", "Stage 2: Preparing base image")
	if err := c.prepareBaseImage(); err != nil {
		return fmt.Errorf("failed to prepare base image: %w", err)
	}

	// Stage 3: Generate cloud-init files
	c.tracer.Trace("cloud-init", "Stage 3: Generating cloud-init files")
	if err := c.generateCloudInitFiles(); err != nil {
		return fmt.Errorf("failed to generate cloud-init files: %w", err)
	}

	// Stage 4: Create cloud-init ISO
	c.tracer.Trace("cloud-init", "Stage 4: Creating cloud-init ISO")
	if err := c.createCloudInitISO(); err != nil {
		return fmt.Errorf("failed to create cloud-init ISO: %w", err)
	}

	// Stage 5: Run VM for customization
	c.tracer.Trace("cloud-init", "Stage 5: Running VM for customization")
	if err := c.runVMForCustomization(); err != nil {
		return fmt.Errorf("failed to run VM for customization: %w", err)
	}

	c.tracer.Trace("cloud-init", "Cloud-init image build completed successfully")
	return nil
}

// GetImagePath returns the path to the final image
func (c *CloudInitImageBuilder) GetImagePath() string {
	return filepath.Join(c.stateDir, "stage3.img")
}

// GetManifest returns the current manifest for this image
func (c *CloudInitImageBuilder) GetManifest() (map[string]string, error) {
	return c.calculateManifest()
}

// downloadBaseImage downloads the base image if needed
func (c *CloudInitImageBuilder) downloadBaseImage() error {
	if c.config.BaseImg == nil {
		return fmt.Errorf("no base image configured")
	}

	c.tracer.Trace("download", "Checking base image download", "url", c.config.BaseImg.URL, "sha256", c.config.BaseImg.SHA256Sum)

	manifestPath := filepath.Join(c.stateDir, "stage1.img.checksum")

	// Check if we need to download
	if _, err := os.Stat(manifestPath); err == nil {
		// Check if checksum matches
		data, err := os.ReadFile(manifestPath)
		if err == nil && strings.TrimSpace(string(data)) == c.config.BaseImg.SHA256Sum {
			// Already downloaded and checksum matches
			c.tracer.Trace("download", "Base image already downloaded and checksum matches")
			return nil
		}
	}

	// Download the base image
	c.tracer.Trace("download", "Downloading base image", "url", c.config.BaseImg.URL)
	downloadedPath, err := c.downloader.Download(c.config.BaseImg.URL, c.config.BaseImg.SHA256Sum)
	if err != nil {
		return fmt.Errorf("failed to download base image: %w", err)
	}

	// Copy to stage1.img
	stage1Path := filepath.Join(c.stateDir, "stage1.img")
	c.tracer.Trace("download", "Copying downloaded image to stage1", "from", downloadedPath, "to", stage1Path)
	if err := c.copyFile(downloadedPath, stage1Path); err != nil {
		return fmt.Errorf("failed to copy downloaded image: %w", err)
	}

	// Save checksum
	if err := os.WriteFile(manifestPath, []byte(c.config.BaseImg.SHA256Sum), 0644); err != nil {
		return fmt.Errorf("failed to save checksum: %w", err)
	}

	c.tracer.Trace("download", "Base image download completed", "path", stage1Path)
	return nil
}

// prepareBaseImage prepares the base image (resize and create overlay)
func (c *CloudInitImageBuilder) prepareBaseImage() error {
	c.tracer.Trace("prepare", "Preparing base image", "targetSize", c.config.ImgSize)

	stage1Path := filepath.Join(c.stateDir, "stage1.img")
	stage2Path := filepath.Join(c.stateDir, "stage2.img")
	stage3Path := filepath.Join(c.stateDir, "stage3.img")

	// Calculate manifest for this stage
	manifest := map[string]string{
		"base_img_hash": c.config.BaseImg.SHA256Sum,
		"img_size":      c.config.ImgSize,
	}

	// Check if we need to rebuild
	manifestPath := filepath.Join(c.stateDir, "stage2.manifest.json")
	if c.manifestMatches(manifestPath, manifest) {
		c.tracer.Trace("prepare", "Base image preparation is up to date, skipping")
		return nil
	}

	// Copy stage1 to stage2
	c.tracer.Trace("prepare", "Copying stage1 to stage2", "from", stage1Path, "to", stage2Path)
	if err := c.copyFile(stage1Path, stage2Path); err != nil {
		return fmt.Errorf("failed to copy stage1 to stage2: %w", err)
	}

	// Resize stage2
	c.tracer.Trace("prepare", "Resizing stage2 image", "path", stage2Path, "size", c.config.ImgSize)
	if err := c.resizeImage(stage2Path, c.config.ImgSize); err != nil {
		return fmt.Errorf("failed to resize image: %w", err)
	}

	// Create overlay (stage3)
	c.tracer.Trace("prepare", "Creating overlay (stage3)", "base", stage2Path, "overlay", stage3Path)
	if err := c.createOverlay(stage2Path, stage3Path); err != nil {
		return fmt.Errorf("failed to create overlay: %w", err)
	}

	// Save manifest
	if err := c.saveStageManifest(manifestPath, manifest); err != nil {
		return fmt.Errorf("failed to save stage2 manifest: %w", err)
	}

	c.tracer.Trace("prepare", "Base image preparation completed successfully")
	return nil
}

// generateCloudInitFiles generates cloud-init files from templates
func (c *CloudInitImageBuilder) generateCloudInitFiles() error {
	if len(c.config.Templates) == 0 {
		c.tracer.Trace("templates", "No templates configured, skipping")
		return nil
	}

	c.tracer.Trace("templates", "Generating cloud-init files", "templateCount", len(c.config.Templates))

	// Execute environment hook if present
	env := c.config.Env
	if c.config.EnvHook != nil {
		c.tracer.Trace("templates", "Executing environment hook", "script", c.config.EnvHook.Script)
		configDir := c.templateProcessor.configDir // FIX: use configDir, not stateDir
		processedEnv, err := c.envHookExecutor.Execute(c.config.EnvHook, configDir, env)
		if err != nil {
			return fmt.Errorf("failed to execute environment hook: %w", err)
		}
		env = processedEnv
		c.tracer.Trace("templates", "Environment hook completed", "envKeys", len(env))
	}

	// Calculate template manifest
	templateManifest, err := c.templateProcessor.CalculateTemplateHashes(c.config.Templates, env)
	if err != nil {
		return fmt.Errorf("failed to calculate template manifest: %w", err)
	}

	// Check if we need to rebuild
	manifestPath := filepath.Join(c.stateDir, "templates.manifest.json")
	if c.manifestMatches(manifestPath, templateManifest) {
		c.tracer.Trace("templates", "Templates are up to date, skipping generation")
		return nil
	}

	// Process templates
	c.tracer.Trace("templates", "Processing templates", "outputDir", c.stateDir)
	if err := c.templateProcessor.ProcessTemplates(c.config.Templates, env, c.stateDir); err != nil {
		return fmt.Errorf("failed to process templates: %w", err)
	}

	// Save manifest
	if err := c.saveStageManifest(manifestPath, templateManifest); err != nil {
		return fmt.Errorf("failed to save template manifest: %w", err)
	}

	c.tracer.Trace("templates", "Template generation completed successfully")
	return nil
}

// createCloudInitISO creates the cloud-init ISO
func (c *CloudInitImageBuilder) createCloudInitISO() error {
	isoPath := filepath.Join(c.stateDir, "cloud-init.iso")

	// Calculate manifest for this stage
	manifest := make(map[string]string)

	// Add template file hashes
	for _, tmpl := range c.config.Templates {
		outputPath := filepath.Join(c.stateDir, tmpl.Output)
		if hash, err := c.calculateFileHash(outputPath); err == nil {
			manifest[tmpl.Output] = hash
		}
	}

	// Download and prepare additional sources
	if err := c.prepareAdditionalSources(); err != nil {
		return fmt.Errorf("failed to prepare additional sources: %w", err)
	}

	// Add additional sources to manifest
	for _, source := range c.config.Sources {
		manifest[source.Filename] = source.SHA256Sum
	}

	// Check if we need to rebuild
	manifestPath := filepath.Join(c.stateDir, "cloud-init.iso.manifest.json")
	if c.manifestMatches(manifestPath, manifest) {
		return nil
	}

	// Create ISO using genisoimage
	if err := c.createISO(isoPath, manifest); err != nil {
		return fmt.Errorf("failed to create ISO: %w", err)
	}

	// Save manifest
	if err := c.saveStageManifest(manifestPath, manifest); err != nil {
		return fmt.Errorf("failed to save ISO manifest: %w", err)
	}

	return nil
}

// runVMForCustomization runs the VM for image customization
func (c *CloudInitImageBuilder) runVMForCustomization() error {
	fmt.Printf("DEBUG: runVMForCustomization() called\n")
	c.tracer.Trace("vm", "Starting VM customization stage", "buildArgsCount", len(c.config.BuildArgs), "buildArgs", c.config.BuildArgs)

	if len(c.config.BuildArgs) == 0 {
		fmt.Printf("DEBUG: No build args found, skipping VM execution\n")
		c.tracer.Trace("vm", "No build args configured, skipping VM execution")
		return nil
	}

	// Calculate manifest for this stage
	manifest := map[string]string{
		"build_args": c.calculateBuildArgsHash(),
	}
	fmt.Printf("DEBUG: Calculated build args hash: %s\n", manifest["build_args"])

	// Add ISO hash
	isoPath := filepath.Join(c.stateDir, "cloud-init.iso")
	fmt.Printf("DEBUG: Checking ISO at: %s\n", isoPath)
	if hash, err := c.calculateFileHash(isoPath); err == nil {
		manifest["cloud_init_iso"] = hash
		fmt.Printf("DEBUG: ISO hash: %s\n", hash)
	} else {
		fmt.Printf("DEBUG: Failed to calculate ISO hash: %v\n", err)
	}

	c.tracer.Trace("vm", "Calculated VM manifest", "manifest", manifest)
	fmt.Printf("DEBUG: Full manifest: %+v\n", manifest)

	// Check if we need to rebuild
	manifestPath := filepath.Join(c.stateDir, "vm.manifest.json")
	fmt.Printf("DEBUG: Checking manifest at: %s\n", manifestPath)
	if c.manifestMatches(manifestPath, manifest) {
		fmt.Printf("DEBUG: Manifest matches, skipping VM execution\n")
		c.tracer.Trace("vm", "VM manifest matches, skipping VM execution")
		return nil
	}

	fmt.Printf("DEBUG: Manifest does not match, running QEMU\n")
	c.tracer.Trace("vm", "VM manifest does not match, running QEMU")

	// Run QEMU
	if err := c.runQEMU(); err != nil {
		fmt.Printf("DEBUG: QEMU failed: %v\n", err)
		return fmt.Errorf("failed to run QEMU: %w", err)
	}

	// Save manifest
	fmt.Printf("DEBUG: Saving manifest to: %s\n", manifestPath)
	if err := c.saveStageManifest(manifestPath, manifest); err != nil {
		fmt.Printf("DEBUG: Failed to save manifest: %v\n", err)
		return fmt.Errorf("failed to save VM manifest: %w", err)
	}

	fmt.Printf("DEBUG: VM customization completed successfully\n")
	c.tracer.Trace("vm", "VM customization completed successfully")
	return nil
}

// Helper methods

func (c *CloudInitImageBuilder) copyFile(src, dst string) error {
	c.tracer.Trace("file", "Copying file", "from", src, "to", dst)
	cmd := exec.Command("cp", src, dst)
	if err := cmd.Run(); err != nil {
		c.tracer.Trace("file", "File copy failed", "error", err.Error())
		return err
	}
	c.tracer.Trace("file", "File copy completed")
	return nil
}

func (c *CloudInitImageBuilder) resizeImage(imagePath, size string) error {
	c.tracer.Trace("qemu-img", "Resizing image", "path", imagePath, "size", size)
	cmd := exec.Command(c.qemuImg, "resize", imagePath, size)
	if err := cmd.Run(); err != nil {
		c.tracer.Trace("qemu-img", "Image resize failed", "error", err.Error())
		return err
	}
	c.tracer.Trace("qemu-img", "Image resize completed")
	return nil
}

func (c *CloudInitImageBuilder) createOverlay(basePath, overlayPath string) error {
	c.tracer.Trace("qemu-img", "Creating overlay", "base", basePath, "overlay", overlayPath)
	cmd := exec.Command(c.qemuImg, "create", "-f", "qcow2", "-F", "qcow2", "-b", basePath, overlayPath)
	if err := cmd.Run(); err != nil {
		c.tracer.Trace("qemu-img", "Overlay creation failed", "error", err.Error())
		return err
	}
	c.tracer.Trace("qemu-img", "Overlay creation completed")
	return nil
}

// prepareAdditionalSources downloads additional sources (no copying needed)
func (c *CloudInitImageBuilder) prepareAdditionalSources() error {
	if len(c.config.Sources) == 0 {
		c.tracer.Trace("sources", "No additional sources configured, skipping")
		return nil
	}

	c.tracer.Trace("sources", "Preparing additional sources", "sourceCount", len(c.config.Sources))

	for _, source := range c.config.Sources {
		c.tracer.Trace("sources", "Downloading source", "filename", source.Filename, "url", source.URL)
		// Download the source file (this ensures it's in the cache)
		_, err := c.downloader.Download(source.URL, source.SHA256Sum)
		if err != nil {
			return fmt.Errorf("failed to download source %s: %w", source.Filename, err)
		}
		c.tracer.Trace("sources", "Source downloaded successfully", "filename", source.Filename)
	}

	c.tracer.Trace("sources", "All additional sources prepared successfully")
	return nil
}

func (c *CloudInitImageBuilder) createISO(isoPath string, manifest map[string]string) error {
	c.tracer.Trace("iso", "Creating cloud-init ISO", "output", isoPath)

	// Build genisoimage command
	args := []string{
		"-output", isoPath,
		"-volid", "cidata",
		"-joliet",
		"-input-charset", "utf-8",
		"-graft-points",
	}

	// Add template files from state directory
	for filename := range manifest {
		if filename != "cloud_init_iso" { // Skip the ISO itself
			// Check if this is a template file (exists in state directory)
			stateFilePath := filepath.Join(c.stateDir, filename)
			if _, err := os.Stat(stateFilePath); err == nil {
				// Template file exists in state directory
				args = append(args, fmt.Sprintf("%s=%s", filename, stateFilePath))
				c.tracer.Trace("iso", "Adding template file to ISO", "filename", filename, "path", stateFilePath)
			} else {
				// This might be a source file - check if it's in our sources config
				for _, source := range c.config.Sources {
					if source.Filename == filename {
						// Use the cached file directly
						cachedPath := c.downloader.GetCachedPath(source.SHA256Sum)
						args = append(args, fmt.Sprintf("%s=%s", filename, cachedPath))
						c.tracer.Trace("iso", "Adding source file to ISO", "filename", filename, "path", cachedPath)
						break
					}
				}
			}
		}
	}

	// Check if we have any files to add
	if len(args) <= 5 { // Only the base args, no files
		return fmt.Errorf("no files found to add to ISO")
	}

	c.tracer.Trace("iso", "Running genisoimage", "args", args)

	cmd := exec.Command("genisoimage", args...)

	// Capture stderr for debugging
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		c.tracer.Trace("iso", "genisoimage failed", "error", err.Error(), "stderr", stderr.String())
		return fmt.Errorf("genisoimage failed: %w, stderr: %s", err, stderr.String())
	}

	c.tracer.Trace("iso", "Cloud-init ISO created successfully", "size", "check")
	return nil
}

func (c *CloudInitImageBuilder) runQEMU() error {
	fmt.Printf("DEBUG: runQEMU() called\n")
	c.tracer.Trace("qemu", "Starting QEMU VM for customization")

	// Build the full environment for template rendering
	env := c.config.Env
	fmt.Printf("DEBUG: Initial env = %+v\n", env)

	if c.config.EnvHook != nil {
		fmt.Printf("DEBUG: Executing env hook\n")
		configDir := c.templateProcessor.configDir // FIX: use configDir, not stateDir
		processedEnv, err := c.envHookExecutor.Execute(c.config.EnvHook, configDir, env)
		if err != nil {
			fmt.Printf("DEBUG: Env hook failed: %v\n", err)
			return fmt.Errorf("failed to execute environment hook: %w", err)
		}
		env = processedEnv
		fmt.Printf("DEBUG: Processed env = %+v\n", env)
	}

	// Add build-specific variables to environment
	env["img_self"] = c.GetImagePath()
	env["cloud_init_iso"] = filepath.Join(c.stateDir, "cloud-init.iso")
	fmt.Printf("DEBUG: Final env = %+v\n", env)

	// Render build_args as Go templates
	args := make([]string, len(c.config.BuildArgs))
	fmt.Printf("DEBUG: Rendering %d build args\n", len(c.config.BuildArgs))
	for i, arg := range c.config.BuildArgs {
		fmt.Printf("DEBUG: Processing build arg %d: %s\n", i, arg)
		// Create a template from the argument string
		tmpl, err := template.New(fmt.Sprintf("build_arg_%d", i)).Parse(arg)
		if err != nil {
			fmt.Printf("DEBUG: Failed to parse template %d: %v\n", i, err)
			return fmt.Errorf("failed to parse build arg template %d: %w", i, err)
		}

		// Execute template with environment
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, env); err != nil {
			fmt.Printf("DEBUG: Failed to execute template %d: %v\n", i, err)
			return fmt.Errorf("failed to execute build arg template %d: %w", i, err)
		}

		args[i] = buf.String()
		fmt.Printf("DEBUG: Rendered arg %d: %s\n", i, args[i])
	}

	fmt.Printf("DEBUG: Final QEMU command: %s %v\n", c.qemuBin, args)

	// Print exact command for manual testing
	cmdStr := c.qemuBin
	for _, arg := range args {
		cmdStr += " " + arg
	}
	fmt.Printf("EXACT QEMU COMMAND: %s\n", cmdStr)
	fmt.Printf("WORKING DIR: %s\n", c.stateDir)

	c.tracer.Trace("qemu", "QEMU command", "binary", c.qemuBin, "args", args, "workingDir", c.stateDir)

	cmd := exec.Command(c.qemuBin, args...)
	cmd.Dir = c.stateDir
	fmt.Printf("DEBUG: QEMU working directory: %s\n", cmd.Dir)

	// Let QEMU write directly to stdout/stderr for better output handling
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the command
	fmt.Printf("DEBUG: Starting QEMU process...\n")
	if err := cmd.Start(); err != nil {
		fmt.Printf("DEBUG: Failed to start QEMU: %v\n", err)
		return fmt.Errorf("failed to start QEMU: %w", err)
	}

	fmt.Printf("DEBUG: QEMU process started with PID: %d\n", cmd.Process.Pid)
	c.tracer.Trace("qemu", "QEMU process started", "pid", cmd.Process.Pid)
	fmt.Printf("QEMU VM started (PID: %d). Waiting for boot and cloud-init completion...\n", cmd.Process.Pid)

	// Create channel for process completion
	doneCh := make(chan error, 1)

	// Wait for process completion
	go func() {
		fmt.Printf("DEBUG: Starting process wait goroutine\n")
		err := cmd.Wait()
		fmt.Printf("DEBUG: Process wait returned: %v\n", err)
		doneCh <- err
	}()

	// Wait for completion or timeout
	fmt.Printf("DEBUG: Waiting for QEMU completion or timeout...\n")
	select {
	case err := <-doneCh:
		fmt.Printf("DEBUG: QEMU process completed with error: %v\n", err)
		if err != nil {
			c.tracer.Trace("qemu", "QEMU process failed", "error", err.Error())
			return fmt.Errorf("QEMU process failed: %w", err)
		}
	case <-time.After(10 * time.Minute): // 10 minute timeout for VM boot and shutdown
		fmt.Printf("DEBUG: QEMU process timed out, killing\n")
		c.tracer.Trace("qemu", "QEMU process timed out, killing")
		cmd.Process.Kill()
		return fmt.Errorf("QEMU process timed out after 10 minutes")
	}

	fmt.Printf("DEBUG: QEMU process completed successfully\n")
	fmt.Printf("QEMU VM completed successfully.\n")

	c.tracer.Trace("qemu", "QEMU process completed successfully")
	return nil
}

func (c *CloudInitImageBuilder) calculateFileHash(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

func (c *CloudInitImageBuilder) calculateBuildArgsHash() string {
	// Build the full environment for hash calculation
	env := c.config.Env
	if c.config.EnvHook != nil {
		configDir := c.templateProcessor.configDir // FIX: use configDir, not stateDir
		if processedEnv, err := c.envHookExecutor.Execute(c.config.EnvHook, configDir, env); err == nil {
			env = processedEnv
		}
	}

	// Add build-specific variables to environment
	env["img_self"] = c.GetImagePath()
	env["cloud_init_iso"] = filepath.Join(c.stateDir, "cloud-init.iso")

	// Create a combined hash of build args and environment
	buildArgsData := strings.Join(c.config.BuildArgs, "|")
	envData, _ := json.Marshal(env)

	combinedData := buildArgsData + "|" + string(envData)
	hash := sha256.Sum256([]byte(combinedData))
	return fmt.Sprintf("%x", hash)
}

func (c *CloudInitImageBuilder) manifestMatches(manifestPath string, currentManifest map[string]string) bool {
	if _, err := os.Stat(manifestPath); err != nil {
		return false
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}

	var storedManifest map[string]string
	if err := json.Unmarshal(data, &storedManifest); err != nil {
		return false
	}

	if len(currentManifest) != len(storedManifest) {
		return false
	}

	for k, v := range currentManifest {
		if storedManifest[k] != v {
			return false
		}
	}

	return true
}

func (c *CloudInitImageBuilder) saveStageManifest(manifestPath string, manifest map[string]string) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, data, 0644)
}

func (c *CloudInitImageBuilder) calculateManifest() (map[string]string, error) {
	// This would calculate the overall manifest for the entire build
	// For now, return a simple manifest
	return map[string]string{
		"builder": "cloud-init",
		"version": "1.0",
	}, nil
}
