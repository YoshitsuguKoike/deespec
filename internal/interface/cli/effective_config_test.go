package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPrintEffectiveConfig_NoPolicy_Defaults(t *testing.T) {
	// Ensure no policy file exists
	tmpDir := t.TempDir()
	oldPolicyPath := GetPolicyPath()
	defer func() {
		// Reset to original path
		os.Setenv("DEESPEC_POLICY_PATH", oldPolicyPath)
	}()

	// Set policy path to non-existent file
	nonExistentPolicy := filepath.Join(tmpDir, "non-existent.yaml")
	os.Setenv("DEESPEC_POLICY_PATH", nonExistentPolicy)

	// Capture stdout and stderr
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

	// Run command
	err := runPrintEffectiveConfig("", "json", false, false)

	// Close writers and read output
	wOut.Close()
	wErr.Close()
	stdout, _ := io.ReadAll(rOut)
	stderr, _ := io.ReadAll(rErr)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check stderr for expected message
	if !strings.Contains(string(stderr), "INFO: no policy file found") {
		t.Errorf("Expected 'no policy file found' in stderr, got: %s", stderr)
	}

	// Parse JSON output
	var effective EffectiveConfig
	if err := json.Unmarshal(stdout, &effective); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\n%s", err, stdout)
	}

	// Verify defaults
	if effective.Meta.PolicyFileFound {
		t.Error("Expected PolicyFileFound to be false")
	}
	if effective.Meta.PolicyPath != "" {
		t.Errorf("Expected empty PolicyPath, got: %s", effective.Meta.PolicyPath)
	}
	if effective.Collision.DefaultMode != "error" {
		t.Errorf("Expected default collision mode 'error', got: %s", effective.Collision.DefaultMode)
	}
	if effective.Path.BaseDir != ".deespec/specs/sbi" {
		t.Errorf("Expected default base_dir '.deespec/specs/sbi', got: %s", effective.Path.BaseDir)
	}
}

func TestPrintEffectiveConfig_PolicyAndCLI_Precedence(t *testing.T) {
	// Create a test policy file
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "test_policy.yaml")
	policyContent := `
collision:
  default_mode: "error"
  suffix_limit: 10
logging:
  stderr_level_default: "warn"
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to write test policy: %v", err)
	}

	// Load policy
	policy, err := LoadRegisterPolicy(policyPath)
	if err != nil {
		t.Fatalf("Failed to load policy: %v", err)
	}

	// Test CLI override
	config, err := ResolveRegisterConfig("suffix", policy)
	if err != nil {
		t.Fatalf("Failed to resolve config: %v", err)
	}

	// CLI should override policy
	if config.CollisionMode != "suffix" {
		t.Errorf("Expected CLI collision mode 'suffix' to override policy, got: %s", config.CollisionMode)
	}

	// Policy value should be used when CLI doesn't override
	if config.SuffixLimit != 10 {
		t.Errorf("Expected policy suffix_limit 10, got: %d", config.SuffixLimit)
	}
	if config.StderrLevel != "warn" {
		t.Errorf("Expected policy stderr_level 'warn', got: %s", config.StderrLevel)
	}
}

func TestPrintEffectiveConfig_FormatCompact(t *testing.T) {
	// Test compact JSON
	config := GetDefaultPolicy()
	resolvedConfig, _ := ResolveRegisterConfig("", config)
	effective := buildEffectiveConfig(resolvedConfig, false, "")

	// Test compact JSON
	compactJSON, err := json.Marshal(effective)
	if err != nil {
		t.Fatalf("Failed to marshal compact JSON: %v", err)
	}

	if bytes.Contains(compactJSON, []byte("\n")) {
		t.Error("Compact JSON should not contain newlines")
	}

	// Test pretty JSON
	prettyJSON, err := json.MarshalIndent(effective, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal pretty JSON: %v", err)
	}

	if !bytes.Contains(prettyJSON, []byte("\n")) {
		t.Error("Pretty JSON should contain newlines")
	}

	// Test YAML format
	yamlOutput, err := yaml.Marshal(effective)
	if err != nil {
		t.Fatalf("Failed to marshal YAML: %v", err)
	}

	if !bytes.Contains(yamlOutput, []byte("meta:")) {
		t.Error("YAML output should contain 'meta:' key")
	}
}

func TestPrintEffectiveConfig_InvalidPolicy(t *testing.T) {
	// Create a policy file with unknown field
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "invalid_policy.yaml")
	policyContent := `
unknown_field: true
collision:
  default_mode: "error"
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to write test policy: %v", err)
	}

	// Load policy should fail
	_, err := LoadRegisterPolicy(policyPath)
	if err == nil {
		t.Error("Expected error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("Expected error to mention unknown field, got: %v", err)
	}
}

