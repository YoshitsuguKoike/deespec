package register

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
)

// Collision constants for backward compatibility with dry_run.go
const (
	CollisionError   = "error"
	CollisionSuffix  = "suffix"
	CollisionReplace = "replace"
)

// Additional constants for backward compatibility
const (
	MaxIDLength    = 64
	MaxTitleLength = 200
	MaxLabelCount  = 32
	MaxInputSize   = 64 * 1024
	MaxPathBytes   = 240
	MaxSlugLength  = 60
)

// isTestMode allows overriding path validation for tests
var isTestMode = false

// Windows reserved names
var windowsReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true,
	"com9": true, "lpt1": true, "lpt2": true, "lpt3": true,
	"lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true,
	"lpt8": true, "lpt9": true,
}

// RegisterSpec for backward compatibility (CLI layer type)
type RegisterSpec struct {
	ID     string   `yaml:"id" json:"id"`
	Title  string   `yaml:"title" json:"title"`
	Labels []string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// ValidationResult for backward compatibility
type ValidationResult struct {
	Warnings []string
	Err      error
}

// readInputWithConfig for backward compatibility with dry_run.go
func readInputWithConfig(stdinFlag bool, fileFlag string, config *ResolvedConfig) ([]byte, error) {
	var input []byte
	var err error

	if stdinFlag {
		config.InputSource = "stdin"
		maxSize := config.InputMaxBytes
		if maxSize == 0 {
			maxSize = 64 * 1024
		}
		input, err = io.ReadAll(io.LimitReader(os.Stdin, int64(maxSize)+1))
		if err != nil {
			return nil, err
		}
	} else {
		config.InputSource = "file"
		file, err := os.Open(fileFlag)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		maxSize := config.InputMaxBytes
		if maxSize == 0 {
			maxSize = 64 * 1024
		}
		input, err = io.ReadAll(io.LimitReader(file, int64(maxSize)+1))
		if err != nil {
			return nil, err
		}
	}

	if len(input) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	config.InputBytes = len(input)

	maxSize := config.InputMaxBytes
	if maxSize == 0 {
		maxSize = 64 * 1024
	}
	if len(input) > maxSize {
		return nil, fmt.Errorf("input size exceeds limit of %d bytes", maxSize)
	}

	return input, nil
}

// decodeStrict for backward compatibility
func decodeStrict(input []byte, spec *RegisterSpec, filePath string) error {
	isJSON := false
	if filePath != "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		isJSON = ext == ".json"
	} else {
		trimmed := bytes.TrimSpace(input)
		if len(trimmed) > 0 && trimmed[0] == '{' {
			isJSON = true
		}
	}

	if isJSON {
		decoder := json.NewDecoder(bytes.NewReader(input))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(spec); err != nil {
			return err
		}
	} else {
		decoder := yaml.NewDecoder(bytes.NewReader(input))
		decoder.KnownFields(true)
		if err := decoder.Decode(spec); err != nil {
			return err
		}
	}

	return nil
}

