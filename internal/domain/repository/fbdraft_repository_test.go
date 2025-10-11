package repository_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"gopkg.in/yaml.v3"
)

// Test Suite for FBDraftRepository

func TestFBDraftRepository_PersistDraft(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	// Create temporary directory
	tempDir := t.TempDir()
	sbiDir := filepath.Join(tempDir, "test-sbi")

	// Create a test draft
	draft := dto.FBDraft{
		TargetTaskID:  "test-task-001",
		ReasonCode:    dto.DepUnresolved,
		Title:         "Test Feedback Draft",
		Summary:       "This task has unresolved dependencies",
		EvidencePaths: []string{"/path/to/evidence1.txt", "/path/to/evidence2.txt"},
		SuggestedFBID: "fb-001",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	// Persist the draft
	draftPath, err := repo.PersistDraft(ctx, draft, sbiDir)
	if err != nil {
		t.Fatalf("Failed to persist draft: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(sbiDir); os.IsNotExist(err) {
		t.Error("SBI directory was not created")
	}

	// Verify draft path is correct
	expectedPath := filepath.Join(sbiDir, "fb_draft.yaml")
	if draftPath != expectedPath {
		t.Errorf("Expected draft path %s, got %s", expectedPath, draftPath)
	}

	// Verify fb_context.md exists and has correct content
	contextPath := filepath.Join(sbiDir, "fb_context.md")
	contextData, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("Failed to read fb_context.md: %v", err)
	}

	contextContent := string(contextData)
	if !strings.Contains(contextContent, draft.TargetTaskID) {
		t.Error("fb_context.md does not contain target task ID")
	}
	if !strings.Contains(contextContent, string(draft.ReasonCode)) {
		t.Error("fb_context.md does not contain reason code")
	}
	if !strings.Contains(contextContent, draft.Summary) {
		t.Error("fb_context.md does not contain summary")
	}

	// Verify fb_evidence.txt exists and has correct content
	evidencePath := filepath.Join(sbiDir, "fb_evidence.txt")
	evidenceData, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatalf("Failed to read fb_evidence.txt: %v", err)
	}

	evidenceContent := string(evidenceData)
	if !strings.Contains(evidenceContent, draft.TargetTaskID) {
		t.Error("fb_evidence.txt does not contain target task ID")
	}
	if !strings.Contains(evidenceContent, string(draft.ReasonCode)) {
		t.Error("fb_evidence.txt does not contain reason code")
	}

	// Verify fb_draft.yaml exists and has correct structure
	var yamlData map[string]interface{}
	yamlBytes, err := os.ReadFile(draftPath)
	if err != nil {
		t.Fatalf("Failed to read fb_draft.yaml: %v", err)
	}

	if err := yaml.Unmarshal(yamlBytes, &yamlData); err != nil {
		t.Fatalf("Failed to unmarshal fb_draft.yaml: %v", err)
	}

	// Verify YAML structure
	if title, ok := yamlData["title"].(string); !ok || title != draft.Title {
		t.Errorf("Expected title '%s', got '%v'", draft.Title, yamlData["title"])
	}

	if relatesTo, ok := yamlData["relates_to"].(string); !ok || relatesTo != draft.TargetTaskID {
		t.Errorf("Expected relates_to '%s', got '%v'", draft.TargetTaskID, yamlData["relates_to"])
	}

	if reasonCode, ok := yamlData["reason_code"].(string); !ok || reasonCode != string(draft.ReasonCode) {
		t.Errorf("Expected reason_code '%s', got '%v'", string(draft.ReasonCode), yamlData["reason_code"])
	}

	// Verify labels contain expected values
	if labels, ok := yamlData["labels"].([]interface{}); ok {
		hasExpectedLabel := false
		for _, label := range labels {
			if labelStr, ok := label.(string); ok && labelStr == "feedback" {
				hasExpectedLabel = true
				break
			}
		}
		if !hasExpectedLabel {
			t.Error("Expected 'feedback' label in fb_draft.yaml")
		}
	}
}

