package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"gopkg.in/yaml.v3"
)

// Test Suite for FBDraftRepositoryImpl

func TestFBDraftRepositoryImpl_NewFBDraftRepositoryImpl(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
	if repo == nil {
		t.Fatal("NewFBDraftRepositoryImpl returned nil")
	}

	// Verify it implements the interface
	if _, ok := repo.(*FBDraftRepositoryImpl); !ok {
		t.Error("NewFBDraftRepositoryImpl did not return *FBDraftRepositoryImpl")
	}
}

func TestFBDraftRepositoryImpl_PersistDraft_Success(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
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
	if !strings.Contains(contextContent, "# 不完全指示検出レポート") {
		t.Error("fb_context.md does not contain expected header")
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
		expectedLabels := []string{"feedback", "pick", "sbi-fb"}
		for _, expectedLabel := range expectedLabels {
			found := false
			for _, label := range labels {
				if labelStr, ok := label.(string); ok && labelStr == expectedLabel {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected '%s' label in fb_draft.yaml", expectedLabel)
			}
		}
	} else {
		t.Error("Labels not found in fb_draft.yaml")
	}

	// Verify por and priority fields
	if por, ok := yamlData["por"].(int); !ok || por != 1 {
		t.Errorf("Expected por 1, got %v", yamlData["por"])
	}

	if priority, ok := yamlData["priority"].(int); !ok || priority != 1 {
		t.Errorf("Expected priority 1, got %v", yamlData["priority"])
	}
}

func TestFBDraftRepositoryImpl_PersistDraft_InvalidCreatedAt(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
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
	draftPath, err := repo.PersistDraft(ctx, draft, sbiDir)
	if err != nil {
		t.Fatalf("Expected to handle invalid time format gracefully, got error: %v", err)
	}

	// Verify files were created
	if _, err := os.Stat(filepath.Join(sbiDir, "fb_context.md")); os.IsNotExist(err) {
		t.Error("fb_context.md was not created despite invalid time format")
	}

	// Verify YAML was created with valid timestamp
	yamlBytes, err := os.ReadFile(draftPath)
	if err != nil {
		t.Fatalf("Failed to read fb_draft.yaml: %v", err)
	}

	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(yamlBytes, &yamlData); err != nil {
		t.Fatalf("Failed to unmarshal fb_draft.yaml: %v", err)
	}

	// Verify details field contains a timestamp
	if details, ok := yamlData["details"].(string); ok {
		if !strings.Contains(details, "検出時刻:") {
			t.Error("YAML details should contain timestamp even with invalid input")
		}
	}
}

func TestFBDraftRepositoryImpl_PersistDraft_NestedDirectoryCreation(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	// Use nested path that doesn't exist
	sbiDir := filepath.Join(tempDir, "level1", "level2", "level3", "test-sbi")

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

	// Verify all three files exist
	files := []string{"fb_context.md", "fb_evidence.txt", "fb_draft.yaml"}
	for _, file := range files {
		filePath := filepath.Join(sbiDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("File %s was not created in nested directory", file)
		}
	}
}

func TestFBDraftRepositoryImpl_PersistDraft_AllReasonCodes(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
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

	for _, reasonCode := range reasonCodes {
		sbiDir := filepath.Join(tempDir, "test-sbi-"+string(reasonCode))

		draft := dto.FBDraft{
			TargetTaskID: "test-task-" + string(reasonCode),
			ReasonCode:   reasonCode,
			Title:        "Test for " + string(reasonCode),
			Summary:      "Testing reason code: " + string(reasonCode),
			CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		}

		draftPath, err := repo.PersistDraft(ctx, draft, sbiDir)
		if err != nil {
			t.Errorf("Failed to persist draft for reason code %s: %v", reasonCode, err)
			continue
		}

		// Verify reason code in YAML
		yamlBytes, err := os.ReadFile(draftPath)
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
			t.Errorf("Expected reason_code '%s', got '%v'", string(reasonCode), yamlData["reason_code"])
		}

		// Verify reason code in context file
		contextPath := filepath.Join(sbiDir, "fb_context.md")
		contextData, err := os.ReadFile(contextPath)
		if err != nil {
			t.Errorf("Failed to read context file for reason code %s: %v", reasonCode, err)
			continue
		}

		if !strings.Contains(string(contextData), string(reasonCode)) {
			t.Errorf("Context file does not contain reason code %s", reasonCode)
		}
	}
}