// validateSpecWithConfig for backward compatibility
func validateSpecWithConfig(spec *RegisterSpec, config *ResolvedConfig) ValidationResult {
	result := ValidationResult{
		Warnings: []string{},
	}

	// ID validation
	if spec.ID == "" {
		result.Err = fmt.Errorf("id is required")
		return result
	}

	// Check ID against pattern if configured
	if config.IDPattern != nil {
		if !config.IDPattern.MatchString(spec.ID) {
			result.Err = fmt.Errorf("invalid id format: does not match pattern %s", config.IDPattern.String())
			return result
		}
	} else {
		// Fallback to basic pattern
		idPattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !idPattern.MatchString(spec.ID) {
			result.Err = fmt.Errorf("invalid ID format")
			return result
		}
	}

	// Check ID length if configured
	if config.IDMaxLen > 0 && len(spec.ID) > config.IDMaxLen {
		result.Err = fmt.Errorf("id length %d exceeds maximum %d", len(spec.ID), config.IDMaxLen)
		return result
	}

	// Title validation
	if config.TitleDenyEmpty && spec.Title == "" {
		result.Err = fmt.Errorf("title is required")
		return result
	}

	// Check title length if configured
	if config.TitleMaxLen > 0 && len(spec.Title) > config.TitleMaxLen {
		result.Err = fmt.Errorf("title length %d exceeds maximum %d", len(spec.Title), config.TitleMaxLen)
		return result
	}

	// Label validation
	for _, label := range spec.Labels {
		if label == "" {
			result.Warnings = append(result.Warnings, "empty label found and ignored")
			continue
		}

		// Check label against pattern if configured
		if config.LabelsPattern != nil {
			if !config.LabelsPattern.MatchString(label) {
				result.Err = fmt.Errorf("invalid label format: '%s' does not match pattern %s", label, config.LabelsPattern.String())
				return result
			}
		}
	}

	// Check for duplicate labels if configured
	if config.LabelsWarnOnDuplicates {
		seen := make(map[string]bool)
		for _, label := range spec.Labels {
			if label == "" {
				continue
			}
			if seen[label] {
				result.Warnings = append(result.Warnings, fmt.Sprintf("duplicate label: %s", label))
			}
			seen[label] = true
		}
	}

	// Check label count if configured
	if config.LabelsMaxCount > 0 && len(spec.Labels) > config.LabelsMaxCount {
		result.Err = fmt.Errorf("number of labels (%d) exceeds maximum %d", len(spec.Labels), config.LabelsMaxCount)
		return result
	}

	return result
}

// buildSafeSpecPathWithConfig for backward compatibility
func buildSafeSpecPathWithConfig(id, title string, config *ResolvedConfig) (string, error) {
	// Use the full slugification logic
	slug := slugifyTitleWithConfig(title, config)

	baseName := id + "_" + slug
	path := filepath.Join(config.PathBaseDir, baseName)

	if len(path) > config.PathMaxBytes {
		return "", fmt.Errorf("spec path exceeds maximum length")
	}

	return path, nil
}

// isPathSafe for backward compatibility
func isPathSafe(base, path string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// toDTO converts CLI RegisterSpec to DTO RegisterSpec
func toDTO(spec *RegisterSpec) *dto.RegisterSpec {
	return &dto.RegisterSpec{
		ID:     spec.ID,
		Title:  spec.Title,
		Labels: spec.Labels,
	}
}

// slugifyTitleWithConfig for backward compatibility
func slugifyTitleWithConfig(title string, config *ResolvedConfig) string {
	// Apply NFKC normalization if configured
	slug := title
	if config.SlugNFKC {
		slug = norm.NFKC.String(slug)
	}

	// Convert to lowercase if configured
	if config.SlugLowercase {
		slug = strings.ToLower(slug)
	}

	// Replace non-allowed characters with hyphens
	// Build pattern from config.SlugAllow (e.g., "a-z0-9-" becomes "[^a-z0-9-]")
	allowPattern := config.SlugAllow
	if allowPattern == "" {
		allowPattern = "a-z0-9-"
	}
	pattern := regexp.MustCompile(fmt.Sprintf(`[^%s]`, allowPattern))
	slug = pattern.ReplaceAllString(slug, "-")

	// Collapse multiple hyphens
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")

	// Trim leading/trailing hyphens, dots, and spaces if configured
	if config.SlugTrimTrailingDotSpace {
		slug = strings.Trim(slug, "-. ")
	} else {
		slug = strings.Trim(slug, "-")
	}

	// Use fallback if empty
	if slug == "" {
		fallback := config.SlugFallback
		if fallback == "" {
			fallback = "spec"
		}
		slug = fallback
	}

	// Check for Windows reserved names and add suffix if needed
	if isWindowsReserved(slug) {
		suffix := config.SlugWindowsReservedSuffix
		if suffix == "" {
			suffix = "-x"
		}
		slug = slug + suffix
	}

	// Limit length
	maxRunes := config.SlugMaxRunes
	if maxRunes == 0 {
		maxRunes = 60
	}
	if len([]rune(slug)) > maxRunes {
		slug = string([]rune(slug)[:maxRunes])
	}

	return slug
}

// checkForSymlinks for backward compatibility
func checkForSymlinks(path string) error {
	cleanPath := filepath.Clean(path)
	parts := strings.Split(cleanPath, string(os.PathSeparator))

	// Handle absolute paths (start with /)
	current := ""
	if filepath.IsAbs(cleanPath) {
		current = string(os.PathSeparator)
	}

	for i, part := range parts {
		if part == "" {
			continue // Skip empty parts (can happen with absolute paths)
		}

		// Build path incrementally
		if i == 0 && filepath.IsAbs(cleanPath) {
			current = filepath.Join(current, part)
		} else if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}

		// Check if this component exists
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				// Non-existent paths are OK (they don't contain symlinks yet)
				continue
			}
			return fmt.Errorf("failed to check path component %s: %w", current, err)
		}

		// Check if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path contains symlink at %s", current)
		}
	}

	return nil
}

