package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLoadRegisterPolicy(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid policy file",
			content: `id:
  pattern: "^[A-Z0-9-]{1,64}$"
  max_len: 64
title:
  max_len: 200
  deny_empty: true
labels:
  pattern: "^[a-z0-9-]+$"
  max_count: 32
  warn_on_duplicates: true
input:
  max_kb: 64
slug:
  nfkc: true
  lowercase: true
  allow: "a-z0-9-"
  max_runes: 60
  fallback: "spec"
  windows_reserved_suffix: "-x"
  trim_trailing_dot_space: true
path:
  base_dir: ".deespec/specs/sbi"
  max_bytes: 240
  deny_symlink_components: true
  enforce_containment: true
collision:
  default_mode: "error"
  suffix_limit: 99
journal:
  record_source: true
  record_input_bytes: true
logging:
  stderr_level_default: "info"
`,
			expectError: false,
		},
		{
			name: "unknown field",
			content: `id:
  pattern: "^[A-Z0-9-]{1,64}$"
  max_len: 64
  unknown_field: "test"
`,
			expectError: true,
			errorMsg:    "field unknown_field not found",
		},
		{
			name: "invalid ID pattern",
			content: `id:
  pattern: "[invalid"
  max_len: 64
`,
			expectError: true,
			errorMsg:    "invalid ID pattern",
		},
		{
			name: "ID max_len exceeds safe limit",
			content: `id:
  pattern: "^[A-Z0-9-]{1,64}$"
  max_len: 300
`,
			expectError: true,
			errorMsg:    "ID max_len exceeds safe limit",
		},
		{
			name: "invalid collision mode",
			content: `collision:
  default_mode: "invalid_mode"
`,
			expectError: true,
			errorMsg:    "invalid collision default_mode",
		},
		{
			name: "invalid logging level",
			content: `logging:
  stderr_level_default: "debug"
`,
			expectError: true,
			errorMsg:    "invalid stderr_level_default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			policyPath := filepath.Join(tmpDir, "register_policy.yaml")
			if err := os.WriteFile(policyPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test policy: %v", err)
			}

			// Load policy
			policy, err := LoadRegisterPolicy(policyPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s' but got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if policy == nil {
					t.Errorf("Expected policy to be loaded")
				}
			}
		})
	}
}

func TestGetDefaultPolicy(t *testing.T) {
	policy := GetDefaultPolicy()

	// Check defaults match expected values
	if policy.ID.Pattern != "^[A-Z0-9-]{1,64}$" {
		t.Errorf("Expected ID pattern '^[A-Z0-9-]{1,64}$', got '%s'", policy.ID.Pattern)
	}
	if policy.ID.MaxLen != 64 {
		t.Errorf("Expected ID max_len 64, got %d", policy.ID.MaxLen)
	}
	if policy.Title.MaxLen != 200 {
		t.Errorf("Expected Title max_len 200, got %d", policy.Title.MaxLen)
	}
	if !policy.Title.DenyEmpty {
		t.Errorf("Expected Title deny_empty to be true")
	}
	if policy.Labels.Pattern != "^[a-z0-9-]+$" {
		t.Errorf("Expected Labels pattern '^[a-z0-9-]+$', got '%s'", policy.Labels.Pattern)
	}
	if policy.Labels.MaxCount != 32 {
		t.Errorf("Expected Labels max_count 32, got %d", policy.Labels.MaxCount)
	}
	if !policy.Labels.WarnOnDuplicates {
		t.Errorf("Expected Labels warn_on_duplicates to be true")
	}
	if policy.Input.MaxKB != 64 {
		t.Errorf("Expected Input max_kb 64, got %d", policy.Input.MaxKB)
	}
	if !policy.Slug.NFKC {
		t.Errorf("Expected Slug NFKC to be true")
	}
	if !policy.Slug.Lowercase {
		t.Errorf("Expected Slug lowercase to be true")
	}
	if policy.Slug.Allow != "a-z0-9-" {
		t.Errorf("Expected Slug allow 'a-z0-9-', got '%s'", policy.Slug.Allow)
	}
	if policy.Slug.MaxRunes != 60 {
		t.Errorf("Expected Slug max_runes 60, got %d", policy.Slug.MaxRunes)
	}
	if policy.Slug.Fallback != "spec" {
		t.Errorf("Expected Slug fallback 'spec', got '%s'", policy.Slug.Fallback)
	}
	if policy.Slug.WindowsReservedSuffix != "-x" {
		t.Errorf("Expected Slug windows_reserved_suffix '-x', got '%s'", policy.Slug.WindowsReservedSuffix)
	}
	if !policy.Slug.TrimTrailingDotSpace {
		t.Errorf("Expected Slug trim_trailing_dot_space to be true")
	}
	if policy.Path.BaseDir != ".deespec/specs/sbi" {
		t.Errorf("Expected Path base_dir '.deespec/specs/sbi', got '%s'", policy.Path.BaseDir)
	}
	if policy.Path.MaxBytes != 240 {
		t.Errorf("Expected Path max_bytes 240, got %d", policy.Path.MaxBytes)
	}
	if !policy.Path.DenySymlinkComponents {
		t.Errorf("Expected Path deny_symlink_components to be true")
	}
	if !policy.Path.EnforceContainment {
		t.Errorf("Expected Path enforce_containment to be true")
	}
	if policy.Collision.DefaultMode != "error" {
		t.Errorf("Expected Collision default_mode 'error', got '%s'", policy.Collision.DefaultMode)
	}
	if policy.Collision.SuffixLimit != 99 {
		t.Errorf("Expected Collision suffix_limit 99, got %d", policy.Collision.SuffixLimit)
	}
	if policy.Journal.RecordSource {
		t.Errorf("Expected Journal record_source to be false")
	}
	if policy.Journal.RecordInputBytes {
		t.Errorf("Expected Journal record_input_bytes to be false")
	}
	if policy.Logging.StderrLevelDefault != "info" {
		t.Errorf("Expected Logging stderr_level_default 'info', got '%s'", policy.Logging.StderrLevelDefault)
	}
}

