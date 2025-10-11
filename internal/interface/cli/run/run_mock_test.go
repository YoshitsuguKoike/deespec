package run

import (
	"fmt"
	"os"
	"testing"
	"time"

	// "github.com/YoshitsuguKoike/deespec/internal/interface/cli/common" // Removed: tests using common.State commented out
	// "github.com/YoshitsuguKoike/deespec/internal/interface/cli/notes" // Moved to Application layer
	"go.uber.org/goleak"
)

// Test helper functions
func TestSummarizeText(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli/run.SetupSignalHandler.func1"))
	tests := []struct {
		name     string
		text     string
		maxLines int
		expected string
	}{
		{
			name:     "Short text",
			text:     "Hello",
			maxLines: 10,
			expected: "Hello",
		},
		{
			name:     "Text needs truncation",
			text:     "Line1\nLine2\nLine3\nLine4\nLine5",
			maxLines: 3,
			expected: "Line1\nLine2\nLine3\n... (total 5 lines)",
		},
		{
			name:     "Empty text",
			text:     "",
			maxLines: 10,
			expected: "",
		},
		{
			name:     "Exact line count",
			text:     "Line1\nLine2\nLine3",
			maxLines: 3,
			expected: "Line1\nLine2\nLine3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := summarizeText(tt.text, tt.maxLines)
			if result != tt.expected {
				t.Errorf("summarizeText(%q, %d) = %q, want %q",
					tt.text, tt.maxLines, result, tt.expected)
			}
		})
	}
}

// Commented out: buildImplementPrompt function removed - state management migrated to DB
/*
func TestBuildImplementPrompt(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli/run.SetupSignalHandler.func1"))
	// Create test state
	st := &common.State{
		WIP:  "TEST-001",
		Turn: 1,
	}

	prompt := buildImplementPrompt(st)
	if prompt == "" {
		t.Error("buildImplementPrompt returned empty string")
	}
}
*/

// Commented out: buildReviewPrompt function removed - state management migrated to DB
/*
func TestBuildReviewPrompt(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli/run.SetupSignalHandler.func1"))
	// Create test state
	st := &common.State{
		WIP:  "TEST-001",
		Turn: 1,
	}

	prompt := buildReviewPrompt(st)
	if prompt == "" {
		t.Error("buildReviewPrompt returned empty string")
	}
}
*/

func TestGetCurrentWorkDir(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli/run.SetupSignalHandler.func1"))
	dir := getCurrentWorkDir()
	if dir == "" {
		t.Error("getCurrentWorkDir returned empty string")
	}

	// Verify it's a valid directory
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("getCurrentWorkDir returned invalid directory: %v", err)
	}
}

// Test nextStep function that was missing
// func TestNextStep(t *testing.T) {
// 	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli/run.SetupSignalHandler.func1"))
// 	tests := []struct {
// 		name     string
// 		current  string
// 		decision string
// 		expected string
// 	}{
// 		{
// 			name:     "plan to implement",
// 			current:  "plan",
// 			decision: "",
// 			expected: "implement",
// 		},
// 		{
// 			name:     "implement to test",
// 			current:  "implement",
// 			decision: "",
// 			expected: "test",
// 		},
// 		{
// 			name:     "test to review",
// 			current:  "test",
// 			decision: "",
// 			expected: "review",
// 		},
// 		{
// 			name:     "review succeeded",
// 			current:  "review",
// 			decision: "SUCCEEDED",
// 			expected: "done",
// 		},
// 		{
// 			name:     "review with OK (legacy)",
// 			current:  "review",
// 			decision: "OK",
// 			expected: "done",
// 		},
// 		{
// 			name:     "review needs changes",
// 			current:  "review",
// 			decision: "NEEDS_CHANGES",
// 			expected: "implement",
// 		},
// 		{
// 			name:     "review failed",
// 			current:  "review",
// 			decision: "FAILED",
// 			expected: "implement",
// 		},
// 		{
// 			name:     "done stays done",
// 			current:  "done",
// 			decision: "",
// 			expected: "done",
// 		},
// 		{
// 			name:     "unknown defaults to plan",
// 			current:  "unknown",
// 			decision: "",
// 			expected: "plan",
// 		},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result := nextStep(tt.current, tt.decision)
// 			if result != tt.expected {
// 				t.Errorf("nextStep(%q, %q) = %q, want %q",
// 					tt.current, tt.decision, result, tt.expected)
// 			}
// 		})
// 	}
// }

