package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadState(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantStep string
		wantTurn int
		wantErr  bool
	}{
		{
			name:     "minimal valid state",
			input:    `{"version":1,"step":"plan","turn":0}`,
			wantStep: "plan",
			wantTurn: 0,
		},
		{
			name:     "state with defaults",
			input:    `{"version":1}`,
			wantStep: "plan", // default
			wantTurn: 0,      // default
		},
		{
			name:     "legacy current field",
			input:    `{"version":1,"current":"implement","turn":2}`,
			wantStep: "implement", // migrated from current
			wantTurn: 2,
		},
		{
			name:     "invalid step value",
			input:    `{"version":1,"step":"foo","turn":1}`,
			wantStep: "foo", // loaded but warned
			wantTurn: 1,
		},
		{
			name:     "both step and current (step wins)",
			input:    `{"version":1,"step":"test","current":"implement","turn":3}`,
			wantStep: "test", // step takes precedence
			wantTurn: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			statePath := filepath.Join(tmpDir, "state.json")
			if err := os.WriteFile(statePath, []byte(tt.input), 0644); err != nil {
				t.Fatal(err)
			}

			// Load state
			state, err := LoadState(statePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Check fields
			if state.Step != tt.wantStep {
				t.Errorf("Step = %v, want %v", state.Step, tt.wantStep)
			}
			if state.Turn != tt.wantTurn {
				t.Errorf("Turn = %v, want %v", state.Turn, tt.wantTurn)
			}
			if state.Version != 1 {
				t.Errorf("Version = %v, want 1", state.Version)
			}
		})
	}
}

func TestSaveStateAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	state := &State{
		Version: 1,
		Step:    "implement",
		Turn:    5,
	}

	// Save state
	beforeSave := time.Now()
	if err := SaveStateAtomic(state, statePath); err != nil {
		t.Fatalf("SaveStateAtomic() error = %v", err)
	}
	afterSave := time.Now()

	// Read back and verify
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	// Check trailing newline
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("Saved file should end with newline")
	}

	// Parse and verify
	var saved State
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}

	if saved.Version != 1 {
		t.Errorf("Version = %v, want 1", saved.Version)
	}
	if saved.Step != "implement" {
		t.Errorf("Step = %v, want implement", saved.Step)
	}
	if saved.Turn != 5 {
		t.Errorf("Turn = %v, want 5", saved.Turn)
	}

	// Check updated_at timestamp
	if saved.Meta == nil || saved.Meta["updated_at"] == nil {
		t.Error("Meta.updated_at should be set")
	} else {
		tsStr, ok := saved.Meta["updated_at"].(string)
		if !ok {
			t.Error("Meta.updated_at should be a string")
		} else {
			// Parse timestamp and verify it's recent
			ts, err := time.Parse(time.RFC3339Nano, tsStr)
			if err != nil {
				t.Errorf("Invalid RFC3339Nano timestamp: %v", err)
			}
			if ts.Before(beforeSave) || ts.After(afterSave) {
				t.Error("Timestamp should be between test boundaries")
			}
			// Check it's UTC and has nanosecond precision
			if !strings.HasSuffix(tsStr, "Z") {
				t.Error("Timestamp should be in UTC (end with Z)")
			}
			if !strings.Contains(tsStr, ".") {
				t.Error("Timestamp should have nanosecond precision")
			}
		}
	}
}

func TestSaveStateAtomicOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Create initial file
	initial := `{"version":1,"step":"plan","turn":0}`
	if err := os.WriteFile(statePath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	// Overwrite with new state
	state := &State{
		Version: 1,
		Step:    "done",
		Turn:    10,
	}

	if err := SaveStateAtomic(state, statePath); err != nil {
		t.Fatalf("SaveStateAtomic() overwrite error = %v", err)
	}

	// Verify overwritten content
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	var saved State
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}

	if saved.Step != "done" || saved.Turn != 10 {
		t.Error("File should be overwritten with new values")
	}
}