func TestFBDraftRepository_PersistDraftInvalidCreatedAt(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	sbiDir := filepath.Join(tempDir, "test-sbi")

	// Create draft with invalid CreatedAt format
	draft := dto.FBDraft{
		TargetTaskID: "test-task-002",
		ReasonCode:   dto.MetaMissing,
		Title:        "Invalid Time Format",
		Summary:      "Testing invalid time format handling",
		CreatedAt:    "invalid-time-format",
	}

	// Should still succeed by using current time
	_, err := repo.PersistDraft(ctx, draft, sbiDir)
	if err != nil {
		t.Fatalf("Expected to handle invalid time format gracefully, got error: %v", err)
	}

	// Verify files were created
	if _, err := os.Stat(filepath.Join(sbiDir, "fb_context.md")); os.IsNotExist(err) {
		t.Error("fb_context.md was not created despite invalid time format")
	}
}

func TestFBDraftRepository_PersistDraftDirectoryCreation(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	// Use nested path that doesn't exist
	sbiDir := filepath.Join(tempDir, "level1", "level2", "test-sbi")

	draft := dto.FBDraft{
		TargetTaskID: "test-task-003",
		ReasonCode:   dto.PathInvalid,
		Title:        "Directory Creation Test",
		Summary:      "Testing nested directory creation",
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	_, err := repo.PersistDraft(ctx, draft, sbiDir)
	if err != nil {
		t.Fatalf("Failed to create nested directories: %v", err)
	}

	// Verify nested directories were created
	if _, err := os.Stat(sbiDir); os.IsNotExist(err) {
		t.Error("Nested directories were not created")
	}
}

func TestFBDraftRepository_PersistDraftAllReasonCodes(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	reasonCodes := []dto.IncompleteReason{
		dto.DepUnresolved,
		dto.DepCycle,
		dto.MetaMissing,
		dto.PathInvalid,
		dto.PromptError,
		dto.TimeFormat,
		dto.JournalGuard,
	}

	tempDir := t.TempDir()

	for i, reasonCode := range reasonCodes {
		sbiDir := filepath.Join(tempDir, "test-sbi-"+string(reasonCode))

		draft := dto.FBDraft{
			TargetTaskID: "test-task-" + string(reasonCode),
			ReasonCode:   reasonCode,
			Title:        "Test for " + string(reasonCode),
			Summary:      "Testing reason code: " + string(reasonCode),
			CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		}

		_, err := repo.PersistDraft(ctx, draft, sbiDir)
		if err != nil {
			t.Errorf("Failed to persist draft for reason code %s: %v", reasonCode, err)
			continue
		}

		// Verify reason code in YAML
		yamlPath := filepath.Join(sbiDir, "fb_draft.yaml")
		yamlBytes, err := os.ReadFile(yamlPath)
		if err != nil {
			t.Errorf("Failed to read YAML for reason code %s: %v", reasonCode, err)
			continue
		}

		var yamlData map[string]interface{}
		if err := yaml.Unmarshal(yamlBytes, &yamlData); err != nil {
			t.Errorf("Failed to unmarshal YAML for reason code %s: %v", reasonCode, err)
			continue
		}

		if rc, ok := yamlData["reason_code"].(string); !ok || rc != string(reasonCode) {
			t.Errorf("Test %d: Expected reason_code '%s', got '%v'", i, string(reasonCode), yamlData["reason_code"])
		}
	}
}

func TestFBDraftRepository_RecordDraftInJournal(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	journalPath := filepath.Join(tempDir, "journal.jsonl")

	draft := dto.FBDraft{
		TargetTaskID:  "test-task-001",
		ReasonCode:    dto.DepUnresolved,
		Title:         "Test Feedback Draft",
		Summary:       "Test summary",
		EvidencePaths: []string{"/path/to/evidence.txt"},
		SuggestedFBID: "fb-001",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	// Record draft in journal
	err := repo.RecordDraftInJournal(ctx, draft, journalPath, 1)
	if err != nil {
		t.Fatalf("Failed to record draft in journal: %v", err)
	}

	// Verify journal file exists
	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		t.Fatal("Journal file was not created")
	}

	// Read and verify journal content
	data, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal file: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) < 1 || lines[0] == "" {
		t.Fatal("Journal file is empty")
	}

	var record map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("Failed to unmarshal journal record: %v", err)
	}

	// Verify record structure
	if turn, ok := record["turn"].(float64); !ok || int(turn) != 1 {
		t.Errorf("Expected turn 1, got %v", record["turn"])
	}

	if step, ok := record["step"].(string); !ok || step != "plan" {
		t.Errorf("Expected step 'plan', got %v", record["step"])
	}

	// Verify artifacts
	artifacts, ok := record["artifacts"].([]interface{})
	if !ok || len(artifacts) == 0 {
		t.Fatal("Expected artifacts in journal record")
	}

	artifact, ok := artifacts[0].(map[string]interface{})
	if !ok {
		t.Fatal("Failed to parse artifact")
	}

	if artifactType, ok := artifact["type"].(string); !ok || artifactType != "fb_sbi_draft" {
		t.Errorf("Expected artifact type 'fb_sbi_draft', got %v", artifact["type"])
	}

	if targetTaskID, ok := artifact["target_task_id"].(string); !ok || targetTaskID != draft.TargetTaskID {
		t.Errorf("Expected target_task_id '%s', got %v", draft.TargetTaskID, artifact["target_task_id"])
	}

	if reasonCode, ok := artifact["reason_code"].(string); !ok || reasonCode != string(draft.ReasonCode) {
		t.Errorf("Expected reason_code '%s', got %v", string(draft.ReasonCode), artifact["reason_code"])
	}
}

