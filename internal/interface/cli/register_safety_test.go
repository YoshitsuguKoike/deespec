package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugifyTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		// Basic ASCII
		{"simple", "Simple Title", "simple-title"},
		{"with numbers", "Title 123", "title-123"},
		{"with hyphens", "Already-Hyphenated", "already-hyphenated"},

		// Non-ASCII and NFKC normalization
		{"japanese", "日本語タイトル", "spec"}, // All non-ASCII removed
		{"mixed", "Test 日本語 Title", "test-title"},
		{"full-width", "ＦＵＬＬ　ＷＩＤＴＨ", "full-width"}, // NFKC normalizes

		// Special characters
		{"special chars", "Title!@#$%^&*()", "title"},
		{"spaces", "Multiple   Spaces", "multiple-spaces"},
		{"leading/trailing", "  Trimmed  ", "trimmed"},

		// Edge cases
		{"empty", "", "spec"},
		{"all special", "!@#$%", "spec"},
		{"dots and spaces", "name.  ", "name"},

		// Windows reserved names
		{"con", "CON", "con-x"},
		{"prn", "prn", "prn-x"},
		{"aux", "AUX", "aux-x"},
		{"com1", "COM1", "com1-x"},
		{"lpt9", "lpt9", "lpt9-x"},

		// Length limits (60 runes max)
		{"long title", strings.Repeat("a", 70), strings.Repeat("a", 60)},
		{"long with dash", strings.Repeat("a", 65) + "-test", strings.Repeat("a", 60)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GetDefaultPolicy()
			resolvedConfig, _ := ResolveRegisterConfig("", config)
			result := slugifyTitleWithConfig(tt.title, resolvedConfig)
			if result != tt.expected {
				t.Errorf("slugifyTitle(%q) = %q; want %q", tt.title, result, tt.expected)
			}
		})
	}
}