func TestResolveRegisterConfig(t *testing.T) {
	tests := []struct {
		name           string
		cliCollision   string
		policy         *RegisterPolicy
		expectedMode   string
		expectedMaxLen int
		expectError    bool
	}{
		{
			name:         "CLI overrides policy collision mode",
			cliCollision: "suffix",
			policy: &RegisterPolicy{Collision: struct {
				DefaultMode string `yaml:"default_mode"`
				SuffixLimit int    `yaml:"suffix_limit"`
			}{DefaultMode: "error", SuffixLimit: 99}},
			expectedMode:   "suffix",
			expectedMaxLen: 0,
			expectError:    false,
		},
		{
			name:         "Policy collision mode used when CLI empty",
			cliCollision: "",
			policy: &RegisterPolicy{Collision: struct {
				DefaultMode string `yaml:"default_mode"`
				SuffixLimit int    `yaml:"suffix_limit"`
			}{DefaultMode: "replace", SuffixLimit: 99}},
			expectedMode:   "replace",
			expectedMaxLen: 0,
			expectError:    false,
		},
		{
			name:         "Default policy used when nil",
			cliCollision: "",
			policy:       nil,
			expectedMode: "error",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ResolveRegisterConfig(tt.cliCollision, tt.policy)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if config == nil {
					t.Errorf("Expected config to be resolved")
					return
				}
				if config.CollisionMode != tt.expectedMode {
					t.Errorf("Expected collision mode '%s', got '%s'", tt.expectedMode, config.CollisionMode)
				}
			}
		})
	}
}

func TestShouldLog(t *testing.T) {
	tests := []struct {
		name        string
		configLevel string
		msgLevel    string
		shouldLog   bool
	}{
		{"info level logs info", "info", "info", true},
		{"info level logs warn", "info", "warn", true},
		{"info level logs error", "info", "error", true},
		{"warn level skips info", "warn", "info", false},
		{"warn level logs warn", "warn", "warn", true},
		{"warn level logs error", "warn", "error", true},
		{"error level skips info", "error", "info", false},
		{"error level skips warn", "error", "warn", false},
		{"error level logs error", "error", "error", true},
		{"unknown message level logs", "info", "debug", true},
		{"unknown config level defaults to info", "debug", "info", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ResolvedConfig{
				StderrLevel: tt.configLevel,
			}

			result := config.ShouldLog(tt.msgLevel)
			if result != tt.shouldLog {
				t.Errorf("Expected ShouldLog(%s) = %v for config level %s, got %v",
					tt.msgLevel, tt.shouldLog, tt.configLevel, result)
			}
		})
	}
}