func TestFBDraftRepository_RecordDraftInJournalMultipleRecords(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	journalPath := filepath.Join(tempDir, "journal.jsonl")

	// Record multiple drafts
	for i := 1; i <= 3; i++ {
		draft := dto.FBDraft{
			TargetTaskID:  "test-task-" + string(rune('0'+i)),
			ReasonCode:    dto.DepUnresolved,
			Title:         "Draft " + string(rune('0'+i)),
			Summary:       "Summary " + string(rune('0'+i)),
			EvidencePaths: []string{},
			SuggestedFBID: "fb-" + string(rune('0'+i)),
			CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		}

		err := repo.RecordDraftInJournal(ctx, draft, journalPath, i)
		if err != nil {
			t.Fatalf("Failed to record draft %d: %v", i, err)
		}
	}

	// Verify all records were appended
	data, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 journal records, got %d", len(lines))
	}

	// Verify each record
	for i, line := range lines {
		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Errorf("Failed to unmarshal record %d: %v", i, err)
			continue
		}

		if turn, ok := record["turn"].(float64); !ok || int(turn) != i+1 {
			t.Errorf("Record %d: Expected turn %d, got %v", i, i+1, record["turn"])
		}
	}
}

func TestFBDraftRepository_IsAlreadyRegistered(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	journalPath := filepath.Join(tempDir, "journal.jsonl")

	// Create a journal with registered artifact
	registeredRecord := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       1,
		"step":       "plan",
		"decision":   "",
		"elapsed_ms": 0,
		"error":      "",
		"artifacts": []interface{}{
			map[string]interface{}{
				"type":           "fb_sbi_registered",
				"target_task_id": "registered-task",
			},
		},
	}

	data, _ := json.Marshal(registeredRecord)
	err := os.WriteFile(journalPath, append(data, '\n'), 0644)
	if err != nil {
		t.Fatalf("Failed to create journal file: %v", err)
	}

	// Test: Task is registered
	isRegistered, err := repo.IsAlreadyRegistered(ctx, "registered-task", journalPath)
	if err != nil {
		t.Fatalf("Failed to check registration: %v", err)
	}

	if !isRegistered {
		t.Error("Expected task to be registered")
	}

	// Test: Task is not registered
	isRegistered, err = repo.IsAlreadyRegistered(ctx, "not-registered-task", journalPath)
	if err != nil {
		t.Fatalf("Failed to check registration: %v", err)
	}

	if isRegistered {
		t.Error("Expected task to not be registered")
	}
}

