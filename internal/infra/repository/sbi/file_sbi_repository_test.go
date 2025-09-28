package sbi_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/sbi"
	infraSbi "github.com/YoshitsuguKoike/deespec/internal/infra/repository/sbi"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

func TestFileSBIRepository_Save(t *testing.T) {
	tests := []struct {
		name      string
		sbi       *sbi.SBI
		wantPath  string
		wantErr   bool
		checkFile func(t *testing.T, fs afero.Fs, path string)
	}{
		{
			name: "Save SBI with guideline and body",
			sbi: &sbi.SBI{
				ID:    "SBI-01J8X5YNFZ4TQ5H5N5RQNT5P78",
				Title: "Test Specification",
				Body: `## ガイドライン

このドキュメントは、チームで共有される仕様書です。

# Test Specification

This is the test body content.`,
			},
			wantPath: ".deespec/specs/sbi/SBI-01J8X5YNFZ4TQ5H5N5RQNT5P78/spec.md",
			wantErr:  false,
			checkFile: func(t *testing.T, fs afero.Fs, specPath string) {
				content, err := afero.ReadFile(fs, specPath)
				if err != nil {
					t.Errorf("Failed to read saved file: %v", err)
					return
				}

				contentStr := string(content)
				if !strings.Contains(contentStr, "## ガイドライン") {
					t.Error("Saved content should contain guideline header")
				}
				if !strings.Contains(contentStr, "# Test Specification") {
					t.Error("Saved content should contain title")
				}
				if !strings.Contains(contentStr, "This is the test body content.") {
					t.Error("Saved content should contain body text")
				}
			},
		},
		{
			name: "Save SBI with empty body",
			sbi: &sbi.SBI{
				ID:    "SBI-01J8X5YNFZ4TQ5H5N5RQNT5P79",
				Title: "Empty Body Spec",
				Body:  "",
			},
			wantPath: ".deespec/specs/sbi/SBI-01J8X5YNFZ4TQ5H5N5RQNT5P79/spec.md",
			wantErr:  false,
			checkFile: func(t *testing.T, fs afero.Fs, specPath string) {
				content, err := afero.ReadFile(fs, specPath)
				if err != nil {
					t.Errorf("Failed to read saved file: %v", err)
					return
				}

				if len(content) != 0 {
					t.Errorf("Expected empty file for empty body, got %d bytes", len(content))
				}
			},
		},
		{
			name: "Save SBI with special characters in body",
			sbi: &sbi.SBI{
				ID:    "SBI-01J8X5YNFZ4TQ5H5N5RQNT5P80",
				Title: "Special Chars",
				Body:  "Content with 日本語 and émojis 🎉 and special chars: <>&\"'",
			},
			wantPath: ".deespec/specs/sbi/SBI-01J8X5YNFZ4TQ5H5N5RQNT5P80/spec.md",
			wantErr:  false,
			checkFile: func(t *testing.T, fs afero.Fs, specPath string) {
				content, err := afero.ReadFile(fs, specPath)
				if err != nil {
					t.Errorf("Failed to read saved file: %v", err)
					return
				}

				contentStr := string(content)
				if contentStr != "Content with 日本語 and émojis 🎉 and special chars: <>&\"'" {
					t.Errorf("Special characters not preserved correctly")
				}
			},
		},
		{
			name: "Overwrite existing SBI",
			sbi: &sbi.SBI{
				ID:    "SBI-01J8X5YNFZ4TQ5H5N5RQNT5P81",
				Title: "Updated Spec",
				Body:  "New content",
			},
			wantPath: ".deespec/specs/sbi/SBI-01J8X5YNFZ4TQ5H5N5RQNT5P81/spec.md",
			wantErr:  false,
			checkFile: func(t *testing.T, fs afero.Fs, specPath string) {
				content, err := afero.ReadFile(fs, specPath)
				if err != nil {
					t.Errorf("Failed to read saved file: %v", err)
					return
				}

				if string(content) != "New content" {
					t.Errorf("File should be overwritten with new content")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create memory filesystem for testing
			fs := afero.NewMemMapFs()

			// For overwrite test, create existing file
			if tt.name == "Overwrite existing SBI" {
				existingPath := filepath.Join(".deespec/specs/sbi", tt.sbi.ID, "spec.md")
				fs.MkdirAll(filepath.Dir(existingPath), 0o755)
				afero.WriteFile(fs, existingPath, []byte("Old content"), 0o644)
			}

			// Create repository with test filesystem
			repo := infraSbi.NewFileSBIRepository(fs)

			// Execute save
			gotPath, err := repo.Save(context.Background(), tt.sbi)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Check returned path
				if gotPath != tt.wantPath {
					t.Errorf("Save() gotPath = %v, want %v", gotPath, tt.wantPath)
				}

				// Check file was created
				exists, err := afero.Exists(fs, gotPath)
				if err != nil {
					t.Errorf("Failed to check file existence: %v", err)
				}
				if !exists {
					t.Errorf("File was not created at %s", gotPath)
				}

				// Run custom file checks
				if tt.checkFile != nil {
					tt.checkFile(t, fs, gotPath)
				}

				// Check directory structure
				expectedDir := filepath.Join(".deespec/specs/sbi", tt.sbi.ID)
				dirInfo, err := fs.Stat(expectedDir)
				if err != nil {
					t.Errorf("Directory not created: %v", err)
				}
				if !dirInfo.IsDir() {
					t.Error("Expected directory but got file")
				}

				// Check meta.yml exists and has correct content
				metaPath := filepath.Join(expectedDir, "meta.yml")
				metaExists, err := afero.Exists(fs, metaPath)
				if err != nil {
					t.Errorf("Failed to check meta.yml existence: %v", err)
				}
				if !metaExists {
					t.Errorf("meta.yml was not created at %s", metaPath)
				} else {
					// Read and verify meta.yml content
					metaContent, err := afero.ReadFile(fs, metaPath)
					if err != nil {
						t.Errorf("Failed to read meta.yml: %v", err)
					}

					var meta sbi.Meta
					if err := yaml.Unmarshal(metaContent, &meta); err != nil {
						t.Errorf("Failed to parse meta.yml: %v", err)
					}

					// Verify meta fields
					if meta.ID != tt.sbi.ID {
						t.Errorf("Meta ID mismatch: got %s, want %s", meta.ID, tt.sbi.ID)
					}
					if meta.Title != tt.sbi.Title {
						t.Errorf("Meta Title mismatch: got %s, want %s", meta.Title, tt.sbi.Title)
					}
					if meta.Labels == nil {
						t.Error("Meta Labels should not be nil")
					}
					if len(meta.Labels) != 0 {
						t.Errorf("Meta Labels should be empty by default, got %v", meta.Labels)
					}
					if meta.CreatedAt.IsZero() {
						t.Error("Meta CreatedAt should not be zero")
					}
					if meta.UpdatedAt.IsZero() {
						t.Error("Meta UpdatedAt should not be zero")
					}
				}
			}
		})
	}
}