// Test ExtractNoteBody
// COMMENTED OUT: notes package moved to Application layer (service/notes_service.go)
// func TestExtractNoteBody(t *testing.T) {
// 	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli/run.SetupSignalHandler.func1"))
// 	tests := []struct {
// 		name     string
// 		content  string
// 		noteType string
// 		expected string
// 	}{
// 		{
// 			name: "Implementation Note",
// 			content: `## Review Result
//
// The implementation looks good.
//
// ## Implementation Note
// This is the implementation note body.
// Another line of the note.
//
// ## Other Section`,
// 			noteType: "implement",
// 			expected: "This is the implementation note body.\nAnother line of the note.\n## Other Section",
// 		},
// 		{
// 			name: "Review Note",
// 			content: `## Analysis
//
// Some analysis here.
//
// ## Review Note
// Review note content here.
// More review content.
//
// ## Conclusion`,
// 			noteType: "review",
// 			expected: "Review note content here.\nMore review content.\n## Conclusion",
// 		},
// 		{
// 			name:     "No note found",
// 			content:  "Some content without notes",
// 			noteType: "implement",
// 			expected: "Some content without notes",
// 		},
// 		{
// 			name: "Note at end of content",
// 			content: `## Summary
//
// Summary text
//
// ## Implementation Note
// Final note content`,
// 			noteType: "implement",
// 			expected: "Final note content",
// 		},
// 		{
// 			name:     "Empty content",
// 			content:  "",
// 			noteType: "implement",
// 			expected: "",
// 		},
// 		{
// 			name: "Alternative note format",
// 			content: `### Implementation Note
// Alternative format note body
// Second line
//
// ### End ###`,
// 			noteType: "implement",
// 			expected: "Alternative format note body\nSecond line\n### End ###",
// 		},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result := notes.ExtractNoteBody(tt.content, tt.noteType)
// 			if result != tt.expected {
// 				t.Errorf("notes.ExtractNoteBody() for %s = %q, want %q",
// 					tt.name, result, tt.expected)
// 			}
// 		})
// 	}
// }

// Test AppendNote basic functionality
func TestLogClaudeInteraction(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli/run.SetupSignalHandler.func1"))
	// Capture log output
	originalTime := time.Now()

	// Test successful interaction
	logClaudeInteraction(
		"Test prompt",
		"Test result",
		nil,
		originalTime,
		originalTime.Add(2*time.Second),
	)

	// Test with error
	logClaudeInteraction(
		"Test prompt",
		"",
		fmt.Errorf("test error"),
		originalTime,
		originalTime.Add(1*time.Second),
	)
}

// Commented out: getTaskDescription function removed - state management migrated to DB
/*
func TestGetTaskDescription(t *testing.T) {
	tests := []struct {
		name     string
		state    *common.State
		expected string
	}{
		{
			name: "With WIP task",
			state: &common.State{
				WIP: "TEST-001",
			},
			expected: "",
		},
		{
			name: "Empty WIP",
			state: &common.State{
				WIP: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTaskDescription(tt.state)
			// Just check that function runs without panic
			_ = result
		})
	}
}
*/