func TestFBDraftRepository_IsAlreadyRegisteredNoJournal(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	journalPath := filepath.Join(tempDir, "non-existent-journal.jsonl")

	// Should return false when journal doesn't exist
	isRegistered, err := repo.IsAlreadyRegistered(ctx, "any-task", journalPath)
	if err != nil {
		t.Errorf("Expected no error for non-existent journal, got: %v", err)
	}

	if isRegistered {
		t.Error("Expected false for non-existent journal")
	}
}

func TestFBDraftRepository_IsAlreadyRegisteredMultipleArtifacts(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	journalPath := filepath.Join(tempDir, "journal.jsonl")

	// Create journal with multiple records and different artifact types
	records := []map[string]interface{}{
		{
			"ts":   time.Now().UTC().Format(time.RFC3339Nano),
			"turn": 1,
			"step": "plan",
			"artifacts": []interface{}{
				map[string]interface{}{
					"type":           "fb_sbi_draft",
					"target_task_id": "task-1",
				},
			},
		},
		{
			"ts":   time.Now().UTC().Format(time.RFC3339Nano),
			"turn": 2,
			"step": "implement",
			"artifacts": []interface{}{
				map[string]interface{}{
					"type":           "fb_sbi_registered",
					"target_task_id": "task-1",
				},
			},
		},
		{
			"ts":   time.Now().UTC().Format(time.RFC3339Nano),
			"turn": 3,
			"step": "plan",
			"artifacts": []interface{}{
				map[string]interface{}{
					"type":           "fb_sbi_registered",
					"target_task_id": "task-2",
				},
			},
		},
	}

	// Write all records to journal
	file, err := os.Create(journalPath)
	if err != nil {
		t.Fatalf("Failed to create journal: %v", err)
	}
	defer file.Close()

	for _, record := range records {
		data, _ := json.Marshal(record)
		file.Write(append(data, '\n'))
	}
	file.Close()

	// Test: task-1 should be registered
	isRegistered, err := repo.IsAlreadyRegistered(ctx, "task-1", journalPath)
	if err != nil {
		t.Fatalf("Failed to check task-1: %v", err)
	}
	if !isRegistered {
		t.Error("Expected task-1 to be registered")
	}

	// Test: task-2 should be registered
	isRegistered, err = repo.IsAlreadyRegistered(ctx, "task-2", journalPath)
	if err != nil {
		t.Fatalf("Failed to check task-2: %v", err)
	}
	if !isRegistered {
		t.Error("Expected task-2 to be registered")
	}

	// Test: task-3 should not be registered
	isRegistered, err = repo.IsAlreadyRegistered(ctx, "task-3", journalPath)
	if err != nil {
		t.Fatalf("Failed to check task-3: %v", err)
	}
	if isRegistered {
		t.Error("Expected task-3 to not be registered")
	}
}

func TestFBDraftRepository_IsAlreadyRegisteredMalformedJSON(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	journalPath := filepath.Join(tempDir, "journal.jsonl")

	// Create journal with mixed valid and invalid JSON
	content := `{"ts":"2024-01-01T00:00:00Z","turn":1,"artifacts":[{"type":"fb_sbi_registered","target_task_id":"task-1"}]}
{invalid json line}
{"ts":"2024-01-01T00:00:00Z","turn":2,"artifacts":[{"type":"fb_sbi_registered","target_task_id":"task-2"}]}
`
	err := os.WriteFile(journalPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write journal: %v", err)
	}

	// Should skip malformed lines and still find valid registrations
	isRegistered, err := repo.IsAlreadyRegistered(ctx, "task-1", journalPath)
	if err != nil {
		t.Fatalf("Failed to check task-1: %v", err)
	}
	if !isRegistered {
		t.Error("Expected task-1 to be found despite malformed JSON in journal")
	}

	isRegistered, err = repo.IsAlreadyRegistered(ctx, "task-2", journalPath)
	if err != nil {
		t.Fatalf("Failed to check task-2: %v", err)
	}
	if !isRegistered {
		t.Error("Expected task-2 to be found despite malformed JSON in journal")
	}
}