func TestEffectiveConfig_ContainsPathInputs(t *testing.T) {
	config := GetDefaultPolicy()
	resolvedConfig, _ := ResolveRegisterConfig("", config)
	effective := buildEffectiveConfig(resolvedConfig, false, "")

	// Check that path_inputs section exists
	if effective.PathInputs.MaxBytes == 0 {
		t.Error("Expected PathInputs.MaxBytes to be set")
	}

	// Verify default safety settings
	if !effective.PathInputs.DenyAbsolute {
		t.Error("Expected DenyAbsolute to be true by default")
	}
	if !effective.PathInputs.DenyDotDot {
		t.Error("Expected DenyDotDot to be true by default")
	}
	if !effective.PathInputs.DenyUNC {
		t.Error("Expected DenyUNC to be true by default")
	}
	if !effective.PathInputs.DenyDriveLetter {
		t.Error("Expected DenyDriveLetter to be true by default")
	}
}

func TestEffectiveConfig_StableKeyOrder(t *testing.T) {
	config := GetDefaultPolicy()
	resolvedConfig, _ := ResolveRegisterConfig("", config)
	effective := buildEffectiveConfig(resolvedConfig, true, ".deespec/etc/policies/register_policy.yaml")

	// Marshal to JSON multiple times and verify order
	var outputs []string
	for i := 0; i < 3; i++ {
		output, err := json.MarshalIndent(effective, "", "  ")
		if err != nil {
			t.Fatalf("Failed to marshal JSON: %v", err)
		}
		outputs = append(outputs, string(output))
	}

	// All outputs should be identical (stable order)
	for i := 1; i < len(outputs); i++ {
		if outputs[i] != outputs[0] {
			t.Errorf("Output %d differs from output 0, key order is not stable", i)
		}
	}

	// Verify meta is first in output
	if !strings.HasPrefix(strings.TrimSpace(outputs[0]), `{
  "meta":`) {
		t.Error("Expected 'meta' to be the first key in JSON output")
	}
}

func TestEffectiveConfig_MetadataFields(t *testing.T) {
	config := GetDefaultPolicy()
	resolvedConfig, _ := ResolveRegisterConfig("", config)

	// Test with policy file found
	effective := buildEffectiveConfig(resolvedConfig, true, ".deespec/etc/policies/test.yaml")

	// Check metadata
	if !effective.Meta.PolicyFileFound {
		t.Error("Expected PolicyFileFound to be true")
	}
	if effective.Meta.PolicyPath == "" {
		t.Error("Expected PolicyPath to be set")
	}
	if len(effective.Meta.SourcePriority) != 3 {
		t.Errorf("Expected 3 source priority items, got %d", len(effective.Meta.SourcePriority))
	}
	if effective.Meta.SourcePriority[0] != "cli" {
		t.Error("Expected first priority to be 'cli'")
	}
	if effective.Meta.Version == "" {
		t.Error("Expected Version to be set")
	}
	if effective.Meta.TsUTC == "" {
		t.Error("Expected TsUTC to be set")
	}
}

func TestRunPrintEffectiveConfig_Integration(t *testing.T) {
	// Create a test policy
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "register_policy.yaml")
	policyContent := `
id:
  pattern: "^TEST-[0-9]{3}$"
  max_len: 10
title:
  max_len: 50
  deny_empty: true
collision:
  default_mode: "replace"
  suffix_limit: 5
journal:
  record_source: true
  record_input_bytes: true
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to write test policy: %v", err)
	}

	// Mock GetPolicyPath to return our test policy
	oldGetPolicyPath := GetPolicyPath
	GetPolicyPath = func() string { return policyPath }
	defer func() { GetPolicyPath = oldGetPolicyPath }()

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

	// Run with CLI override
	err := runPrintEffectiveConfig("suffix", "json", false, false)

	wOut.Close()
	wErr.Close()
	stdout, _ := io.ReadAll(rOut)
	stderr, _ := io.ReadAll(rErr)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check stderr
	if !strings.Contains(string(stderr), "INFO: policy loaded") {
		t.Errorf("Expected policy loaded message in stderr, got: %s", stderr)
	}

	// Parse and verify output
	var effective EffectiveConfig
	if err := json.Unmarshal(stdout, &effective); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// CLI override should take precedence
	if effective.Collision.DefaultMode != "suffix" {
		t.Errorf("Expected CLI override 'suffix', got: %s", effective.Collision.DefaultMode)
	}

	// Policy values should be used
	if effective.ID.MaxLen != 10 {
		t.Errorf("Expected policy max_len 10, got: %d", effective.ID.MaxLen)
	}
	if effective.Collision.SuffixLimit != 5 {
		t.Errorf("Expected policy suffix_limit 5, got: %d", effective.Collision.SuffixLimit)
	}
	if !effective.Journal.RecordSource {
		t.Error("Expected journal.record_source to be true from policy")
	}
}