func TestPolicyIntegration(t *testing.T) {
	// Create a custom policy
	policy := &RegisterPolicy{}
	policy.ID.Pattern = "^TEST-[0-9]{3}$"
	policy.ID.MaxLen = 8
	policy.Title.MaxLen = 50
	policy.Title.DenyEmpty = true
	policy.Labels.Pattern = "^test-[a-z]+$"
	policy.Labels.MaxCount = 5
	policy.Labels.WarnOnDuplicates = true
	policy.Input.MaxKB = 10
	policy.Slug.NFKC = true
	policy.Slug.Lowercase = true
	policy.Slug.Allow = "a-z0-9"
	policy.Slug.MaxRunes = 20
	policy.Slug.Fallback = "test"
	policy.Slug.WindowsReservedSuffix = "-test"
	policy.Slug.TrimTrailingDotSpace = true
	policy.Path.BaseDir = ".deespec/test"
	policy.Path.MaxBytes = 100
	policy.Path.DenySymlinkComponents = true
	policy.Path.EnforceContainment = true
	policy.Collision.DefaultMode = "suffix"
	policy.Collision.SuffixLimit = 10
	policy.Journal.RecordSource = true
	policy.Journal.RecordInputBytes = true
	policy.Logging.StderrLevelDefault = "warn"

	// Resolve config
	config, err := ResolveRegisterConfig("", policy)
	if err != nil {
		t.Fatalf("Failed to resolve config: %v", err)
	}

	// Verify pattern compilation
	if config.IDPattern == nil {
		t.Errorf("Expected ID pattern to be compiled")
	} else if !config.IDPattern.MatchString("TEST-123") {
		t.Errorf("Expected pattern to match 'TEST-123'")
	}

	// Verify labels pattern
	if config.LabelsPattern == nil {
		t.Errorf("Expected Labels pattern to be compiled")
	} else if !config.LabelsPattern.MatchString("test-abc") {
		t.Errorf("Expected pattern to match 'test-abc'")
	}

	// Verify input size conversion
	if config.InputMaxBytes != 10*1024 {
		t.Errorf("Expected InputMaxBytes to be 10240, got %d", config.InputMaxBytes)
	}

	// Verify all fields transferred correctly
	if config.IDMaxLen != 8 {
		t.Errorf("Expected IDMaxLen 8, got %d", config.IDMaxLen)
	}
	if config.TitleMaxLen != 50 {
		t.Errorf("Expected TitleMaxLen 50, got %d", config.TitleMaxLen)
	}
	if !config.TitleDenyEmpty {
		t.Errorf("Expected TitleDenyEmpty to be true")
	}
	if config.LabelsMaxCount != 5 {
		t.Errorf("Expected LabelsMaxCount 5, got %d", config.LabelsMaxCount)
	}
	if !config.LabelsWarnOnDuplicates {
		t.Errorf("Expected LabelsWarnOnDuplicates to be true")
	}
	if !config.SlugNFKC {
		t.Errorf("Expected SlugNFKC to be true")
	}
	if !config.SlugLowercase {
		t.Errorf("Expected SlugLowercase to be true")
	}
	if config.SlugAllow != "a-z0-9" {
		t.Errorf("Expected SlugAllow 'a-z0-9', got '%s'", config.SlugAllow)
	}
	if config.SlugMaxRunes != 20 {
		t.Errorf("Expected SlugMaxRunes 20, got %d", config.SlugMaxRunes)
	}
	if config.SlugFallback != "test" {
		t.Errorf("Expected SlugFallback 'test', got '%s'", config.SlugFallback)
	}
	if config.SlugWindowsReservedSuffix != "-test" {
		t.Errorf("Expected SlugWindowsReservedSuffix '-test', got '%s'", config.SlugWindowsReservedSuffix)
	}
	if !config.SlugTrimTrailingDotSpace {
		t.Errorf("Expected SlugTrimTrailingDotSpace to be true")
	}
	if config.PathBaseDir != ".deespec/test" {
		t.Errorf("Expected PathBaseDir '.deespec/test', got '%s'", config.PathBaseDir)
	}
	if config.PathMaxBytes != 100 {
		t.Errorf("Expected PathMaxBytes 100, got %d", config.PathMaxBytes)
	}
	if !config.PathDenySymlinkComponents {
		t.Errorf("Expected PathDenySymlinkComponents to be true")
	}
	if !config.PathEnforceContainment {
		t.Errorf("Expected PathEnforceContainment to be true")
	}
	if config.CollisionMode != "suffix" {
		t.Errorf("Expected CollisionMode 'suffix', got '%s'", config.CollisionMode)
	}
	if config.SuffixLimit != 10 {
		t.Errorf("Expected SuffixLimit 10, got %d", config.SuffixLimit)
	}
	if !config.JournalRecordSource {
		t.Errorf("Expected JournalRecordSource to be true")
	}
	if !config.JournalRecordInputBytes {
		t.Errorf("Expected JournalRecordInputBytes to be true")
	}
	if config.StderrLevel != "warn" {
		t.Errorf("Expected StderrLevel 'warn', got '%s'", config.StderrLevel)
	}
}

