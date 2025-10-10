package integrated

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/validator/common"
)

func TestIntegratedValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	basePath := filepath.Join(tmpDir, ".deespec")
	if err := os.MkdirAll(filepath.Join(basePath, "etc"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(basePath, "var"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(basePath, "prompts", "system"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid test files
	workflowContent := `steps:
  - id: plan
    agent: claude_cli
    prompt_path: prompts/system/plan.md
  - id: test
    agent: system
    prompt_path: prompts/system/test.md`

	stateContent := `{
  "version": 1,
  "step": "plan",
  "turn": 1,
  "meta.updated_at": "2025-01-26T10:00:00.000000000Z"
}`

	healthContent := `{
  "ts": "2025-01-26T10:00:00.000000000Z",
  "turn": 1,
  "step": "plan",
  "ok": true,
  "error": ""
}`

	journalContent := `{"ts":"2025-01-26T10:00:00.000000000Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":0,"error":"","artifacts":[]}
{"ts":"2025-01-26T10:00:10.000000000Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":10000,"error":"","artifacts":[]}`

	// Write test files
	workflowPath := filepath.Join(basePath, "etc", "workflow.yaml")
	statePath := filepath.Join(basePath, "var", "state.json")
	healthPath := filepath.Join(basePath, "var", "health.json")
	journalPath := filepath.Join(basePath, "var", "journal.ndjson")

	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(stateContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(healthPath, []byte(healthContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath, []byte(journalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create prompt files
	for _, f := range []string{"plan.md", "test.md"} {
		path := filepath.Join(basePath, "prompts", "system", f)
		if err := os.WriteFile(path, []byte("dummy prompt"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Run integrated validation
	config := &DoctorConfig{
		BasePath:    basePath,
		StatePath:   statePath,
		HealthPath:  healthPath,
		JournalPath: journalPath,
	}

	report, err := RunIntegratedValidation(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify report structure
	if report.Version != 1 {
		t.Errorf("expected version 1, got %d", report.Version)
	}

	if len(report.Components) != 3 {
		t.Errorf("expected 3 components, got %d", len(report.Components))
	}

	// Check each component exists
	for _, name := range []string{"state", "health", "journal"} {
		if _, ok := report.Components[name]; !ok {
			t.Errorf("missing component: %s", name)
		}
	}

	// Check summary
	if report.Summary.Components != 3 {
		t.Errorf("expected 3 components in summary, got %d", report.Summary.Components)
	}

	// All should be valid, so errors should be 0
	if report.Summary.Error != 0 {
		t.Errorf("expected 0 errors, got %d", report.Summary.Error)
		// Debug output
		data, _ := json.MarshalIndent(report, "", "  ")
		t.Logf("Report: %s", data)
	}

	// Test summary consistency
	if err := ValidateSummaryConsistency(report); err != nil {
		t.Errorf("summary consistency check failed: %v", err)
	}
}

func TestIntegratedValidationWithErrors(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	basePath := filepath.Join(tmpDir, ".deespec")
	if err := os.MkdirAll(filepath.Join(basePath, "etc"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(basePath, "var"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create invalid test files
	workflowContent := `steps:
  - id: plan
    agent: invalid_agent
    prompt_path: /etc/passwd`

	stateContent := `{
  "version": 1,
  "step": "invalid_step",
  "turn": "not_a_number"
}`

	healthContent := `invalid json`

	journalContent := `not valid ndjson
{invalid json}`

	// Write test files
	workflowPath := filepath.Join(basePath, "etc", "workflow.yaml")
	statePath := filepath.Join(basePath, "var", "state.json")
	healthPath := filepath.Join(basePath, "var", "health.json")
	journalPath := filepath.Join(basePath, "var", "journal.ndjson")

	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(stateContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(healthPath, []byte(healthContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath, []byte(journalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run integrated validation
	config := &DoctorConfig{
		BasePath:    basePath,
		StatePath:   statePath,
		HealthPath:  healthPath,
		JournalPath: journalPath,
	}

	report, err := RunIntegratedValidation(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have errors
	if report.Summary.Error == 0 {
		t.Error("expected errors but got none")
		// Debug output
		data, _ := json.MarshalIndent(report, "", "  ")
		t.Logf("Report: %s", data)
	}

	// At least state and health should have errors
	if state := report.Components["state"]; state != nil {
		if state.Summary.Error == 0 {
			t.Error("expected state to have errors")
		}
	}

	if health := report.Components["health"]; health != nil {
		if health.Summary.Error == 0 {
			t.Error("expected health to have errors")
		}
	}
}

func TestComponentStatus(t *testing.T) {
	report := &IntegratedReport{
		Components: map[string]*common.ValidationResult{
			"state": {
				Summary: common.Summary{Warn: 1},
			},
			"health": {
				Summary: common.Summary{OK: 1},
			},
			"journal": {
				Summary: common.Summary{Error: 2},
			},
		},
	}

	status := GetComponentStatus(report)

	if status.State != "warn" {
		t.Errorf("expected state status 'warn', got %s", status.State)
	}

	if status.Health != "ok" {
		t.Errorf("expected health status 'ok', got %s", status.Health)
	}

	if status.Journal != "error" {
		t.Errorf("expected journal status 'error', got %s", status.Journal)
	}
}

func TestJSONMarshal(t *testing.T) {
	report := NewIntegratedReport()
	report.Components["state"] = common.NewValidationResult()
	report.Components["health"] = common.NewValidationResult()
	report.Summary = IntegratedSummary{
		Components: 2,
		OK:         2,
		Warn:       0,
		Error:      0,
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal report: %v", err)
	}

	var parsed IntegratedReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal report: %v", err)
	}

	if parsed.Version != 1 {
		t.Errorf("expected version 1, got %d", parsed.Version)
	}

	if len(parsed.Components) != 2 {
		t.Errorf("expected 2 components, got %d", len(parsed.Components))
	}

	if parsed.Summary.Components != 2 {
		t.Errorf("expected 2 components in summary, got %d", parsed.Summary.Components)
	}
}
