package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
)

// Validation constants
const (
	MaxIDLength    = 64
	MaxTitleLength = 200
	MaxLabelCount  = 32
	MaxInputSize   = 64 * 1024 // 64KB
	MaxPathBytes   = 240       // Max path length in bytes
	MaxSlugLength  = 60        // Max slug length in runes
)

// Collision handling modes
const (
	CollisionError   = "error"
	CollisionSuffix  = "suffix"
	CollisionReplace = "replace"
)

// Windows reserved names (case-insensitive)
var windowsReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true,
	"com9": true, "lpt1": true, "lpt2": true, "lpt3": true,
	"lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true,
	"lpt8": true, "lpt9": true,
}

// RegisterSpec represents the input specification for registration
type RegisterSpec struct {
	ID     string   `yaml:"id" json:"id"`
	Title  string   `yaml:"title" json:"title"`
	Labels []string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// RegisterResult represents the JSON output for registration
type RegisterResult struct {
	OK       bool     `json:"ok"`
	ID       string   `json:"id"`
	SpecPath string   `json:"spec_path"`
	Warnings []string `json:"warnings"`
	Error    string   `json:"error,omitempty"`
}

// ValidationResult holds validation warnings and errors
type ValidationResult struct {
	Warnings []string
	Err      error
}

// NewRegisterCommand creates the register command
func NewRegisterCommand() *cobra.Command {
	var stdinFlag bool
	var fileFlag string
	var onCollision string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new SBI specification",
		Long:  "Register a new SBI specification from stdin or file input",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegisterWithFlags(cmd, args, stdinFlag, fileFlag, onCollision)
		},
	}

	cmd.Flags().BoolVar(&stdinFlag, "stdin", false, "Read input from stdin")
	cmd.Flags().StringVar(&fileFlag, "file", "", "Read input from file")
	cmd.Flags().StringVar(&onCollision, "on-collision", CollisionError, "How to handle path collisions (error|suffix|replace)")

	return cmd
}

var exitFunc = os.Exit

func runRegisterWithFlags(cmd *cobra.Command, args []string, stdinFlag bool, fileFlag string, onCollision string) error {
	// Initialize stderr logger
	stderrLog := log.New(os.Stderr, "", 0)

	// Check exclusive flags
	if stdinFlag && fileFlag != "" {
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{},
			Error:    "cannot specify both --stdin and --file",
		}
		stderrLog.Println("ERROR: cannot specify both --stdin and --file")
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	if !stdinFlag && fileFlag == "" {
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{},
			Error:    "must specify either --stdin or --file",
		}
		stderrLog.Println("ERROR: must specify either --stdin or --file")
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Read input
	input, err := readInput(stdinFlag, fileFlag)
	if err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{},
			Error:    fmt.Sprintf("failed to read input: %v", err),
		}
		stderrLog.Printf("ERROR: failed to read input: %v\n", err)
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Decode input
	var spec RegisterSpec
	if err := decodeStrict(input, &spec, fileFlag); err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{},
			Error:    fmt.Sprintf("invalid input: %v", err),
		}
		stderrLog.Printf("ERROR: invalid input: %v\n", err)
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Validate specification with enhanced validation
	validationResult := validateSpecEnhanced(&spec)
	if validationResult.Err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       spec.ID,
			SpecPath: "",
			Warnings: []string{},
			Error:    validationResult.Err.Error(),
		}
		stderrLog.Printf("ERROR: %v\n", validationResult.Err)
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Validate collision mode
	if onCollision != CollisionError && onCollision != CollisionSuffix && onCollision != CollisionReplace {
		result := RegisterResult{
			OK:       false,
			ID:       spec.ID,
			SpecPath: "",
			Warnings: []string{},
			Error:    fmt.Sprintf("invalid --on-collision value: %s (must be error|suffix|replace)", onCollision),
		}
		stderrLog.Printf("ERROR: invalid collision mode: %s\n", onCollision)
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Calculate and validate spec path
	specPath, err := buildSafeSpecPath(spec.ID, spec.Title)
	if err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       spec.ID,
			SpecPath: "",
			Warnings: []string{},
			Error:    fmt.Sprintf("failed to build spec path: %v", err),
		}
		stderrLog.Printf("ERROR: %v\n", err)
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Resolve collision
	finalPath, collisionWarning, err := resolveCollision(specPath, onCollision)
	if err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       spec.ID,
			SpecPath: "",
			Warnings: []string{},
			Error:    err.Error(),
		}
		stderrLog.Printf("ERROR: %v\n", err)
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Add collision warning if any
	if collisionWarning != "" {
		validationResult.Warnings = append(validationResult.Warnings, collisionWarning)
		stderrLog.Printf("WARN: %s\n", collisionWarning)
	}

	// Log warnings to stderr
	for _, warning := range validationResult.Warnings {
		if warning != collisionWarning { // Don't log twice
			stderrLog.Printf("WARN: %s\n", warning)
		}
	}

	// Log success info
	stderrLog.Printf("INFO: spec_path resolved: %s\n", finalPath)

	// Success result
	result := RegisterResult{
		OK:       true,
		ID:       spec.ID,
		SpecPath: finalPath,
		Warnings: validationResult.Warnings,
	}
	printJSONLine(result)

	// Append to journal with spec_path
	if err := appendToJournalWithPath(spec.ID, true, validationResult.Warnings, finalPath); err != nil {
		stderrLog.Printf("WARN: failed to append to journal: %v\n", err)
	}

	return nil
}