// resolveCollisionWithConfig for backward compatibility
func resolveCollisionWithConfig(path string, config *ResolvedConfig) (string, string, error) {
	// Validate collision mode first (before checking path existence)
	validModes := map[string]bool{CollisionError: true, CollisionSuffix: true, CollisionReplace: true}
	if config.CollisionMode != "" && !validModes[config.CollisionMode] {
		return "", "", fmt.Errorf("invalid collision mode: %s (must be error|suffix|replace)", config.CollisionMode)
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return path, "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to check path: %w", err)
	}

	switch config.CollisionMode {
	case CollisionError:
		return "", "", fmt.Errorf("path already exists: %s", path)
	case CollisionReplace:
		warning := fmt.Sprintf("replaced existing spec at %s", path)
		return path, warning, nil
	case CollisionSuffix:
		limit := config.SuffixLimit
		if limit == 0 {
			limit = 100
		}
		for i := 2; i <= limit+1; i++ {
			// Use underscore for suffix to match test expectations
			newPath := fmt.Sprintf("%s_%d", path, i)

			_, err := os.Stat(newPath)
			if os.IsNotExist(err) {
				warning := fmt.Sprintf("path collision resolved with suffix: %s", newPath)
				return newPath, warning, nil
			}
		}
		return "", "", fmt.Errorf("could not resolve collision: exceeded suffix limit")
	default:
		return "", "", fmt.Errorf("invalid collision mode: %s", config.CollisionMode)
	}
}

// isWindowsReserved for backward compatibility
func isWindowsReserved(name string) bool {
	lower := strings.ToLower(name)
	base := strings.Split(lower, ".")[0]
	return windowsReservedNames[base]
}

// validateID for backward compatibility
func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	if len(id) > MaxIDLength {
		return fmt.Errorf("id exceeds maximum length of %d", MaxIDLength)
	}
	// Default policy requires uppercase letters, numbers, hyphens, and underscores only (no lowercase)
	idPattern := regexp.MustCompile(`^[A-Z0-9_-]+$`)
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid id format: must contain only uppercase letters, numbers, hyphens, and underscores")
	}
	return nil
}

// validateTitle for backward compatibility
func validateTitle(title string) error {
	if title == "" {
		return fmt.Errorf("title is required")
	}
	if len(title) > MaxTitleLength {
		return fmt.Errorf("title exceeds maximum length of %d", MaxTitleLength)
	}
	return nil
}

// validateLabels for backward compatibility
func validateLabels(labels []string) (warnings []string, err error) {
	// Default policy pattern: lowercase letters, numbers, and hyphens only
	labelPattern := regexp.MustCompile(`^[a-z0-9-]+$`)

	// Check label count - exceeding MaxLabelCount is a warning, not an error
	if len(labels) > MaxLabelCount {
		warnings = append(warnings, fmt.Sprintf("labels count exceeds maximum: %d > %d", len(labels), MaxLabelCount))
	}

	seen := make(map[string]bool)
	for _, label := range labels {
		if label == "" {
			warnings = append(warnings, "empty label found and ignored")
			continue
		}

		// Check label format according to default policy
		if !labelPattern.MatchString(label) {
			return warnings, fmt.Errorf("invalid label format: '%s' must contain only lowercase letters, numbers, and hyphens", label)
		}

		if seen[label] {
			warnings = append(warnings, fmt.Sprintf("duplicate label: %s", label))
		}
		seen[label] = true
	}

	return warnings, nil
}