func TestValidateSpecWithConfig(t *testing.T) {
	tests := []struct {
		name        string
		spec        RegisterSpec
		config      *ResolvedConfig
		expectError bool
		errorMsg    string
		warnCount   int
	}{
		{
			name: "valid spec",
			spec: RegisterSpec{
				ID:     "TEST-001",
				Title:  "Test Spec",
				Labels: []string{"test", "demo"},
			},
			config: &ResolvedConfig{
				IDPattern:              regexp.MustCompile("^TEST-[0-9]{3}$"),
				IDMaxLen:               10,
				TitleMaxLen:            50,
				TitleDenyEmpty:         true,
				LabelsPattern:          regexp.MustCompile("^[a-z]+$"),
				LabelsMaxCount:         5,
				LabelsWarnOnDuplicates: true,
			},
			expectError: false,
			warnCount:   0,
		},
		{
			name: "invalid ID pattern",
			spec: RegisterSpec{
				ID:    "INVALID",
				Title: "Test Spec",
			},
			config: &ResolvedConfig{
				IDPattern:      regexp.MustCompile("^TEST-[0-9]{3}$"),
				TitleDenyEmpty: true,
			},
			expectError: true,
			errorMsg:    "invalid id format",
		},
		{
			name: "ID too long",
			spec: RegisterSpec{
				ID:    "TEST-001-EXTRA",
				Title: "Test Spec",
			},
			config: &ResolvedConfig{
				IDMaxLen:       10,
				TitleDenyEmpty: true,
			},
			expectError: true,
			errorMsg:    "exceeds maximum",
		},
		{
			name: "title truncated",
			spec: RegisterSpec{
				ID:    "TEST-001",
				Title: "This is a very long title that will be truncated",
			},
			config: &ResolvedConfig{
				TitleMaxLen: 20,
			},
			expectError: true,
			errorMsg:    "exceeds maximum",
		},
		{
			name: "duplicate labels",
			spec: RegisterSpec{
				ID:     "TEST-001",
				Title:  "Test",
				Labels: []string{"test", "test", "demo"},
			},
			config: &ResolvedConfig{
				LabelsWarnOnDuplicates: true,
			},
			expectError: false,
			warnCount:   1,
		},
		{
			name: "invalid label pattern",
			spec: RegisterSpec{
				ID:     "TEST-001",
				Title:  "Test",
				Labels: []string{"test-123", "demo"},
			},
			config: &ResolvedConfig{
				LabelsPattern: regexp.MustCompile("^[a-z]+$"),
			},
			expectError: true,
			errorMsg:    "invalid label format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := tt.spec // Make a copy
			result := validateSpecWithConfig(&spec, tt.config)

			if tt.expectError {
				if result.Err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(result.Err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s' but got: %v", tt.errorMsg, result.Err)
				}
			} else {
				if result.Err != nil {
					t.Errorf("Unexpected error: %v", result.Err)
				}
				if len(result.Warnings) != tt.warnCount {
					t.Errorf("Expected %d warnings, got %d: %v", tt.warnCount, len(result.Warnings), result.Warnings)
				}
			}
		})
	}
}