// isTestMode allows overriding path validation for tests
var isTestMode = false

func readInput(useStdin bool, filePath string) ([]byte, error) {
	var input []byte
	var err error

	if useStdin {
		input, err = io.ReadAll(io.LimitReader(os.Stdin, MaxInputSize+1))
		if err != nil {
			return nil, err
		}
	} else {
		// Check for dangerous paths (skip in test mode)
		if !isTestMode && (strings.Contains(filePath, "..") || (filepath.IsAbs(filePath) && !strings.Contains(filePath, "/tmp/"))) {
			return nil, fmt.Errorf("invalid file path: paths with '..' or absolute paths outside /tmp are not allowed")
		}

		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		input, err = io.ReadAll(io.LimitReader(file, MaxInputSize+1))
		if err != nil {
			return nil, err
		}
	}

	if len(input) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	if len(input) > MaxInputSize {
		return nil, fmt.Errorf("input size exceeds limit of %d bytes", MaxInputSize)
	}

	return input, nil
}

func decodeStrict(input []byte, spec *RegisterSpec, filePath string) error {
	// Determine format from file extension or try both
	isJSON := false
	if filePath != "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		isJSON = ext == ".json"
	} else {
		// Try to detect format by checking if it starts with {
		trimmed := bytes.TrimSpace(input)
		if len(trimmed) > 0 && trimmed[0] == '{' {
			isJSON = true
		}
	}

	if isJSON {
		// JSON decode with strict mode
		decoder := json.NewDecoder(bytes.NewReader(input))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(spec); err != nil {
			return err
		}
	} else {
		// YAML decode with strict mode
		decoder := yaml.NewDecoder(bytes.NewReader(input))
		decoder.KnownFields(true)
		if err := decoder.Decode(spec); err != nil {
			return err
		}
	}

	return nil
}

func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	// New pattern: ^[A-Z0-9-]{1,64}$
	idPattern := regexp.MustCompile(`^[A-Z0-9-]{1,64}$`)
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid id format: must match ^[A-Z0-9-]{1,64}$")
	}

	if len(id) > MaxIDLength {
		return fmt.Errorf("id length exceeds maximum of %d characters", MaxIDLength)
	}

	return nil
}

func validateTitle(title string) error {
	if title == "" {
		return fmt.Errorf("title is required and cannot be empty")
	}

	if len(title) > MaxTitleLength {
		return fmt.Errorf("title length exceeds maximum of %d characters", MaxTitleLength)
	}

	return nil
}

