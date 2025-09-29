package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/app"
)

// TestStateJournalTXConsistency verifies atomic updates of state and journal
func TestStateJournalTXConsistency(t *testing.T) {
	// Setup test environment
	tempDir, err := os.MkdirTemp("", "state_tx_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .deespec structure
	varDir := filepath.Join(tempDir, ".deespec/var")
	if err := os.MkdirAll(varDir, 0755); err != nil {
		t.Fatalf("mkdir %s failed: %v", varDir, err)
	}
	if err := os.MkdirAll(filepath.Join(varDir, "txn"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Setup paths
	paths := app.Paths{
		Var:       varDir,
		State:     filepath.Join(varDir, "state.json"),
		Journal:   filepath.Join(varDir, "journal.ndjson"),
		StateLock: filepath.Join(varDir, "state.lock"),
		Health:    filepath.Join(varDir, "health.json"),
	}

	// Create initial state
	state := &State{
		Version: 1,
		Turn:    1,
		Current: "plan",
		Meta: struct {
			UpdatedAt string `json:"updated_at"`
		}{
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		WIP: "SBI-TEST-001",
	}

	// Save initial state
	stateData, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(paths.State, stateData, 0644)

	// Test 1: Successful atomic update
	t.Run("AtomicUpdate", func(t *testing.T) {
		journalRec := map[string]interface{}{
			"ts":       time.Now().UTC().Format(time.RFC3339Nano),
			"turn":     2,
			"step":     "implement",
			"decision": "",
		}

		prevVersion := state.Version
		err := SaveStateAndJournalTX(state, journalRec, paths, prevVersion)
		if err != nil {
			t.Fatalf("SaveStateAndJournalTX failed: %v", err)
		}

		// Verify state was updated
		savedState, err := os.ReadFile(paths.State)
		if err != nil {
			t.Fatalf("Failed to read state: %v", err)
		}

		var loadedState State
		json.Unmarshal(savedState, &loadedState)
		if loadedState.Version != 2 {
			t.Errorf("Expected version 2, got %d", loadedState.Version)
		}

		// Verify journal was appended
		journalData, err := os.ReadFile(paths.Journal)
		if err != nil {
			t.Fatalf("Failed to read journal: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(journalData)), "\n")
		if len(lines) != 1 {
			t.Errorf("Expected 1 journal entry, got %d", len(lines))
		}

		var entry map[string]interface{}
		json.Unmarshal([]byte(lines[0]), &entry)
		if entry["turn"].(float64) != 2 {
			t.Errorf("Journal entry has wrong turn: %v", entry["turn"])
		}
	})

	// Test 2: CAS version protection (basic check)
	t.Run("CASProtection", func(t *testing.T) {
		// Try to update with wrong version
		wrongVersion := 999

		journalRec := map[string]interface{}{
			"ts":   time.Now().UTC().Format(time.RFC3339Nano),
			"turn": 99,
			"step": "invalid",
		}

		testState := &State{
			Version: wrongVersion,
			Turn:    99,
			Current: "test",
			Meta: struct {
				UpdatedAt string `json:"updated_at"`
			}{
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			},
			WIP: state.WIP,
		}

		err := SaveStateAndJournalTX(testState, journalRec, paths, wrongVersion)
		if err == nil {
			t.Error("Expected CAS failure with wrong version")
		}

		if !strings.Contains(err.Error(), "version changed") {
			t.Errorf("Expected CAS error, got: %v", err)
		}
	})

	// Test 3: Basic functionality verification
	t.Run("BasicFunction", func(t *testing.T) {
		// Just verify the TX function works without error
		currentVersion := state.Version

		journalRec := map[string]interface{}{
			"ts":   time.Now().UTC().Format(time.RFC3339Nano),
			"turn": state.Turn + 1,
			"step": "basic_test",
		}

		testState := &State{
			Version: currentVersion,
			Turn:    state.Turn + 1,
			Current: "basic",
			Meta: struct {
				UpdatedAt string `json:"updated_at"`
			}{
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			},
			WIP: state.WIP,
		}

		err := SaveStateAndJournalTX(testState, journalRec, paths, currentVersion)
		if err != nil {
			t.Errorf("Basic TX operation failed: %v", err)
		}

		// Verify state was updated
		if testState.Version != currentVersion+1 {
			t.Errorf("State version not updated: expected %d, got %d", currentVersion+1, testState.Version)
		}
	})
}

// TestStateJournalConsistencyCheck verifies state/journal consistency after operations
func TestStateJournalConsistencyCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "consistency_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	varDir := filepath.Join(tempDir, ".deespec/var")
	if err := os.MkdirAll(filepath.Join(varDir, "txn"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	paths := app.Paths{
		Var:     varDir,
		State:   filepath.Join(varDir, "state.json"),
		Journal: filepath.Join(varDir, "journal.ndjson"),
	}

	// Run multiple TX updates
	state := &State{
		Version: 1,
		Turn:    1,
		Current: "plan",
		Meta: struct {
			UpdatedAt string `json:"updated_at"`
		}{
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}

	// Save initial state
	stateData, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(paths.State, stateData, 0644)

	// Perform 100 updates
	for i := 0; i < 100; i++ {
		// Load current state from disk to get correct version
		currentState, err := loadState(paths.State)
		if err != nil {
			t.Fatalf("Failed to load state for update %d: %v", i, err)
		}

		journalRec := map[string]interface{}{
			"ts":   time.Now().UTC().Format(time.RFC3339Nano),
			"turn": i + 2,
			"step": fmt.Sprintf("step_%d", i),
		}

		prevVersion := currentState.Version
		currentState.Turn = i + 2

		err = SaveStateAndJournalTX(currentState, journalRec, paths, prevVersion)
		if err != nil {
			t.Fatalf("Update %d failed: %v", i, err)
		}
	}

	// Verify final consistency
	// Read final state from disk
	finalState, err := loadState(paths.State)
	if err != nil {
		t.Fatalf("Failed to load final state: %v", err)
	}

	// Read journal entries
	journalData, _ := os.ReadFile(paths.Journal)
	lines := strings.Split(strings.TrimSpace(string(journalData)), "\n")

	// Should have exactly 100 journal entries
	if len(lines) != 100 {
		t.Errorf("Expected 100 journal entries, got %d", len(lines))
	}

	// State turn should match last journal entry
	var lastEntry map[string]interface{}
	json.Unmarshal([]byte(lines[len(lines)-1]), &lastEntry)

	t.Logf("Final state: version=%d, turn=%d", finalState.Version, finalState.Turn)
	t.Logf("Last journal entry: turn=%v", lastEntry["turn"])

	// State turn should match last journal entry
	if lastEntry["turn"] != nil {
		if float64(finalState.Turn) != lastEntry["turn"].(float64) {
			t.Errorf("State/Journal mismatch: state turn=%d, journal turn=%v",
				finalState.Turn, lastEntry["turn"])
		}
	} else {
		t.Errorf("Last journal entry has no turn field")
	}

	// State version should be 101 (1 initial + 100 updates)
	expectedVersion := 1 + 100
	if finalState.Version != expectedVersion {
		t.Errorf("Expected state version %d, got %d", expectedVersion, finalState.Version)
	}

	t.Logf("Successfully verified %d consistent state/journal updates", len(lines))
}
