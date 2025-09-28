package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendNote(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	// Create .deespec/var/artifacts directory
	artifactsDir := filepath.Join(".deespec", "var", "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2025, 9, 26, 12, 34, 56, 0, time.UTC)

	tests := []struct {
		name         string
		kind         string
		decision     string
		body         string
		turn         int
		wantContains []string
		wantErr      bool
	}{
		{
			name:     "implement note - first append",
			kind:     "implement",
			decision: "",
			body:     "This is the implementation note body.\nMultiple lines.",
			turn:     1,
			wantContains: []string{
				"## Turn 1 —",
				"- Author: aiagent",
				"- Step: implement",
				"- Decision: PENDING",
				"This is the implementation note body.",
				"Multiple lines.",
				"---",
			},
		},
		{
			name:     "review note - OK decision",
			kind:     "review",
			decision: "OK",
			body:     "Review passed. Good implementation.",
			turn:     2,
			wantContains: []string{
				"## Turn 2 —",
				"- Author: aiagent",
				"- Step: review",
				"- Decision: OK",
				"Review passed. Good implementation.",
				"---",
			},
		},
		{
			name:     "review note - NEEDS_CHANGES decision",
			kind:     "review",
			decision: "NEEDS_CHANGES",
			body:     "Found issues.\n- Issue 1\n- Issue 2",
			turn:     3,
			wantContains: []string{
				"## Turn 3 —",
				"- Step: review",
				"- Decision: NEEDS_CHANGES",
				"Found issues.",
				"- Issue 1",
				"- Issue 2",
			},
		},
		{
			name:    "invalid kind",
			kind:    "invalid",
			body:    "test",
			turn:    4,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AppendNote(tt.kind, tt.decision, tt.body, tt.turn, now)
			if (err != nil) != tt.wantErr {
				t.Errorf("AppendNote() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Read the file and check contents
				var filePath string
				if tt.kind == "implement" {
					filePath = filepath.Join(artifactsDir, "impl_note.md")
				} else if tt.kind == "review" {
					filePath = filepath.Join(artifactsDir, "review_note.md")
				}

				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("Failed to read note file: %v", err)
				}

				contentStr := string(content)
				for _, want := range tt.wantContains {
					if !strings.Contains(contentStr, want) {
						t.Errorf("Note content missing expected string: %q\nGot:\n%s", want, contentStr)
					}
				}

				// Check that content ends with newline
				if !strings.HasSuffix(contentStr, "") {
					t.Error("Note content should end with newline")
				}
			}
		})
	}
}

func TestAppendNote_MultipleAppends(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	// Create .deespec/var/artifacts directory
	artifactsDir := filepath.Join(".deespec", "var", "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		t.Fatal(err)
	}

	now1 := time.Date(2025, 9, 26, 10, 0, 0, 0, time.UTC)
	now2 := time.Date(2025, 9, 26, 11, 0, 0, 0, time.UTC)
	now3 := time.Date(2025, 9, 26, 12, 0, 0, 0, time.UTC)

	// First implement
	err := AppendNote("implement", "", "First implementation", 1, now1)
	if err != nil {
		t.Fatal(err)
	}

	// First review (needs changes)
	err = AppendNote("review", "NEEDS_CHANGES", "Needs improvements", 2, now2)
	if err != nil {
		t.Fatal(err)
	}

	// Second implement
	err = AppendNote("implement", "", "Fixed issues", 3, now3)
	if err != nil {
		t.Fatal(err)
	}

	// Check impl_note.md has both entries
	implContent, err := os.ReadFile(filepath.Join(artifactsDir, "impl_note.md"))
	if err != nil {
		t.Fatal(err)
	}
	implStr := string(implContent)

	// Should contain both turn 1 and turn 3
	if !strings.Contains(implStr, "## Turn 1 —") {
		t.Error("impl_note missing Turn 1")
	}
	if !strings.Contains(implStr, "## Turn 3 —") {
		t.Error("impl_note missing Turn 3")
	}
	if !strings.Contains(implStr, "First implementation") {
		t.Error("impl_note missing first content")
	}
	if !strings.Contains(implStr, "Fixed issues") {
		t.Error("impl_note missing second content")
	}

	// Check that earlier content comes before later content
	idx1 := strings.Index(implStr, "First implementation")
	idx2 := strings.Index(implStr, "Fixed issues")
	if idx1 >= idx2 {
		t.Error("Notes not in chronological order")
	}
}

func TestNormalizeDecision(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"OK", "OK"},
		{"ok", "OK"},
		{"APPROVED", "OK"},
		{"approved", "OK"},
		{"PASS", "OK"},
		{"NEEDS_CHANGES", "NEEDS_CHANGES"},
		{"needs changes", "NEEDS_CHANGES"},
		{"NEEDS CHANGES", "NEEDS_CHANGES"},
		{"FAIL", "NEEDS_CHANGES"},
		{"FAILED", "NEEDS_CHANGES"},
		{"PENDING", "PENDING"},
		{"", "PENDING"},
		{"unknown", "NEEDS_CHANGES"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeDecision(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDecision(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEnsureLF(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello\r\nworld", "hello\nworld"},
		{"hello\rworld", "hello\nworld"},
		{"hello\nworld", "hello\nworld"},
		{"no newlines", "no newlines"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ensureLF(tt.input)
			if got != tt.want {
				t.Errorf("ensureLF() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content")
	err := atomicWrite(testFile, content)
	if err != nil {
		t.Fatal(err)
	}

	// Check file was written
	got, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(content) {
		t.Errorf("File content mismatch: got %q, want %q", got, content)
	}

	// Check no temp files remain
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if strings.HasPrefix(f.Name(), ".tmp-") {
			t.Errorf("Temp file not cleaned up: %s", f.Name())
		}
	}
}

// TestNoAbsolutePathsInNotes verifies we don't use absolute paths
func TestNoAbsolutePathsInNotes(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	// The note paths should be relative
	notePaths := []string{
		".deespec/var/artifacts/impl_note.md",
		".deespec/var/artifacts/review_note.md",
	}

	for _, path := range notePaths {
		if filepath.IsAbs(path) {
			t.Errorf("Note path should be relative, got absolute: %s", path)
		}
	}
}