func validateLabels(labels []string) (warnings []string, err error) {
	// Labels are optional in SBI-REG-002
	if labels == nil {
		return nil, nil
	}

	// Check for non-array type is handled by YAML/JSON decoder

	labelPattern := regexp.MustCompile(`^[a-z0-9-]+$`)
	labelMap := make(map[string]bool)

	for _, label := range labels {
		if !labelPattern.MatchString(label) {
			return nil, fmt.Errorf("invalid label format '%s': must match ^[a-z0-9-]+$", label)
		}

		// Check for duplicates
		if labelMap[label] {
			warnings = append(warnings, fmt.Sprintf("duplicate label: %s", label))
		}
		labelMap[label] = true
	}

	// Check count limit
	if len(labels) > MaxLabelCount {
		warnings = append(warnings, fmt.Sprintf("labels count exceeds %d (%d)", MaxLabelCount, len(labels)))
	}

	return warnings, nil
}

func validateSpecEnhanced(spec *RegisterSpec) ValidationResult {
	result := ValidationResult{
		Warnings: []string{},
	}

	// Validate ID
	if err := validateID(spec.ID); err != nil {
		result.Err = err
		return result
	}

	// Validate Title
	if err := validateTitle(spec.Title); err != nil {
		result.Err = err
		return result
	}

	// Validate Labels
	warnings, err := validateLabels(spec.Labels)
	if err != nil {
		result.Err = err
		return result
	}
	result.Warnings = append(result.Warnings, warnings...)

	return result
}

// slugifyTitle converts a title to a safe slug following strict rules
func slugifyTitle(title string) string {
	// NFKC normalization
	title = norm.NFKC.String(title)
	title = strings.ToLower(title)

	// Build slug with only allowed characters
	var b strings.Builder
	var lastDash bool
	for _, r := range title {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		default:
			// Replace any other character with dash
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	slug := b.String()
	slug = strings.Trim(slug, "-")

	// Default if empty
	if slug == "" {
		slug = "spec"
	}

	// Check for Windows reserved names
	if isWindowsReserved(slug) {
		slug += "-x"
	}

	// Remove trailing dots and spaces (Windows compatibility)
	slug = strings.TrimRight(slug, ". ")

	// Length limit (60 runes)
	if utf8.RuneCountInString(slug) > MaxSlugLength {
		runes := []rune(slug)
		slug = string(runes[:MaxSlugLength])
		slug = strings.Trim(slug, "-")
	}

	return slug
}

// isWindowsReserved checks if a name is a Windows reserved name
func isWindowsReserved(name string) bool {
	lower := strings.ToLower(name)
	return windowsReservedNames[lower]
}

// buildSafeSpecPath builds and validates a safe spec path
func buildSafeSpecPath(id, title string) (string, error) {
	slug := slugifyTitle(title)
	dirName := fmt.Sprintf("%s_%s", id, slug)

	// Clean the directory name
	dirName = filepath.Clean(dirName)

	// Check for dangerous patterns
	if dirName == "." || dirName == ".." || strings.Contains(dirName, "..") {
		return "", fmt.Errorf("path traversal detected in directory name")
	}

	// Check for path separators (should not exist after slugification)
	if strings.ContainsAny(dirName, "/\\") {
		return "", fmt.Errorf("path separator detected in directory name")
	}

	// Build the full path
	basePath := ".deespec/specs/sbi"
	fullPath := filepath.Join(basePath, dirName)

	// Validate path length
	if len([]byte(fullPath)) > MaxPathBytes {
		// Try to shorten the slug
		maxSlugBytes := MaxPathBytes - len([]byte(basePath)) - len([]byte(id)) - 2 // -2 for "_/"
		if maxSlugBytes < 10 {
			return "", fmt.Errorf("path would exceed maximum length of %d bytes", MaxPathBytes)
		}
		// Truncate slug to fit
		for len([]byte(slug)) > maxSlugBytes && len(slug) > 1 {
			runes := []rune(slug)
			slug = string(runes[:len(runes)-1])
		}
		slug = strings.Trim(slug, "-")
		dirName = fmt.Sprintf("%s_%s", id, slug)
		fullPath = filepath.Join(basePath, dirName)
	}

	// Additional safety check: ensure path is within base directory
	if !isPathSafe(basePath, fullPath) {
		return "", fmt.Errorf("path traversal detected")
	}

	return fullPath, nil
}

// isPathSafe checks if a path is safely within the base directory
func isPathSafe(base, path string) bool {
	// Clean both paths
	base = filepath.Clean(base)
	path = filepath.Clean(path)

	// Get absolute paths
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Check if path is within base
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}

	// Check for .. in relative path
	if strings.Contains(rel, "..") {
		return false
	}

	return true
}

