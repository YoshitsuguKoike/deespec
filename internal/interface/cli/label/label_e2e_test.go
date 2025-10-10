package label

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/app/config"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/label"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/di"
)

// TestLabelE2E tests the complete label workflow
func TestLabelE2E(t *testing.T) {
	// Create temporary database for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Initialize container with test database
	labelConfig := config.LabelConfig{
		TemplateDirs: []string{tmpDir},
		Import: config.LabelImportConfig{
			AutoPrefixFromDir: true,
			MaxLineCount:      1000,
			ExcludePatterns:   []string{"*.secret.md"},
		},
		Validation: config.LabelValidationConfig{
			AutoSyncOnMismatch: false,
			WarnOnLargeFiles:   true,
		},
	}

	container, err := di.NewContainer(di.Config{
		DBPath:      dbPath,
		AgentType:   "gemini-cli", // Use mock agent for tests
		LabelConfig: labelConfig,
	})
	if err != nil {
		t.Fatalf("Failed to initialize container: %v", err)
	}
	defer container.Close()

	labelRepo := container.GetLabelRepository()
	ctx := context.Background()

	// Test 1: Register a new label
	t.Run("Register", func(t *testing.T) {
		// Create a temporary template file
		templatePath := "test-template.md" // Relative path
		fullPath := filepath.Join(tmpDir, templatePath)
		templateContent := "# Test Template\nThis is a test template for label."
		if err := os.WriteFile(fullPath, []byte(templateContent), 0644); err != nil {
			t.Fatalf("Failed to create template file: %v", err)
		}

		// Create label with relative path
		lbl := label.NewLabel("test-label", "Test label description", []string{templatePath}, 5)

		// Save to repository
		if err := labelRepo.Save(ctx, lbl); err != nil {
			t.Fatalf("Failed to save label: %v", err)
		}

		// Verify label was saved
		if lbl.ID() == 0 {
			t.Error("Expected non-zero label ID after save")
		}
	})

	// Test 2: Find label by name
	t.Run("FindByName", func(t *testing.T) {
		lbl, err := labelRepo.FindByName(ctx, "test-label")
		if err != nil {
			t.Fatalf("Failed to find label: %v", err)
		}

		if lbl.Name() != "test-label" {
			t.Errorf("Expected name 'test-label', got '%s'", lbl.Name())
		}

		if lbl.Description() != "Test label description" {
			t.Errorf("Expected description 'Test label description', got '%s'", lbl.Description())
		}

		if lbl.Priority() != 5 {
			t.Errorf("Expected priority 5, got %d", lbl.Priority())
		}
	})

	// Test 3: List active labels
	t.Run("ListActive", func(t *testing.T) {
		labels, err := labelRepo.FindActive(ctx)
		if err != nil {
			t.Fatalf("Failed to list labels: %v", err)
		}

		if len(labels) != 1 {
			t.Errorf("Expected 1 active label, got %d", len(labels))
		}

		if labels[0].Name() != "test-label" {
			t.Errorf("Expected first label to be 'test-label', got '%s'", labels[0].Name())
		}
	})

	// Test 4: Update label
	t.Run("Update", func(t *testing.T) {
		lbl, err := labelRepo.FindByName(ctx, "test-label")
		if err != nil {
			t.Fatalf("Failed to find label: %v", err)
		}

		lbl.SetDescription("Updated description")
		lbl.SetPriority(10)

		if err := labelRepo.Update(ctx, lbl); err != nil {
			t.Fatalf("Failed to update label: %v", err)
		}

		// Verify update
		updated, err := labelRepo.FindByName(ctx, "test-label")
		if err != nil {
			t.Fatalf("Failed to find updated label: %v", err)
		}

		if updated.Description() != "Updated description" {
			t.Errorf("Expected description 'Updated description', got '%s'", updated.Description())
		}

		if updated.Priority() != 10 {
			t.Errorf("Expected priority 10, got %d", updated.Priority())
		}
	})

	// Test 5: Validate label integrity
	t.Run("ValidateIntegrity", func(t *testing.T) {
		lbl, err := labelRepo.FindByName(ctx, "test-label")
		if err != nil {
			t.Fatalf("Failed to find label: %v", err)
		}

		// Sync from file first
		if err := labelRepo.SyncFromFile(ctx, lbl.ID()); err != nil {
			t.Fatalf("Failed to sync from file: %v", err)
		}

		// Validate
		result, err := labelRepo.ValidateIntegrity(ctx, lbl.ID())
		if err != nil {
			t.Fatalf("Failed to validate integrity: %v", err)
		}

		if result.Status != "OK" {
			t.Errorf("Expected status OK, got %s", result.Status)
		}
	})

	// Test 6: Deactivate label
	t.Run("Deactivate", func(t *testing.T) {
		lbl, err := labelRepo.FindByName(ctx, "test-label")
		if err != nil {
			t.Fatalf("Failed to find label: %v", err)
		}

		lbl.Deactivate()

		if err := labelRepo.Update(ctx, lbl); err != nil {
			t.Fatalf("Failed to update label: %v", err)
		}

		// Verify it's not in active list
		active, err := labelRepo.FindActive(ctx)
		if err != nil {
			t.Fatalf("Failed to list active labels: %v", err)
		}

		if len(active) != 0 {
			t.Errorf("Expected 0 active labels, got %d", len(active))
		}

		// But should be in all labels
		all, err := labelRepo.FindAll(ctx)
		if err != nil {
			t.Fatalf("Failed to list all labels: %v", err)
		}

		if len(all) != 1 {
			t.Errorf("Expected 1 total label, got %d", len(all))
		}
	})

	// Test 7: Delete label
	t.Run("Delete", func(t *testing.T) {
		lbl, err := labelRepo.FindByName(ctx, "test-label")
		if err != nil {
			t.Fatalf("Failed to find label: %v", err)
		}

		if err := labelRepo.Delete(ctx, lbl.ID()); err != nil {
			t.Fatalf("Failed to delete label: %v", err)
		}

		// Verify deletion
		_, err = labelRepo.FindByName(ctx, "test-label")
		if err == nil {
			t.Error("Expected error when finding deleted label, got nil")
		}
	})
}

