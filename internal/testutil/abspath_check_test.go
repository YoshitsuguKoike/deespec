package testutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test_NoAbsolutePathsInTests verifies that no test files use absolute paths
func Test_NoAbsolutePathsInTests(t *testing.T) {
	// Get the root directory of the project
	rootDir := "../.." // Relative to internal/testutil

	// Walk through all Go test files
	violations := []string{}
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and .git directories
		if info.IsDir() && (info.Name() == "vendor" || info.Name() == ".git") {
			return filepath.SkipDir
		}

		// Only check Go test files
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip files that test path validation logic
		if strings.Contains(path, "incomplete_test.go") {
			// This file tests path validation and needs absolute paths
			return nil
		}

		// Skip this file itself
		if strings.Contains(path, "abspath_check_test.go") {
			return nil
		}

		// Check for absolute path patterns
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		contentStr := string(content)
		lines := strings.Split(contentStr, "\n")

		for i, line := range lines {
			lineNum := i + 1

			// Check for filepath.Abs calls
			if strings.Contains(line, "filepath.Abs(") {
				violations = append(violations, formatViolation(path, lineNum, line, "filepath.Abs() call detected"))
			}

			// Check for hardcoded absolute paths (Unix-style)
			// Skip comments and strings that are obviously examples
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") {
				// Look for paths starting with /
				if containsAbsolutePath(line) {
					violations = append(violations, formatViolation(path, lineNum, line, "Absolute path literal detected"))
				}
			}

			// Check for filepath.Join with absolute paths
			if strings.Contains(line, "filepath.Join(") && containsAbsolutePath(line) {
				violations = append(violations, formatViolation(path, lineNum, line, "filepath.Join with absolute path"))
			}
		}

		// Also check using AST parsing for more accurate detection
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			// If we can't parse, skip AST checks but continue
			return nil
		}

		// Walk the AST looking for problematic patterns
		ast.Inspect(node, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				// Check string literals
				if x.Kind == token.STRING {
					value := x.Value
					// Remove quotes
					if len(value) >= 2 {
						value = value[1 : len(value)-1]
					}
					// Check if it's an absolute path
					if strings.HasPrefix(value, "/") && len(value) > 1 && !isAllowedPath(value) {
						pos := fset.Position(x.Pos())
						violations = append(violations, formatViolation(path, pos.Line, value, "Absolute path in string literal"))
					}
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk directory tree: %v", err)
	}

	// Report violations
	if len(violations) > 0 {
		t.Errorf("Found %d absolute path violations in test files:\n", len(violations))
		for _, v := range violations {
			t.Error(v)
		}
		t.Fatal("Tests must use relative paths only. Use testutil.NewTestWorkspace() and relative paths.")
	}
}

// containsAbsolutePath checks if a line contains an absolute path pattern
func containsAbsolutePath(line string) bool {
	// Skip if it's in a comment
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
		return false
	}

	// Look for patterns that indicate absolute paths
	patterns := []string{
		`"/"`,      // Root directory
		`"/Users/`, // Mac user paths
		`"/home/`,  // Linux user paths
		`"/etc/`,   // System paths
		`"/var/`,   // System paths
		`"/opt/`,   // System paths
		`"/usr/`,   // System paths
		`"C:\\`,    // Windows paths
		`"D:\\`,    // Windows paths
	}

	for _, pattern := range patterns {
		if strings.Contains(line, pattern) {
			// Make sure it's not in a comment or test data
			if !strings.Contains(line, "//") || strings.Index(line, pattern) < strings.Index(line, "//") {
				return true
			}
		}
	}

	return false
}

// isAllowedPath checks if a path is allowed (e.g., test data or examples)
func isAllowedPath(path string) bool {
	allowedPrefixes := []string{
		"/dev/null",
		"/tmp/", // Allowed for explicit temp file tests
	}

	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// Allow paths in test data or golden files
	if strings.Contains(path, "testdata") || strings.Contains(path, ".golden") {
		return true
	}

	return false
}

// formatViolation formats a violation message
func formatViolation(file string, line int, content, reason string) string {
	// Make path relative for cleaner output
	relPath := file
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, file); err == nil {
			relPath = rel
		}
	}

	content = strings.TrimSpace(content)
	if len(content) > 80 {
		content = content[:80] + "..."
	}

	return formatString("%s:%d - %s\n  > %s", relPath, line, reason, content)
}

// formatString is a helper to format strings (avoiding fmt import in test utilities)
func formatString(format string, args ...interface{}) string {
	result := format
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			result = strings.Replace(result, "%s", v, 1)
		case int:
			result = strings.Replace(result, "%d", formatInt(v), 1)
		}
	}
	return result
}

// formatInt converts an int to string
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}

	var result string
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}

	if negative {
		result = "-" + result
	}

	return result
}
