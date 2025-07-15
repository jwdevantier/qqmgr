// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package img

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// TemplateProcessor handles template processing
type TemplateProcessor struct {
	configDir string
}

// NewTemplateProcessor creates a new template processor
func NewTemplateProcessor(configDir string) *TemplateProcessor {
	return &TemplateProcessor{
		configDir: configDir,
	}
}

// ProcessTemplates processes all templates and writes them to the output directory
func (t *TemplateProcessor) ProcessTemplates(
	templates []TemplateConfig,
	env map[string]interface{},
	outputDir string,
) error {
	for _, tmplConfig := range templates {
		if err := t.processTemplate(tmplConfig, env, outputDir); err != nil {
			return fmt.Errorf("failed to process template %s: %w", tmplConfig.Template, err)
		}
	}
	return nil
}

// processTemplate processes a single template
func (t *TemplateProcessor) processTemplate(
	tmplConfig TemplateConfig,
	env map[string]interface{},
	outputDir string,
) error {
	// Load template from file
	tmpl, err := t.loadTemplate(tmplConfig.Template)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, env); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Write to output file
	outputPath := filepath.Join(outputDir, tmplConfig.Output)
	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// loadTemplate loads a template from a file relative to the config directory
func (t *TemplateProcessor) loadTemplate(templatePath string) (*template.Template, error) {
	fullPath := filepath.Join(t.configDir, templatePath)
	return template.ParseFiles(fullPath)
}

// CalculateTemplateHashes calculates hashes of template files and environment for caching
func (t *TemplateProcessor) CalculateTemplateHashes(
	templates []TemplateConfig,
	env map[string]interface{},
) (map[string]string, error) {
	manifest := make(map[string]string)

	// Calculate hash of environment
	envData, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal environment: %w", err)
	}
	envHash := sha256.Sum256(envData)
	manifest["env"] = fmt.Sprintf("%x", envHash)

	// Calculate hashes of template files
	for _, tmplConfig := range templates {
		fullPath := filepath.Join(t.configDir, tmplConfig.Template)

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read template %s: %w", tmplConfig.Template, err)
		}

		hash := sha256.Sum256(data)
		manifest[tmplConfig.Template] = fmt.Sprintf("%x", hash)
	}

	return manifest, nil
}