// TestEnrichTaskWithLabelsE2E tests the label enrichment in prompts
// Commented out: uses private functions (NewClaudeCodePromptBuilder, contains)
/*
func TestEnrichTaskWithLabelsE2E(t *testing.T) {
	// Create temporary directory and files
	tmpDir := t.TempDir()

	// Create a label template file
	labelFile := filepath.Join(tmpDir, "security.md")
	labelContent := `# Security Guidelines
- Always validate user input
- Use parameterized queries
- Implement proper authentication`

	if err := os.WriteFile(labelFile, []byte(labelContent), 0644); err != nil {
		t.Fatalf("Failed to create label file: %v", err)
	}

	// Create meta.yml with labels
	metaDir := filepath.Join(tmpDir, ".deespec", "SBI-001")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatalf("Failed to create meta directory: %v", err)
	}

	metaContent := `labels:
  - security
`
	metaFile := filepath.Join(metaDir, "meta.yml")
	if err := os.WriteFile(metaFile, []byte(metaContent), 0644); err != nil {
		t.Fatalf("Failed to create meta file: %v", err)
	}

	// Create label directory
	labelDir := filepath.Join(tmpDir, ".deespec", "prompts", "labels")
	if err := os.MkdirAll(labelDir, 0755); err != nil {
		t.Fatalf("Failed to create label directory: %v", err)
	}

	// Copy label file to label directory
	targetLabelFile := filepath.Join(labelDir, "security.md")
	if err := os.WriteFile(targetLabelFile, []byte(labelContent), 0644); err != nil {
		t.Fatalf("Failed to create target label file: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Test enrichment
	builder := NewClaudeCodePromptBuilder(
		tmpDir,
		filepath.Join(tmpDir, ".deespec", "specs", "sbi", "SBI-001"),
		"SBI-001",
		1,
		"implement",
		nil, // Test fallback mode
	)

	taskDesc := "Implement user registration feature"
	enriched := builder.EnrichTaskWithLabels(taskDesc, "SBI-001")

	// Verify enrichment
	if !contains(enriched, "Implement user registration feature") {
		t.Error("Enriched description should contain original task description")
	}

	if !contains(enriched, "Label-Specific Guidelines") || !contains(enriched, "security") {
		t.Error("Enriched description should contain label-specific guidelines")
	}

	if !contains(enriched, "Security Guidelines") {
		t.Error("Enriched description should contain security guidelines content")
	}
}
*/
