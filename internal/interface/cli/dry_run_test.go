package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/pkg/specpath"
	"github.com/YoshitsuguKoike/deespec/internal/testutil"
)

func TestDryRun_NoSideEffects(t *testing.T) {
	// Create test workspace
	cleanup := testutil.NewTestWorkspace(t)
	t.Cleanup(cleanup)

	// Create a simple policy
	policyContent := `
id:
  pattern: "^[A-Z0-9-]{1,64}$"
  max_len: 64
collision:
  default_mode: "error"
`
	testutil.WriteTestPolicy(t, policyContent)

	// Run dry-run
	input := `
id: TEST-001
title: Test Spec
labels: [test]
`
	// Create temp input file with relative path
	inputFile := testutil.WriteTestInput(t, input)

	// Capture output
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	// Run dry-run
	err := runDryRun(false, inputFile, "", "json", false)

	wOut.Close()
	wErr.Close()
	stdout, _ := io.ReadAll(rOut)
	stderr, _ := io.ReadAll(rErr)

	if err != nil {
		t.Errorf("Unexpected error: %v\nStderr: %s", err, stderr)
	}

	// Parse output
	var report DryRunReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		t.Fatalf("Failed to parse output: %v\n%s", err, stdout)
	}

	// Verify dry-run flag is set
	if !report.Meta.DryRun {
		t.Error("Expected dry_run to be true in meta")
	}

	// Verify no spec directory was created
	expectedPath := ".deespec/specs/sbi/TEST-001_test-spec"
	testutil.AssertFileNotExists(t, expectedPath)

	// Verify no journal was written
	testutil.AssertFileNotExists(t, ".deespec/journal.ndjson")
}

func TestDryRun_Precedence(t *testing.T) {
	// Create test workspace
	cleanup := testutil.NewTestWorkspace(t)
	t.Cleanup(cleanup)

	// Create policy with collision mode "error"
	policyPath := "test_policy.yaml"
	policyContent := `
collision:
  default_mode: "error"
  suffix_limit: 5
`
	os.WriteFile(policyPath, []byte(policyContent), 0644)

	// Load policy
	policy, _ := LoadRegisterPolicy(policyPath)

	// Test CLI override
	config, _ := ResolveRegisterConfig("suffix", policy)

	// CLI should override policy
	if config.CollisionMode != "suffix" {
		t.Errorf("Expected CLI collision mode 'suffix', got: %s", config.CollisionMode)
	}

	// Policy value should be used for suffix limit
	if config.SuffixLimit != 5 {
		t.Errorf("Expected policy suffix_limit 5, got: %d", config.SuffixLimit)
	}
}

func TestDryRun_CollisionErrorMode_Fails(t *testing.T) {
	// Create test workspace
	cleanup := testutil.NewTestWorkspace(t)
	t.Cleanup(cleanup)

	// Compute the expected spec path
	cfg := specpath.ResolvedConfig{
		PathBaseDir:    ".deespec/specs/sbi",
		SlugAllowChars: "a-z0-9-",
		SlugMaxRunes:   60,
		SlugLowercase:  true,
		SlugNFKC:       true,
	}
	existingPath, _ := specpath.ComputeSpecPath("TEST-COLL-001", "Test", cfg)
	testutil.CreateSpecDirectory(t, existingPath)

	// Verify the directory was created
	if _, err := os.Stat(existingPath); err != nil {
		t.Fatalf("Failed to create existing directory: %v", err)
	}
	t.Logf("Created collision path: %s", existingPath)

	// Don't create a policy file - let defaults apply
	// Creating an empty or minimal policy causes parse errors

	// Input that would collide
	input := `
id: TEST-COLL-001
title: Test
`
	inputFile := testutil.WriteTestInput(t, input)

	// Capture output
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	// Run dry-run
	err := runDryRun(false, inputFile, "", "json", false)

	wOut.Close()
	wErr.Close()
	stdout, _ := io.ReadAll(rOut)
	stderr, _ := io.ReadAll(rErr)

	// Should fail with collision error
	if err == nil {
		t.Error("Expected error for collision in error mode")
	} else if !strings.Contains(err.Error(), "path already exists") {
		t.Errorf("Expected 'path already exists' error, got: %v", err)
	}

	// Parse output
	var report DryRunReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		t.Fatalf("Failed to parse output: %v\n%s", err, stdout)
	}

	t.Logf("Dry-run resolution:")
	t.Logf("  BaseDir: %s", report.Resolution.BaseDir)
	t.Logf("  Slug: %s", report.Resolution.Slug)
	t.Logf("  SpecPath: %s", report.Resolution.SpecPath)
	t.Logf("  FinalPath: %s", report.Resolution.FinalPath)
	t.Logf("  Collision detected: %v", report.Resolution.CollisionWouldHappen)

	// Verify collision detection
	if !report.Resolution.CollisionWouldHappen {
		t.Error("Expected collision_would_happen to be true")
	}

	if report.Resolution.CollisionResolution != "error:would-fail" {
		t.Errorf("Expected collision_resolution 'error:would-fail', got: %s", report.Resolution.CollisionResolution)
	}

	// Verify stderr contains error
	if !strings.Contains(string(stderr), "ERROR:") {
		t.Errorf("Expected ERROR in stderr, got: %s", stderr)
	}
}

