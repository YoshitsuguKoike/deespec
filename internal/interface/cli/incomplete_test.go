package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectIncomplete(t *testing.T) {
	tests := []struct {
		name           string
		task           *Task
		ctx            *PickContext
		expectedReasons []IncompleteReason
	}{
		{
			name: "unresolved dependency",
			task: &Task{
				ID:    "SBI-002",
				Title: "Test Task",
				Meta: map[string]interface{}{
					"depends_on": []string{"SBI-001"},
				},
			},
			ctx: &PickContext{
				CompletedTasks: map[string]bool{
					// SBI-001 is not completed
				},
				AllTasks: []*Task{},
			},
			expectedReasons: []IncompleteReason{DepUnresolved},
		},
		{
			name: "cyclic dependency",
			task: &Task{
				ID:    "SBI-001",
				Title: "Task 1",
				Meta: map[string]interface{}{
					"depends_on": []string{"SBI-002"},
				},
			},
			ctx: &PickContext{
				CompletedTasks: map[string]bool{},
				AllTasks: []*Task{
					{
						ID: "SBI-001",
						Meta: map[string]interface{}{
							"depends_on": []string{"SBI-002"},
						},
					},
					{
						ID: "SBI-002",
						Meta: map[string]interface{}{
							"depends_on": []string{"SBI-001"},
						},
					},
				},
			},
			expectedReasons: []IncompleteReason{DepUnresolved, DepCycle},
		},
		{
			name: "missing meta fields",
			task: &Task{
				ID:    "",
				Title: "",
				Meta:  map[string]interface{}{},
			},
			ctx: &PickContext{
				CompletedTasks: map[string]bool{},
				AllTasks:       []*Task{},
			},
			expectedReasons: []IncompleteReason{MetaMissing},
		},
		{
			name: "invalid path with absolute",
			task: &Task{
				ID:         "SBI-003",
				Title:      "Test",
				Meta:       map[string]interface{}{},
				PromptPath: "/absolute/path/prompt.txt",
			},
			ctx: &PickContext{
				CompletedTasks: map[string]bool{},
				AllTasks:       []*Task{},
			},
			expectedReasons: []IncompleteReason{PathInvalid},
		},
		{
			name: "invalid path with parent dir",
			task: &Task{
				ID:         "SBI-004",
				Title:      "Test",
				Meta:       map[string]interface{}{},
				PromptPath: "../parent/prompt.txt",
			},
			ctx: &PickContext{
				CompletedTasks: map[string]bool{},
				AllTasks:       []*Task{},
			},
			expectedReasons: []IncompleteReason{PathInvalid},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drafts, err := DetectIncomplete(tt.task, tt.ctx)
			if err != nil {
				t.Errorf("DetectIncomplete() error = %v", err)
				return
			}

			if len(drafts) != len(tt.expectedReasons) {
				t.Errorf("Expected %d drafts, got %d", len(tt.expectedReasons), len(drafts))
			}

			// Check that all expected reasons are present
			reasonMap := make(map[IncompleteReason]bool)
			for _, draft := range drafts {
				reasonMap[draft.ReasonCode] = true
			}

			for _, expectedReason := range tt.expectedReasons {
				if !reasonMap[expectedReason] {
					t.Errorf("Expected reason %s not found in drafts", expectedReason)
				}
			}
		})
	}
}

func TestPersistFBDraft(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")

	draft := FBDraft{
		TargetTaskID: "SBI-001",
		ReasonCode:   DepUnresolved,
		Title:        "【FB】SBI-001 の不完全指示修正",
		Summary:      "依存未解決: depends_on=[SBI-000]（未完了）",
		CreatedAt:    time.Now().UTC(),
	}

	draftPath, err := PersistFBDraft(draft, artifactsDir)
	if err != nil {
		t.Fatalf("PersistFBDraft() error = %v", err)
	}

	// Check that files were created
	expectedFiles := []string{
		filepath.Join(artifactsDir, "fb_sbi", "SBI-001", "context.md"),
		filepath.Join(artifactsDir, "fb_sbi", "SBI-001", "evidence.txt"),
		filepath.Join(artifactsDir, "fb_sbi", "SBI-001", "draft.yaml"),
	}

	for _, file := range expectedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected file %s not created", file)
		}
	}

	// Verify the draft.yaml content
	if draftPath != expectedFiles[2] {
		t.Errorf("Expected draft path %s, got %s", expectedFiles[2], draftPath)
	}

	// Read and verify draft.yaml has required fields
	draftContent, err := os.ReadFile(draftPath)
	if err != nil {
		t.Errorf("Failed to read draft.yaml: %v", err)
	}

	requiredStrings := []string{
		"title:", "labels:", "por:", "priority:",
		"relates_to:", "reason_code:", "details:",
	}

	for _, required := range requiredStrings {
		if !testContains(string(draftContent), required) {
			t.Errorf("draft.yaml missing required field: %s", required)
		}
	}
}

func TestContainsInvalidPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"relative/path/file.txt", false},
		{"./local/file.txt", false},
		{"/absolute/path/file.txt", true},
		{"../parent/file.txt", true},
		{"path/../file.txt", true},
		{"C:\\Windows\\path", true},
		{"path\\with\\backslash", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := containsInvalidPath(tt.path)
			if result != tt.expected {
				t.Errorf("containsInvalidPath(%s) = %v, expected %v",
					tt.path, result, tt.expected)
			}
		})
	}
}

func TestDetectTaskCycle(t *testing.T) {
	tests := []struct {
		name     string
		task     *Task
		allTasks []*Task
		hasCycle bool
	}{
		{
			name: "no cycle",
			task: &Task{
				ID: "SBI-003",
				Meta: map[string]interface{}{
					"depends_on": []string{"SBI-002"},
				},
			},
			allTasks: []*Task{
				{ID: "SBI-001", Meta: map[string]interface{}{}},
				{ID: "SBI-002", Meta: map[string]interface{}{"depends_on": []string{"SBI-001"}}},
				{ID: "SBI-003", Meta: map[string]interface{}{"depends_on": []string{"SBI-002"}}},
			},
			hasCycle: false,
		},
		{
			name: "direct cycle",
			task: &Task{
				ID: "SBI-001",
				Meta: map[string]interface{}{
					"depends_on": []string{"SBI-002"},
				},
			},
			allTasks: []*Task{
				{ID: "SBI-001", Meta: map[string]interface{}{"depends_on": []string{"SBI-002"}}},
				{ID: "SBI-002", Meta: map[string]interface{}{"depends_on": []string{"SBI-001"}}},
			},
			hasCycle: true,
		},
		{
			name: "indirect cycle",
			task: &Task{
				ID: "SBI-001",
				Meta: map[string]interface{}{
					"depends_on": []string{"SBI-002"},
				},
			},
			allTasks: []*Task{
				{ID: "SBI-001", Meta: map[string]interface{}{"depends_on": []string{"SBI-002"}}},
				{ID: "SBI-002", Meta: map[string]interface{}{"depends_on": []string{"SBI-003"}}},
				{ID: "SBI-003", Meta: map[string]interface{}{"depends_on": []string{"SBI-001"}}},
			},
			hasCycle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectTaskCycle(tt.task, tt.allTasks)
			if result != tt.hasCycle {
				t.Errorf("detectTaskCycle() = %v, expected %v", result, tt.hasCycle)
			}
		})
	}
}

// Helper function for string contains
func testContains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || testContainsMiddle(s, substr)))
}

func testContainsMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}