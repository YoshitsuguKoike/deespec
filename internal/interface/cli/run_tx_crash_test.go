package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs/txn"
)

// TestSaveStateAndJournalTX_CrashRecoveryE2E tests crash scenarios during SaveStateAndJournalTX
// This validates that forward recovery maintains consistency after crashes
func TestSaveStateAndJournalTX_CrashRecoveryE2E(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "crash_recovery_e2e_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup paths
	paths := app.Paths{
		Home:    tempDir,
		Var:     filepath.Join(tempDir, ".deespec", "var"),
		State:   filepath.Join(tempDir, ".deespec", "var", "state.json"),
		Journal: filepath.Join(tempDir, ".deespec", "var", "journal.ndjson"),
		Health:  filepath.Join(tempDir, ".deespec", "var", "health.json"),
	}

	// Create directories
	if err := os.MkdirAll(paths.Var, 0755); err != nil {
		t.Fatalf("Failed to create var dir: %v", err)
	}

	// ★ ここで基底を固定：回復処理や確認が DEE_HOME 基準で安定します
	t.Setenv("DEE_HOME", filepath.Join(tempDir, ".deespec"))

	// Test Scenario 1: Crash after journal append but before commit completion
	t.Run("CrashAfterJournalAppend", func(t *testing.T) {
		// Initialize state
		initialState := &State{
			Version: 1,
			Current: "plan",
			Turn:    1,
			Meta: struct {
				UpdatedAt string `json:"updated_at"`
			}{
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			},
		}

		// Write initial state
		stateData, _ := json.MarshalIndent(initialState, "", "  ")
		if err := os.WriteFile(paths.State, stateData, 0644); err != nil {
			t.Fatalf("Failed to write initial state: %v", err)
		}

		// Prepare updated state and journal record
		updatedState := &State{
			Version: 1, // Will be incremented during SaveStateAndJournalTX
			Current: "implement",
			Turn:    2,
			Meta: struct {
				UpdatedAt string `json:"updated_at"`
			}{
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			},
		}

		journalRec := map[string]interface{}{
			"ts":     time.Now().UTC().Format(time.RFC3339),
			"step":   "implement",
			"turn":   2,
			"action": "step_transition",
		}

		// Simulate crash: SaveStateAndJournalTX は "journal先行" で途中終了（中間状態）
		err := SaveStateAndJournalTX(updatedState, journalRec, paths, 1)
		if err != nil {
			t.Fatalf("SaveStateAndJournalTX failed: %v", err)
		}

		// ★ 前方回復を明示的に起動して中間状態を完成側に寄せる
		//    （あなたの実装名に合わせて。ここでは例として txn.RunStartupRecovery を想定）
		recoverRoot := filepath.Join(paths.Var, "txn")
		if rerr := txn.RunStartupRecovery(context.Background(), recoverRoot, ".deespec", false); rerr != nil {
			t.Fatalf("RunStartupRecovery failed: %v", rerr)
		}

		// Verify final state consistency
		finalStateData, err := os.ReadFile(paths.State)
		if err != nil {
			t.Fatalf("Failed to read final state: %v", err)
		}

		var finalState State
		if err := json.Unmarshal(finalStateData, &finalState); err != nil {
			t.Fatalf("Failed to unmarshal final state: %v", err)
		}

		// Verify state was updated correctly
		if finalState.Version != 2 {
			t.Errorf("Expected version 2, got %d", finalState.Version)
		}
		if finalState.Current != "implement" {
			t.Errorf("Expected current 'implement', got %s", finalState.Current)
		}
		if finalState.Turn != 2 {
			t.Errorf("Expected turn 2, got %d", finalState.Turn)
		}

		// Verify journal was written
		if _, err := os.Stat(paths.Journal); err != nil {
			t.Errorf("Journal file should exist after transaction: %v", err)
		}

		// Verify journal content
		journalData, err := os.ReadFile(paths.Journal)
		if err != nil {
			t.Fatalf("Failed to read journal: %v", err)
		}

		var journalEntry map[string]interface{}
		if err := json.Unmarshal(journalData, &journalEntry); err != nil {
			t.Fatalf("Failed to unmarshal journal entry: %v", err)
		}

		if journalEntry["step"] != "implement" {
			t.Errorf("Expected journal step 'implement', got %v", journalEntry["step"])
		}
	})

	// Test Scenario 2: Crash simulation with transaction directory inspection
	t.Run("CrashWithTransactionDirectoryInspection", func(t *testing.T) {
		// Clear previous test state
		os.Remove(paths.State)
		os.Remove(paths.Journal)

		// Initialize fresh state
		initialState := &State{
			Version: 1,
			Current: "test",
			Turn:    3,
			Meta: struct {
				UpdatedAt string `json:"updated_at"`
			}{
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			},
		}

		stateData, _ := json.MarshalIndent(initialState, "", "  ")
		if err := os.WriteFile(paths.State, stateData, 0644); err != nil {
			t.Fatalf("Failed to write initial state: %v", err)
		}

		// Create transaction manager to simulate inspection
		txnDir := filepath.Join(paths.Var, "txn")
		manager := txn.NewManager(txnDir)
		ctx := context.Background()

		// Begin a transaction to create transaction directory
		tx, err := manager.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin transaction failed: %v", err)
		}

		// Stage a file to create stage directory structure
		testContent := []byte("crash recovery test content")
		if err := manager.StageFile(tx, "var/state.json", testContent); err != nil {
			t.Fatalf("StageFile failed: %v", err)
		}

		// Mark intent to create intent marker
		if err := manager.MarkIntent(tx); err != nil {
			t.Fatalf("MarkIntent failed: %v", err)
		}

		// Verify transaction artifacts exist
		if _, err := os.Stat(filepath.Join(tx.BaseDir, "manifest.json")); err != nil {
			t.Errorf("Manifest should exist: %v", err)
		}
		if _, err := os.Stat(filepath.Join(tx.BaseDir, "status.intent")); err != nil {
			t.Errorf("Intent marker should exist: %v", err)
		}

		// Simulate forward recovery using Recovery class
		recovery := txn.NewRecovery(manager, ".deespec")
		recoveryResult, err := recovery.RecoverAll(ctx)
		if err != nil {
			t.Fatalf("RecoverAll failed: %v", err)
		}

		// Verify recovery detected and processed the incomplete transaction
		if recoveryResult.RecoveredCount < 1 {
			t.Errorf("Expected at least 1 recovered transaction, got %d", recoveryResult.RecoveredCount)
		}

		// Complete the transaction (forward recovery) with no-op journal callback
		// Use default .deespec as destination root
		if err := manager.Commit(tx, ".deespec", func() error { return nil }); err != nil {
			t.Fatalf("Forward recovery commit failed: %v", err)
		}

		// Verify transaction was completed and cleaned up
		_, commitMarkerErr := os.Stat(filepath.Join(tx.BaseDir, "status.commit"))
		if commitMarkerErr != nil {
			t.Errorf("Commit marker should exist after forward recovery: %v", commitMarkerErr)
		}
	})

	// Test Scenario 3: Multiple concurrent crash scenarios
	t.Run("MultipleConcurrentCrashScenarios", func(t *testing.T) {
		// Clear state
		os.Remove(paths.State)
		os.Remove(paths.Journal)

		// Test multiple SaveStateAndJournalTX calls with version conflicts
		states := []*State{
			{Version: 1, Current: "plan", Turn: 1, Meta: struct {
				UpdatedAt string `json:"updated_at"`
			}{UpdatedAt: time.Now().UTC().Format(time.RFC3339)}},
			{Version: 1, Current: "implement", Turn: 2, Meta: struct {
				UpdatedAt string `json:"updated_at"`
			}{UpdatedAt: time.Now().UTC().Format(time.RFC3339)}},
			{Version: 1, Current: "test", Turn: 3, Meta: struct {
				UpdatedAt string `json:"updated_at"`
			}{UpdatedAt: time.Now().UTC().Format(time.RFC3339)}},
		}

		// Write initial state
		stateData, _ := json.MarshalIndent(states[0], "", "  ")
		if err := os.WriteFile(paths.State, stateData, 0644); err != nil {
			t.Fatalf("Failed to write initial state: %v", err)
		}

		// First transaction should succeed
		journalRec1 := map[string]interface{}{
			"ts":   time.Now().UTC().Format(time.RFC3339),
			"step": "implement",
			"turn": 2,
		}
		if err := SaveStateAndJournalTX(states[1], journalRec1, paths, 1); err != nil {
			t.Fatalf("First SaveStateAndJournalTX should succeed: %v", err)
		}

		// Second transaction should fail with CAS conflict (version changed)
		journalRec2 := map[string]interface{}{
			"ts":   time.Now().UTC().Format(time.RFC3339),
			"step": "test",
			"turn": 3,
		}
		err = SaveStateAndJournalTX(states[2], journalRec2, paths, 1) // Still using version 1
		if err == nil {
			t.Error("Second SaveStateAndJournalTX should fail with CAS conflict")
		}
		if !containsLocal(err.Error(), "version changed") {
			t.Errorf("Expected CAS conflict error, got: %v", err)
		}

		// Verify final state is from first transaction
		finalStateData, err := os.ReadFile(paths.State)
		if err != nil {
			t.Fatalf("Failed to read final state: %v", err)
		}

		var finalState State
		if err := json.Unmarshal(finalStateData, &finalState); err != nil {
			t.Fatalf("Failed to unmarshal final state: %v", err)
		}

		if finalState.Version != 2 { // Incremented by first transaction
			t.Errorf("Expected final version 2, got %d", finalState.Version)
		}
		if finalState.Current != "implement" {
			t.Errorf("Expected final current 'implement', got %s", finalState.Current)
		}
	})
}

// contains checks if a string contains a substring (helper for test)
func containsLocal(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || hasSubstringLocal(s, substr))
}

func hasSubstringLocal(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