// checkForSymlinks checks if any component of the path is a symlink
func checkForSymlinks(path string) error {
	path = filepath.Clean(path)
	parts := strings.Split(path, string(filepath.Separator))

	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}

		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				// Path doesn't exist yet, that's OK
				return nil
			}
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink detected in path: %s", current)
		}
	}

	return nil
}

// resolveCollision handles path collisions according to the specified mode
func resolveCollision(path string, mode string) (string, string, error) {
	// First check for symlinks in the base path
	if err := checkForSymlinks(filepath.Dir(path)); err != nil {
		return "", "", err
	}

	switch mode {
	case CollisionError:
		if _, err := os.Stat(path); err == nil {
			return "", "", fmt.Errorf("path already exists: %s", path)
		}
		return path, "", nil

	case CollisionSuffix:
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, "", nil
		}

		// Try with suffixes _2 through _99
		for i := 2; i <= 99; i++ {
			newPath := fmt.Sprintf("%s_%d", path, i)
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				warning := fmt.Sprintf("collision resolved with suffix: _%d", i)
				return newPath, warning, nil
			}
		}
		return "", "", fmt.Errorf("exhausted suffix attempts (tried _2 through _99)")

	case CollisionReplace:
		if _, err := os.Stat(path); err == nil {
			// For replace mode, we trust the path was already validated by buildSafeSpecPath
			// Just verify it contains our expected base path
			if !strings.Contains(path, "specs/sbi/") {
				return "", "", fmt.Errorf("refusing to remove path outside specs/sbi: %s", path)
			}

			// Remove the existing directory
			if err := os.RemoveAll(path); err != nil {
				return "", "", fmt.Errorf("failed to remove existing path: %v", err)
			}
			return path, "replaced existing directory", nil
		}
		return path, "", nil

	default:
		return "", "", fmt.Errorf("invalid collision mode: %s", mode)
	}
}

func printJSONLine(result RegisterResult) {
	output, _ := json.Marshal(result)
	os.Stdout.Write(output)
	os.Stdout.Write([]byte("\n"))
}

// appendToJournalWithPath appends a registration event to the journal with spec_path
func appendToJournalWithPath(id string, ok bool, warnings []string, specPath string) error {
	// Create journal directory if it doesn't exist
	journalDir := ".deespec/var"
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		return fmt.Errorf("failed to create journal directory: %w", err)
	}

	journalPath := filepath.Join(journalDir, "journal.ndjson")

	// Read existing journal to get turn number
	turn := 0
	if file, err := os.Open(journalPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var entry map[string]interface{}
			if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
				if t, ok := entry["turn"].(float64); ok && int(t) > turn {
					turn = int(t)
				}
			}
		}
		turn++ // Increment for new entry
	}

	// Create journal entry with 7 required keys
	startTime := time.Now()
	entry := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       turn,
		"step":       "plan",  // Using "plan" as registration is part of planning phase
		"decision":   "PENDING",
		"elapsed_ms": int64(time.Since(startTime).Milliseconds()),
		"error":      "",
		"artifacts": []map[string]interface{}{
			{
				"type":      "register",
				"id":        id,
				"ok":        ok,
				"warnings":  warnings,
				"spec_path": specPath,
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal journal entry: %w", err)
	}

	// Atomic write using temp file and rename
	tmpPath := journalPath + ".tmp"

	// Open in append mode
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open temp journal file: %w", err)
	}

	// If original journal exists, copy its content first
	if origFile, err := os.Open(journalPath); err == nil {
		defer origFile.Close()
		if _, err := io.Copy(file, origFile); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to copy existing journal: %w", err)
		}
	}

	// Write new entry
	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write journal entry: %w", err)
	}

	if _, err := file.Write([]byte("\n")); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write newline: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close journal file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, journalPath); err != nil {
		return fmt.Errorf("failed to rename journal file: %w", err)
	}

	return nil
}