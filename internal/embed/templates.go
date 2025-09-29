package embed

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed templates/etc/* templates/etc/policies/* templates/prompts/* templates/var/* templates/specs/* templates/templates/*
var templatesFS embed.FS

// Template represents a template file to be written
type Template struct {
	Path    string
	Content []byte
	Mode    os.FileMode
}

// GetTemplates returns all templates to be written during init
func GetTemplates() ([]Template, error) {
	var templates []Template

	// Walk through all embedded files
	err := fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the templates root directory itself
		if path == "templates" {
			return nil
		}

		// Skip directories (we'll create them when writing files)
		if d.IsDir() {
			return nil
		}

		// Read file content
		content, err := templatesFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Remove "templates/" prefix and ".tmpl" suffix for the destination path
		destPath := strings.TrimPrefix(path, "templates/")
		destPath = strings.TrimSuffix(destPath, ".tmpl")

		// Special handling for state.json - inject current timestamp
		if strings.HasSuffix(destPath, "state.json") {
			contentStr := string(content)
			contentStr = fmt.Sprintf(contentStr, time.Now().UTC().Format(time.RFC3339))
			content = []byte(contentStr)
		}

		templates = append(templates, Template{
			Path:    destPath,
			Content: content,
			Mode:    0644,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return templates, nil
}

// WriteTemplateResult represents the result of writing a template
type WriteTemplateResult struct {
	Path   string
	Action string // "WROTE", "SKIP", "WROTE (force)"
}

// WriteTemplate writes a template file atomically and returns the action taken
func WriteTemplate(baseDir string, tmpl Template, force bool) (*WriteTemplateResult, error) {
	fullPath := filepath.Join(baseDir, tmpl.Path)
	result := &WriteTemplateResult{Path: tmpl.Path}

	// Create directory if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Check if file exists (unless force is true)
	exists := false
	if _, err := os.Stat(fullPath); err == nil {
		exists = true
	}

	if exists && !force {
		// File exists, skip
		result.Action = "SKIP"
		return result, nil
	}

	// Write to temp file first for atomic write
	tmpFile := fullPath + ".tmp"
	if err := os.WriteFile(tmpFile, tmpl.Content, tmpl.Mode); err != nil {
		return nil, fmt.Errorf("failed to write temp file %s: %w", tmpFile, err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, fullPath); err != nil {
		os.Remove(tmpFile) // Clean up temp file
		return nil, fmt.Errorf("failed to rename %s to %s: %w", tmpFile, fullPath, err)
	}

	if force && exists {
		result.Action = "WROTE (force)"
	} else {
		result.Action = "WROTE"
	}
	return result, nil
}