func TestIsWindowsReserved(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"con lower", "con", true},
		{"CON upper", "CON", true},
		{"CoN mixed", "CoN", true},
		{"prn", "prn", true},
		{"aux", "aux", true},
		{"nul", "nul", true},
		{"com1", "com1", true},
		{"com9", "com9", true},
		{"lpt1", "lpt1", true},
		{"lpt9", "lpt9", true},
		{"not reserved", "console", false},
		{"not reserved com", "com", false},
		{"not reserved com10", "com10", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWindowsReserved(tt.input)
			if result != tt.expected {
				t.Errorf("isWindowsReserved(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildSafeSpecPath(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		title     string
		wantErr   bool
		errMsg    string
		checkPath func(string) bool
	}{
		{
			name:  "normal path",
			id:    "SBI-TEST-001",
			title: "Test Spec",
			checkPath: func(p string) bool {
				return p == ".deespec/specs/sbi/SBI-TEST-001_test-spec"
			},
		},
		{
			name:    "traversal attempt with ..",
			id:      "SBI-TEST",
			title:   "../../../etc/passwd",
			wantErr: false, // Should be sanitized, not error
			checkPath: func(p string) bool {
				// .. gets removed in slugification
				return strings.Contains(p, "etc-passwd") && !strings.Contains(p, "..")
			},
		},
		{
			name:  "windows reserved name",
			id:    "SBI-TEST",
			title: "CON",
			checkPath: func(p string) bool {
				return strings.HasSuffix(p, "_con-x")
			},
		},
		{
			name:  "very long path",
			id:    strings.Repeat("A", 50),
			title: strings.Repeat("B", 200),
			checkPath: func(p string) bool {
				// Should be truncated to fit MaxPathBytes
				return len([]byte(p)) <= MaxPathBytes
			},
		},
		{
			name:  "path with slashes removed",
			id:    "SBI-TEST",
			title: "path/with/slashes",
			checkPath: func(p string) bool {
				// Slashes become dashes in slugification
				expected := ".deespec/specs/sbi/SBI-TEST_path-with-slashes"
				return p == expected
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GetDefaultPolicy()
			resolvedConfig, _ := ResolveRegisterConfig("", config)
			path, err := buildSafeSpecPathWithConfig(tt.id, tt.title, resolvedConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildSafeSpecPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("buildSafeSpecPath() error = %v, want containing %q", err, tt.errMsg)
			}
			if tt.checkPath != nil && !tt.checkPath(path) {
				t.Errorf("buildSafeSpecPath() path = %q, failed check", path)
			}
		})
	}
}

func TestIsPathSafe(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(baseDir, 0755)

	tests := []struct {
		name     string
		base     string
		path     string
		expected bool
	}{
		{
			name:     "path within base",
			base:     baseDir,
			path:     filepath.Join(baseDir, "subdir", "file"),
			expected: true,
		},
		{
			name:     "path equals base",
			base:     baseDir,
			path:     baseDir,
			expected: true,
		},
		{
			name:     "path outside base",
			base:     baseDir,
			path:     tmpDir,
			expected: false,
		},
		{
			name:     "path with ..",
			base:     baseDir,
			path:     filepath.Join(baseDir, "..", "other"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPathSafe(tt.base, tt.path)
			if result != tt.expected {
				t.Errorf("isPathSafe(%q, %q) = %v; want %v", tt.base, tt.path, result, tt.expected)
			}
		})
	}
}

func TestCheckForSymlinks(t *testing.T) {
	// Create temp directory structure
	tmpDirRaw := t.TempDir()
	// Resolve any symlinks in the temp dir path itself (e.g., /var -> /private/var on macOS)
	tmpDir, _ := filepath.EvalSymlinks(tmpDirRaw)
	realDir := filepath.Join(tmpDir, "real")
	os.MkdirAll(realDir, 0755)

	// Create a symlink
	linkDir := filepath.Join(tmpDir, "link")
	os.Symlink(realDir, linkDir)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "path without symlinks",
			path:    filepath.Join(tmpDir, "real", "subdir"),
			wantErr: false,
		},
		{
			name:    "path with symlink",
			path:    filepath.Join(linkDir, "subdir"),
			wantErr: true,
		},
		{
			name:    "non-existent path",
			path:    filepath.Join(tmpDir, "nonexistent", "path"),
			wantErr: false, // Non-existent paths are OK
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkForSymlinks(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkForSymlinks(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestResolveCollision(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, ".deespec", "specs", "sbi")
	os.MkdirAll(baseDir, 0755)

	existingPath := filepath.Join(baseDir, "existing")
	os.MkdirAll(existingPath, 0755)

	tests := []struct {
		name        string
		path        string
		mode        string
		expectErr   bool
		checkResult func(string, string) bool
	}{
		{
			name:      "error mode - no collision",
			path:      filepath.Join(baseDir, "new"),
			mode:      CollisionError,
			expectErr: false,
		},
		{
			name:      "error mode - with collision",
			path:      existingPath,
			mode:      CollisionError,
			expectErr: true,
		},
		{
			name:      "suffix mode - no collision",
			path:      filepath.Join(baseDir, "new2"),
			mode:      CollisionSuffix,
			expectErr: false,
		},
		{
			name:      "suffix mode - with collision",
			path:      existingPath,
			mode:      CollisionSuffix,
			expectErr: false,
			checkResult: func(path, warning string) bool {
				return strings.HasSuffix(path, "_2") && strings.Contains(warning, "suffix")
			},
		},
		{
			name:      "replace mode - no collision",
			path:      filepath.Join(baseDir, "new3"),
			mode:      CollisionReplace,
			expectErr: false,
		},
		{
			name:      "replace mode - with collision",
			path:      existingPath,
			mode:      CollisionReplace,
			expectErr: false,
			checkResult: func(path, warning string) bool {
				// Should have removed and recreated
				return path == existingPath && strings.Contains(warning, "replaced")
			},
		},
		{
			name:      "invalid mode",
			path:      filepath.Join(baseDir, "any"),
			mode:      "invalid",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Recreate existing path if it was removed
			if tt.path == existingPath && tt.mode == CollisionReplace {
				os.MkdirAll(existingPath, 0755)
			}

			config := GetDefaultPolicy()
			config.Collision.DefaultMode = tt.mode
			resolvedConfig, _ := ResolveRegisterConfig("", config)
			resultPath, warning, err := resolveCollisionWithConfig(tt.path, resolvedConfig)
			if (err != nil) != tt.expectErr {
				t.Errorf("resolveCollision() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if tt.checkResult != nil && !tt.checkResult(resultPath, warning) {
				t.Errorf("resolveCollision() = (%q, %q), failed check", resultPath, warning)
			}
		})
	}
}

func TestCollisionSuffixExhaustion(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, ".deespec", "specs", "sbi")
	os.MkdirAll(baseDir, 0755)

	basePath := filepath.Join(baseDir, "test")

	// Create base and many suffix variations
	os.MkdirAll(basePath, 0755)
	for i := 2; i <= 10; i++ {
		os.MkdirAll(fmt.Sprintf("%s_%d", basePath, i), 0755)
	}

	// Should find _11
	config := GetDefaultPolicy()
	config.Collision.DefaultMode = CollisionSuffix
	resolvedConfig, _ := ResolveRegisterConfig("", config)
	resultPath, warning, err := resolveCollisionWithConfig(basePath, resolvedConfig)
	if err != nil {
		t.Fatalf("resolveCollision() unexpected error: %v", err)
	}

	if !strings.HasSuffix(resultPath, "_11") {
		t.Errorf("expected path ending with _11, got %q", resultPath)
	}

	if !strings.Contains(warning, "suffix") {
		t.Errorf("expected warning about suffix, got %q", warning)
	}
}