func TestFileSBIRepository_Save_ConcurrentWrites(t *testing.T) {
	// Test that concurrent saves to different SBIs work correctly
	fs := afero.NewMemMapFs()
	repo := infraSbi.NewFileSBIRepository(fs)

	// Create multiple SBIs
	sbis := []*sbi.SBI{
		{
			ID:    "SBI-CONCURRENT1",
			Title: "Spec 1",
			Body:  "Content 1",
		},
		{
			ID:    "SBI-CONCURRENT2",
			Title: "Spec 2",
			Body:  "Content 2",
		},
		{
			ID:    "SBI-CONCURRENT3",
			Title: "Spec 3",
			Body:  "Content 3",
		},
	}

	// Save all concurrently
	type result struct {
		path string
		err  error
		sbi  *sbi.SBI
	}

	results := make(chan result, len(sbis))

	for _, s := range sbis {
		go func(sbiEntity *sbi.SBI) {
			path, err := repo.Save(context.Background(), sbiEntity)
			results <- result{path: path, err: err, sbi: sbiEntity}
		}(s)
	}

	// Collect results
	for i := 0; i < len(sbis); i++ {
		res := <-results
		if res.err != nil {
			t.Errorf("Concurrent save failed for %s: %v", res.sbi.ID, res.err)
			continue
		}

		// Verify file content
		content, err := afero.ReadFile(fs, res.path)
		if err != nil {
			t.Errorf("Failed to read file for %s: %v", res.sbi.ID, err)
			continue
		}

		if string(content) != res.sbi.Body {
			t.Errorf("Content mismatch for %s: got %q, want %q",
				res.sbi.ID, string(content), res.sbi.Body)
		}
	}
}