func TestFBDraftRepository_IntegrationPersistAndRecord(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	sbiDir := filepath.Join(tempDir, "sbi")
	journalPath := filepath.Join(tempDir, "journal.jsonl")

	draft := dto.FBDraft{
		TargetTaskID:  "integration-test-task",
		ReasonCode:    dto.DepCycle,
		Title:         "Integration Test",
		Summary:       "Testing full workflow",
		EvidencePaths: []string{"/path/to/evidence.txt"},
		SuggestedFBID: "fb-integration",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	// Step 1: Persist draft
	draftPath, err := repo.PersistDraft(ctx, draft, sbiDir)
	if err != nil {
		t.Fatalf("Failed to persist draft: %v", err)
	}

	if _, err := os.Stat(draftPath); os.IsNotExist(err) {
		t.Error("Draft file does not exist after persistence")
	}

	// Step 2: Record in journal
	err = repo.RecordDraftInJournal(ctx, draft, journalPath, 1)
	if err != nil {
		t.Fatalf("Failed to record in journal: %v", err)
	}

	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		t.Error("Journal file does not exist after recording")
	}

	// Step 3: Check registration (should not be registered yet)
	isRegistered, err := repo.IsAlreadyRegistered(ctx, draft.TargetTaskID, journalPath)
	if err != nil {
		t.Fatalf("Failed to check registration: %v", err)
	}

	if isRegistered {
		t.Error("Draft should not be registered (only drafted)")
	}

	// Step 4: Simulate registration
	registeredRecord := map[string]interface{}{
		"ts":   time.Now().UTC().Format(time.RFC3339Nano),
		"turn": 2,
		"step": "implement",
		"artifacts": []interface{}{
			map[string]interface{}{
				"type":           "fb_sbi_registered",
				"target_task_id": draft.TargetTaskID,
			},
		},
	}

	file, err := os.OpenFile(journalPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open journal: %v", err)
	}
	data, _ := json.Marshal(registeredRecord)
	file.Write(append(data, '\n'))
	file.Close()

	// Step 5: Check registration again (should be registered now)
	isRegistered, err = repo.IsAlreadyRegistered(ctx, draft.TargetTaskID, journalPath)
	if err != nil {
		t.Fatalf("Failed to check registration after registration: %v", err)
	}

	if !isRegistered {
		t.Error("Draft should be registered after registration record")
	}
}

func TestFBDraftRepository_FilePermissions(t *testing.T) {
	repo := repository.NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	sbiDir := filepath.Join(tempDir, "sbi")

	draft := dto.FBDraft{
		TargetTaskID: "permission-test",
		ReasonCode:   dto.MetaMissing,
		Title:        "Permission Test",
		Summary:      "Testing file permissions",
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	_, err := repo.PersistDraft(ctx, draft, sbiDir)
	if err != nil {
		t.Fatalf("Failed to persist draft: %v", err)
	}

	// Check directory permissions (should be 0755)
	dirInfo, err := os.Stat(sbiDir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	if dirInfo.Mode().Perm() != 0755 {
		t.Errorf("Expected directory permission 0755, got %o", dirInfo.Mode().Perm())
	}

	// Check file permissions (should be 0644)
	files := []string{"fb_context.md", "fb_evidence.txt", "fb_draft.yaml"}
	for _, filename := range files {
		filePath := filepath.Join(sbiDir, filename)
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			t.Errorf("Failed to stat %s: %v", filename, err)
			continue
		}

		if fileInfo.Mode().Perm() != 0644 {
			t.Errorf("Expected %s permission 0644, got %o", filename, fileInfo.Mode().Perm())
		}
	}
}