// Commented out: determineStep function removed - state management migrated to DB
/*
func TestDetermineStep(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus string
		attempt       int
		expectedStep  string
	}{
		{
			name:          "WIP status",
			currentStatus: "WIP",
			attempt:       1,
			expectedStep:  "implement_try",
		},
		{
			name:          "REVIEW status",
			currentStatus: "REVIEW",
			attempt:       1,
			expectedStep:  "first_review",
		},
		{
			name:          "REVIEW&WIP status",
			currentStatus: "REVIEW&WIP",
			attempt:       3,
			expectedStep:  "reviewer_force_implement",
		},
		{
			name:          "DONE status",
			currentStatus: "DONE",
			attempt:       1,
			expectedStep:  "done",
		},
		{
			name:          "READY status",
			currentStatus: "READY",
			attempt:       1,
			expectedStep:  "implement_try",
		},
		{
			name:          "Empty status",
			currentStatus: "",
			attempt:       1,
			expectedStep:  "implement_try",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineStep(tt.currentStatus, tt.attempt)
			if result != tt.expectedStep {
				t.Errorf("determineStep(%q, %d) = %q, want %q",
					tt.currentStatus, tt.attempt, result, tt.expectedStep)
			}
		})
	}
}
*/

// Commented out: buildPromptByStatus function removed - state management migrated to DB
/*
func TestBuildPromptByStatus(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	// Create test state
	st := &common.State{
		WIP:    "TEST-001",
		Turn:   1,
		Status: "WIP",
	}

	tests := []struct {
		name        string
		status      string
		expectEmpty bool
	}{
		{
			name:        "WIP status",
			status:      "WIP",
			expectEmpty: false,
		},
		{
			name:        "REVIEW status",
			status:      "REVIEW",
			expectEmpty: false,
		},
		{
			name:        "REVIEW&WIP status",
			status:      "REVIEW&WIP",
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st.Status = tt.status
			prompt := buildPromptByStatus(st, nil)
			if tt.expectEmpty {
				if prompt != "" {
					t.Errorf("Expected empty prompt for status=%q", tt.status)
				}
			} else {
				if prompt == "" {
					t.Errorf("Expected non-empty prompt for status=%q", tt.status)
				}
			}
		})
	}
}
*/

// func TestNewRunCmd(t *testing.T) {
// 	// Test that newRunCmd creates a valid command
// 	cmd := newRunCmd()
// 	if cmd == nil {
// 		t.Fatal("newRunCmd returned nil")
// 	}
// 	if cmd.Use != "run" {
// 		t.Errorf("Expected Use='run', got %q", cmd.Use)
// 	}
// 	if cmd.Short == "" {
// 		t.Error("Expected non-empty Short description")
// 	}
// }

func TestGetCurrentWorkingDirectory(t *testing.T) {
	// Test the function returns a valid directory
	dir := getCurrentWorkDir()
	if dir == "" {
		t.Error("getCurrentWorkDir returned empty string")
	}
}

// COMMENTED OUT: notes package moved to Application layer (service/notes_service.go)
// func TestAppendNoteBasic(t *testing.T) {
// 	// Create temp directory for test
// 	tmpDir := t.TempDir()
// 	originalDir, _ := os.Getwd()
// 	if err := os.Chdir(tmpDir); err != nil {
// 		t.Fatal(err)
// 	}
// 	defer os.Chdir(originalDir)
//
// 	// Create necessary directories - based on actual implementation
// 	sbiDir := filepath.Join(".deespec", "specs", "sbi", "TEST-001")
// 	if err := os.MkdirAll(sbiDir, 0755); err != nil {
// 		t.Fatal(err)
// 	}
//
// 	// Test appending a note with fixed time
// 	testTime, _ := time.Parse(time.RFC3339, "2023-09-01T10:00:00Z")
// 	err := notes.AppendNote("implement", "PENDING", "Test note body", 1, "TEST-001", testTime)
// 	if err != nil {
// 		t.Errorf("AppendNote failed: %v", err)
// 	}
//
// 	// Check if file was created in the correct location
// 	noteFile := filepath.Join(sbiDir, "impl_notes.md")
// 	if _, err := os.Stat(noteFile); os.IsNotExist(err) {
// 		t.Error("Note file was not created")
// 	}
// }