func TestDryRun_CollisionSuffixMode_Succeeds(t *testing.T) {
	// Create test workspace
	cleanup := testutil.NewTestWorkspace(t)
	t.Cleanup(cleanup)

	// Compute the expected spec path
	cfg := specpath.ResolvedConfig{
		PathBaseDir:    ".deespec/specs/sbi",
		SlugAllowChars: "a-z0-9-",
		SlugMaxRunes:   60,
		SlugLowercase:  true,
		SlugNFKC:       true,
	}
	existingPath, _ := specpath.ComputeSpecPath("TEST-COLL-002", "Test", cfg)
	testutil.CreateSpecDirectory(t, existingPath)

	// Input that would collide
	input := `
id: TEST-COLL-002
title: Test
`
	inputFile := testutil.WriteTestInput(t, input)

	// Capture output
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	rOut, wOut, _ := os.Pipe()
	_, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	// Run dry-run with suffix mode
	err := runDryRun(false, inputFile, "suffix", "json", false)

	wOut.Close()
	wErr.Close()
	stdout, _ := io.ReadAll(rOut)

	// Should succeed
	if err != nil {
		t.Errorf("Unexpected error in suffix mode: %v", err)
	}

	// Parse output
	var report DryRunReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		t.Fatalf("Failed to parse output: %v\n%s", err, stdout)
	}

	// Verify collision resolution
	if !report.Resolution.CollisionWouldHappen {
		t.Error("Expected collision_would_happen to be true")
	}

	if report.Resolution.CollisionResolution != "suffix:_2" {
		t.Errorf("Expected collision_resolution 'suffix:_2', got: %s", report.Resolution.CollisionResolution)
	}

	if !strings.HasSuffix(report.Resolution.FinalPath, "_2") {
		t.Errorf("Expected final_path to end with _2, got: %s", report.Resolution.FinalPath)
	}
}

func TestDryRun_PathSafety(t *testing.T) {
	// Create test workspace
	cleanup := testutil.NewTestWorkspace(t)
	t.Cleanup(cleanup)

	// Test with title that looks like traversal
	input := `
id: TEST-SAFE-001
title: "../../../etc/passwd"
`
	inputFile := testutil.WriteTestInput(t, input)

	// Capture output
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	rOut, wOut, _ := os.Pipe()
	_, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	// Run dry-run
	err := runDryRun(false, inputFile, "", "json", false)

	wOut.Close()
	wErr.Close()
	stdout, _ := io.ReadAll(rOut)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Parse output
	var report DryRunReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// Slug should sanitize the traversal attempt
	if strings.Contains(report.Resolution.Slug, "..") {
		t.Errorf("Slug should not contain '..': %s", report.Resolution.Slug)
	}

	// Path should be safe
	if !report.Resolution.PathSafe {
		t.Error("Expected path_safe to be true after slug sanitization")
	}

	// Final path should not contain traversal
	if strings.Contains(report.Resolution.FinalPath, "..") {
		t.Errorf("Final path should not contain '..': %s", report.Resolution.FinalPath)
	}
}