func TestFBDraftRepositoryImpl_PersistDraft_FilePermissions(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	sbiDir := filepath.Join(tempDir, "test-sbi")

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

func TestFBDraftRepositoryImpl_RecordDraftInJournal_Success(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
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

	if decision, ok := record["decision"].(string); !ok || decision != "" {
		t.Errorf("Expected empty decision, got %v", record["decision"])
	}

	if elapsedMs, ok := record["elapsed_ms"].(float64); !ok || int(elapsedMs) != 0 {
		t.Errorf("Expected elapsed_ms 0, got %v", record["elapsed_ms"])
	}

	// Verify timestamp format
	if ts, ok := record["ts"].(string); !ok {
		t.Error("Expected 'ts' field in journal record")
	} else {
		if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
			t.Errorf("Invalid timestamp format: %s", ts)
		}
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

	if title, ok := artifact["title"].(string); !ok || title != draft.Title {
		t.Errorf("Expected title '%s', got %v", draft.Title, artifact["title"])
	}

	if summary, ok := artifact["summary"].(string); !ok || summary != draft.Summary {
		t.Errorf("Expected summary '%s', got %v", draft.Summary, artifact["summary"])
	}

	if suggestedFBID, ok := artifact["suggested_fb_id"].(string); !ok || suggestedFBID != draft.SuggestedFBID {
		t.Errorf("Expected suggested_fb_id '%s', got %v", draft.SuggestedFBID, artifact["suggested_fb_id"])
	}
}

func TestFBDraftRepositoryImpl_RecordDraftInJournal_MultipleRecords(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	journalPath := filepath.Join(tempDir, "journal.jsonl")

	// Record multiple drafts
	for i := 1; i <= 5; i++ {
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
	if len(lines) != 5 {
		t.Errorf("Expected 5 journal records, got %d", len(lines))
	}

	// Verify each record has correct turn number
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

func TestFBDraftRepositoryImpl_RecordDraftInJournal_AppendToExisting(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	journalPath := filepath.Join(tempDir, "journal.jsonl")

	// Create initial journal entry manually
	initialRecord := map[string]interface{}{
		"ts":   time.Now().UTC().Format(time.RFC3339Nano),
		"turn": 0,
		"step": "init",
	}
	initialData, _ := json.Marshal(initialRecord)
	err := os.WriteFile(journalPath, append(initialData, '\n'), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial journal: %v", err)
	}

	// Add new draft
	draft := dto.FBDraft{
		TargetTaskID: "test-task-append",
		ReasonCode:   dto.DepCycle,
		Title:        "Append Test",
		Summary:      "Testing append to existing journal",
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	err = repo.RecordDraftInJournal(ctx, draft, journalPath, 1)
	if err != nil {
		t.Fatalf("Failed to append to journal: %v", err)
	}

	// Verify both records exist
	data, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 records, got %d", len(lines))
	}
}

func TestFBDraftRepositoryImpl_IsAlreadyRegistered_RegisteredTask(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
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

func TestFBDraftRepositoryImpl_IsAlreadyRegistered_NoJournal(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
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

func TestFBDraftRepositoryImpl_IsAlreadyRegistered_EmptyJournal(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	journalPath := filepath.Join(tempDir, "empty-journal.jsonl")

	// Create empty journal
	err := os.WriteFile(journalPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty journal: %v", err)
	}

	// Should return false for empty journal
	isRegistered, err := repo.IsAlreadyRegistered(ctx, "any-task", journalPath)
	if err != nil {
		t.Errorf("Expected no error for empty journal, got: %v", err)
	}

	if isRegistered {
		t.Error("Expected false for empty journal")
	}
}

func TestFBDraftRepositoryImpl_IsAlreadyRegistered_MultipleArtifacts(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
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

func TestFBDraftRepositoryImpl_IsAlreadyRegistered_MalformedJSON(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
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

func TestFBDraftRepositoryImpl_IsAlreadyRegistered_MixedArtifactTypes(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
	ctx := context.Background()

	tempDir := t.TempDir()
	journalPath := filepath.Join(tempDir, "journal.jsonl")

	// Create journal with multiple artifact types in single record
	record := map[string]interface{}{
		"ts":   time.Now().UTC().Format(time.RFC3339Nano),
		"turn": 1,
		"step": "plan",
		"artifacts": []interface{}{
			map[string]interface{}{
				"type":           "fb_sbi_draft",
				"target_task_id": "task-1",
			},
			map[string]interface{}{
				"type":           "fb_sbi_registered",
				"target_task_id": "task-2",
			},
			map[string]interface{}{
				"type": "other_artifact",
				"data": "something",
			},
		},
	}

	data, _ := json.Marshal(record)
	err := os.WriteFile(journalPath, append(data, '\n'), 0644)
	if err != nil {
		t.Fatalf("Failed to write journal: %v", err)
	}

	// task-1 should not be registered (only drafted)
	isRegistered, err := repo.IsAlreadyRegistered(ctx, "task-1", journalPath)
	if err != nil {
		t.Fatalf("Failed to check task-1: %v", err)
	}
	if isRegistered {
		t.Error("Expected task-1 to not be registered (only drafted)")
	}

	// task-2 should be registered
	isRegistered, err = repo.IsAlreadyRegistered(ctx, "task-2", journalPath)
	if err != nil {
		t.Fatalf("Failed to check task-2: %v", err)
	}
	if !isRegistered {
		t.Error("Expected task-2 to be registered")
	}
}

func TestFBDraftRepositoryImpl_IntegrationTest(t *testing.T) {
	repo := NewFBDraftRepositoryImpl()
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

	// Step 4: Simulate registration by adding a registered record
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