func TestDryRun_JournalPreviewSchema(t *testing.T) {
	// Create test workspace
	cleanup := testutil.NewTestWorkspace(t)
	t.Cleanup(cleanup)

	input := `
id: TEST-JOURNAL-001
title: Journal Test
labels: [test]
`
	inputFile := testutil.WriteTestInput(t, input)

	// Capture output
	oldStdout := os.Stdout
	defer func() {
		os.Stdout = oldStdout
	}()

	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	// Run dry-run
	runDryRun(false, inputFile, "", "json", false)

	wOut.Close()
	stdout, _ := io.ReadAll(rOut)

	// Parse output
	var report DryRunReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// Verify journal preview structure
	journal := report.JournalPreview

	// Check required fields
	if journal.Ts == "" {
		t.Error("Journal preview missing 'ts' field")
	}

	if journal.Step != "plan" {
		t.Errorf("Expected step 'plan', got: %s", journal.Step)
	}

	if journal.Decision != "PENDING" {
		t.Errorf("Expected decision 'PENDING', got: %s", journal.Decision)
	}

	if len(journal.Artifacts) == 0 {
		t.Error("Journal preview missing artifacts")
	}

	// Verify artifact structure
	if len(journal.Artifacts) > 0 {
		artifact := journal.Artifacts[0]
		if artifact["type"] != "register" {
			t.Errorf("Expected artifact type 'register', got: %v", artifact["type"])
		}
		if artifact["id"] != "TEST-JOURNAL-001" {
			t.Errorf("Expected artifact id 'TEST-JOURNAL-001', got: %v", artifact["id"])
		}
		if artifact["spec_path"] == nil {
			t.Error("Artifact missing spec_path")
		}
	}
}

func TestDryRun_OutputSchemaStable(t *testing.T) {
	// Create test workspace
	cleanup := testutil.NewTestWorkspace(t)
	t.Cleanup(cleanup)

	input := `
id: TEST-SCHEMA-001
title: Schema Test
`
	inputFile := testutil.WriteTestInput(t, input)

	// Run multiple times to check stability
	var outputs []string
	for i := 0; i < 3; i++ {
		// Capture output
		oldStdout := os.Stdout
		rOut, wOut, _ := os.Pipe()
		os.Stdout = wOut

		runDryRun(false, inputFile, "", "json", false)

		wOut.Close()
		stdout, _ := io.ReadAll(rOut)
		os.Stdout = oldStdout

		// Parse and re-marshal to normalize
		var report DryRunReport
		json.Unmarshal(stdout, &report)

		// Clear timestamp fields for comparison
		report.Meta.TsUTC = "NORMALIZED"
		report.JournalPreview.Ts = "NORMALIZED"
		report.JournalPreview.ElapsedMs = 0

		normalized, _ := json.MarshalIndent(report, "", "  ")
		outputs = append(outputs, string(normalized))
	}

	// All outputs should be identical (stable order)
	for i := 1; i < len(outputs); i++ {
		if outputs[i] != outputs[0] {
			t.Errorf("Output %d differs from output 0, schema is not stable", i)
		}
	}

	// Verify schema version is present
	var report DryRunReport
	json.Unmarshal([]byte(outputs[0]), &report)
	if report.Meta.SchemaVersion != 1 {
		t.Errorf("Expected schema_version 1, got: %d", report.Meta.SchemaVersion)
	}
}

func TestDryRun_StderrLevel(t *testing.T) {
	// Create test workspace
	cleanup := testutil.NewTestWorkspace(t)
	t.Cleanup(cleanup)

	// Create policy with stderr_level_default: error
	policyPath := "test_policy.yaml"
	policyContent := `
logging:
  stderr_level_default: "error"
`
	os.WriteFile(policyPath, []byte(policyContent), 0644)

	// Mock GetPolicyPath
	oldGetPolicyPath := GetPolicyPath
	GetPolicyPath = func() string { return policyPath }
	defer func() { GetPolicyPath = oldGetPolicyPath }()

	// Create input
	input := `
id: TEST-LOG-001
title: Log Test
`
	inputFile := testutil.WriteTestInput(t, input)

	// Capture stderr
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()

	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	// Also capture stdout to avoid pollution
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	// Run dry-run
	runDryRun(false, inputFile, "", "json", false)

	wErr.Close()
	wOut.Close()
	stderr, _ := io.ReadAll(rErr)
	io.ReadAll(rOut) // Drain stdout

	// With stderr_level_default: error, INFO messages should not appear
	if strings.Contains(string(stderr), "INFO:") {
		t.Errorf("INFO messages should not appear with stderr_level_default: error, got: %s", stderr)
	}
}
