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
	var printEffectiveConfig bool
	var format string
	var compact bool
	var redactSecrets bool

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new SBI specification",
		Long:  "Register a new SBI specification from stdin or file input",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle print-effective-config first (no side effects)
			if printEffectiveConfig {
				return runPrintEffectiveConfig(onCollision, format, compact, redactSecrets)
			}
			return runRegisterWithFlags(cmd, args, stdinFlag, fileFlag, onCollision)
		},
	}

	cmd.Flags().BoolVar(&stdinFlag, "stdin", false, "Read input from stdin")
	cmd.Flags().StringVar(&fileFlag, "file", "", "Read input from file")
	cmd.Flags().StringVar(&onCollision, "on-collision", CollisionError, "How to handle path collisions (error|suffix|replace)")
	cmd.Flags().BoolVar(&printEffectiveConfig, "print-effective-config", false, "Print the effective configuration and exit")
	cmd.Flags().StringVar(&format, "format", "json", "Output format for effective config (json|yaml)")
	cmd.Flags().BoolVar(&compact, "compact", false, "Use compact format (single line JSON)")
	cmd.Flags().BoolVar(&redactSecrets, "redact-secrets", true, "Redact sensitive values in output")

	return cmd
}

var exitFunc = os.Exit

func runRegisterWithFlags(cmd *cobra.Command, args []string, stdinFlag bool, fileFlag string, onCollision string) error {
	// Load policy
	policy, err := LoadRegisterPolicy(GetPolicyPath())
	if err != nil {
		// Policy file error is fatal
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{},
			Error:    fmt.Sprintf("failed to load policy: %v", err),
		}
		fmt.Fprintf(os.Stderr, "ERROR: failed to load policy: %v\n", err)
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Resolve configuration with precedence
	config, err := ResolveRegisterConfig(onCollision, policy)
	if err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{},
			Error:    fmt.Sprintf("failed to resolve config: %v", err),
		}
		fmt.Fprintf(os.Stderr, "ERROR: failed to resolve config: %v\n", err)
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Initialize stderr logger with level control
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
		if config.ShouldLog("error") {
			stderrLog.Println("ERROR: cannot specify both --stdin and --file")
		}
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
		if config.ShouldLog("error") {
			stderrLog.Println("ERROR: must specify either --stdin or --file")
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Track input source for journal
	if stdinFlag {
		config.InputSource = "stdin"
	} else {
		config.InputSource = "file"
	}

	// Read input with policy-based size limit
	input, err := readInputWithConfig(stdinFlag, fileFlag, config)
	if err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       "",
			SpecPath: "",
			Warnings: []string{},
			Error:    fmt.Sprintf("failed to read input: %v", err),
		}
		if config.ShouldLog("error") {
			stderrLog.Printf("ERROR: failed to read input: %v\n", err)
		}
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
		if config.ShouldLog("error") {
			stderrLog.Printf("ERROR: invalid input: %v\n", err)
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Validate specification with policy-based validation
	validationResult := validateSpecWithConfig(&spec, config)
	if validationResult.Err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       spec.ID,
			SpecPath: "",
			Warnings: []string{},
			Error:    validationResult.Err.Error(),
		}
		if config.ShouldLog("error") {
			stderrLog.Printf("ERROR: %v\n", validationResult.Err)
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Collision mode is already validated in config resolution

	// Calculate and validate spec path with config
	specPath, err := buildSafeSpecPathWithConfig(spec.ID, spec.Title, config)
	if err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       spec.ID,
			SpecPath: "",
			Warnings: []string{},
			Error:    fmt.Sprintf("failed to build spec path: %v", err),
		}
		if config.ShouldLog("error") {
			stderrLog.Printf("ERROR: %v\n", err)
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Resolve collision using config mode
	finalPath, collisionWarning, err := resolveCollisionWithConfig(specPath, config)
	if err != nil {
		result := RegisterResult{
			OK:       false,
			ID:       spec.ID,
			SpecPath: "",
			Warnings: []string{},
			Error:    err.Error(),
		}
		if config.ShouldLog("error") {
			stderrLog.Printf("ERROR: %v\n", err)
		}
		printJSONLine(result)
		exitFunc(1)
		return nil
	}

	// Add collision warning if any
	if collisionWarning != "" {
		validationResult.Warnings = append(validationResult.Warnings, collisionWarning)
		if config.ShouldLog("warn") {
			stderrLog.Printf("WARN: %s\n", collisionWarning)
		}
	}

	// Log warnings to stderr (respecting log level)
	for _, warning := range validationResult.Warnings {
		if warning != collisionWarning && config.ShouldLog("warn") { // Don't log twice
			stderrLog.Printf("WARN: %s\n", warning)
		}
	}

	// Log success info (respecting log level)
	if config.ShouldLog("info") {
		stderrLog.Printf("INFO: spec_path resolved: %s\n", finalPath)
	}

	// Success result
	result := RegisterResult{
		OK:       true,
		ID:       spec.ID,
		SpecPath: finalPath,
		Warnings: validationResult.Warnings,
	}
	printJSONLine(result)

	// Append to journal with spec_path and optional metadata
	if err := appendToJournalWithConfig(&spec, &result, config); err != nil {
		if config.ShouldLog("warn") {
			stderrLog.Printf("WARN: failed to append to journal: %v\n", err)
		}
	}

	return nil
}

// isTestMode allows overriding path validation for tests
var isTestMode = false

// readInputWithConfig reads input with policy-based validation
func readInputWithConfig(stdinFlag bool, fileFlag string, config *ResolvedConfig) ([]byte, error) {
	var input []byte
	var err error

	if stdinFlag {
		config.InputSource = "stdin"
		maxSize := config.InputMaxBytes
		if maxSize == 0 {
			maxSize = MaxInputSize
		}
		input, err = io.ReadAll(io.LimitReader(os.Stdin, int64(maxSize)+1))
		if err != nil {
			return nil, err
		}
	} else {
		config.InputSource = "file"
		// Check for dangerous paths (skip in test mode)
		if !isTestMode && (strings.Contains(fileFlag, "..") || (filepath.IsAbs(fileFlag) && !strings.Contains(fileFlag, "/tmp/"))) {
			return nil, fmt.Errorf("invalid file path: paths with '..' or absolute paths outside /tmp are not allowed")
		}

		file, err := os.Open(fileFlag)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		maxSize := config.InputMaxBytes
		if maxSize == 0 {
			maxSize = MaxInputSize
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
		maxSize = MaxInputSize
	}
	if len(input) > maxSize {
		return nil, fmt.Errorf("input size exceeds limit of %d bytes", maxSize)
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

// validateSpecWithConfig validates spec using policy configuration
func validateSpecWithConfig(spec *RegisterSpec, config *ResolvedConfig) ValidationResult {
	result := ValidationResult{
		Warnings: []string{},
	}

	// Validate ID
	if spec.ID == "" {
		result.Err = fmt.Errorf("id is required")
		return result
	}
	if config.IDPattern != nil && !config.IDPattern.MatchString(spec.ID) {
		result.Err = fmt.Errorf("invalid id format: must match %s", config.IDPattern.String())
		return result
	}
	if config.IDMaxLen > 0 && len(spec.ID) > config.IDMaxLen {
		result.Err = fmt.Errorf("id length exceeds maximum of %d characters", config.IDMaxLen)
		return result
	}

	// Validate Title
	if spec.Title == "" && config.TitleDenyEmpty {
		result.Err = fmt.Errorf("title is required and cannot be empty")
		return result
	}
	if config.TitleMaxLen > 0 && len(spec.Title) > config.TitleMaxLen {
		result.Warnings = append(result.Warnings, fmt.Sprintf("title truncated from %d to %d characters", len(spec.Title), config.TitleMaxLen))
		spec.Title = spec.Title[:config.TitleMaxLen]
	}

	// Validate Labels
	if spec.Labels != nil {
		// Check count limit
		if config.LabelsMaxCount > 0 && len(spec.Labels) > config.LabelsMaxCount {
			result.Warnings = append(result.Warnings, fmt.Sprintf("labels count exceeds %d (%d)", config.LabelsMaxCount, len(spec.Labels)))
			spec.Labels = spec.Labels[:config.LabelsMaxCount]
		}

		// Validate and deduplicate labels
		labelMap := make(map[string]bool)
		var validLabels []string
		for _, label := range spec.Labels {
			if config.LabelsPattern != nil && !config.LabelsPattern.MatchString(label) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("invalid label removed: %s (must match %s)", label, config.LabelsPattern.String()))
				continue
			}
			if labelMap[label] {
				if config.LabelsWarnOnDuplicates {
					result.Warnings = append(result.Warnings, fmt.Sprintf("duplicate label removed: %s", label))
				}
				continue
			}
			labelMap[label] = true
			validLabels = append(validLabels, label)
		}
		spec.Labels = validLabels
	}

	return result
}

// slugifyTitleWithConfig converts a title to a safe slug using policy settings
func slugifyTitleWithConfig(title string, config *ResolvedConfig) string {
	// Apply NFKC normalization if enabled
	if config.SlugNFKC {
		title = norm.NFKC.String(title)
	}

	// Convert to lowercase if enabled
	if config.SlugLowercase {
		title = strings.ToLower(title)
	}

	// Build allowed character set from policy
	allowed := make(map[rune]bool)
	// Parse the allow string as character ranges and literals
	// e.g., "a-z0-9-" means a-z range, 0-9 range, and literal hyphen
	allowStr := config.SlugAllow
	for i := 0; i < len(allowStr); i++ {
		if i+2 < len(allowStr) && allowStr[i+1] == '-' && allowStr[i+2] != '-' {
			// This is a range like a-z or 0-9
			start := allowStr[i]
			end := allowStr[i+2]
			for c := start; c <= end; c++ {
				allowed[rune(c)] = true
			}
			i += 2 // Skip the range
		} else {
			// This is a literal character
			allowed[rune(allowStr[i])] = true
		}
	}

	// Build slug with only allowed characters
	var b strings.Builder
	var lastDash bool
	for _, r := range title {
		if allowed[r] {
			b.WriteRune(r)
			lastDash = false
		} else {
			// Replace non-allowed with dash
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	slug := b.String()
	slug = strings.Trim(slug, "-")

	// Use fallback if empty
	if slug == "" {
		slug = config.SlugFallback
		if slug == "" {
			slug = "spec"
		}
	}

	// Check for Windows reserved names and add suffix if configured
	if config.SlugWindowsReservedSuffix != "" && isWindowsReserved(slug) {
		slug += config.SlugWindowsReservedSuffix
	}

	// Remove trailing dots and spaces if configured
	if config.SlugTrimTrailingDotSpace {
		slug = strings.TrimRight(slug, ". ")
	}

	// Length limit based on policy
	maxRunes := config.SlugMaxRunes
	if maxRunes == 0 {
		maxRunes = MaxSlugLength
	}
	if utf8.RuneCountInString(slug) > maxRunes {
		runes := []rune(slug)
		slug = string(runes[:maxRunes])
		slug = strings.Trim(slug, "-")
	}

	return slug
}

// isWindowsReserved checks if a name is a Windows reserved name
func isWindowsReserved(name string) bool {
	lower := strings.ToLower(name)
	return windowsReservedNames[lower]
}

// buildSafeSpecPathWithConfig builds and validates a safe spec path using policy
func buildSafeSpecPathWithConfig(id, title string, config *ResolvedConfig) (string, error) {
	slug := slugifyTitleWithConfig(title, config)
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

	// Build the full path using base directory from policy
	basePath := config.PathBaseDir
	fullPath := filepath.Join(basePath, dirName)

	// Validate path length
	maxBytes := config.PathMaxBytes
	if maxBytes == 0 {
		maxBytes = MaxPathBytes
	}
	if len([]byte(fullPath)) > maxBytes {
		// Try to shorten the slug
		maxSlugBytes := maxBytes - len([]byte(basePath)) - len([]byte(id)) - 2 // -2 for "_/"
		if maxSlugBytes < 10 {
			return "", fmt.Errorf("path would exceed maximum length of %d bytes", maxBytes)
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

	// Additional safety checks based on policy
	if config.PathEnforceContainment && !isPathSafe(basePath, fullPath) {
		return "", fmt.Errorf("path traversal detected")
	}

	// Check for symlinks if policy requires
	if config.PathDenySymlinkComponents {
		if err := checkForSymlinks(filepath.Dir(fullPath)); err != nil {
			return "", err
		}
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

// resolveCollisionWithConfig handles path collisions according to policy configuration
func resolveCollisionWithConfig(path string, config *ResolvedConfig) (string, string, error) {
	// First check for symlinks if policy requires
	if config.PathDenySymlinkComponents {
		if err := checkForSymlinks(filepath.Dir(path)); err != nil {
			return "", "", err
		}
	}

	switch config.CollisionMode {
	case CollisionError:
		if _, err := os.Stat(path); err == nil {
			return "", "", fmt.Errorf("path already exists: %s", path)
		}
		return path, "", nil

	case CollisionSuffix:
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, "", nil
		}

		// Try with suffixes up to the configured limit
		limit := config.SuffixLimit
		if limit == 0 {
			limit = 99
		}
		for i := 2; i <= limit; i++ {
			newPath := fmt.Sprintf("%s_%d", path, i)
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				warning := fmt.Sprintf("collision resolved with suffix: _%d", i)
				return newPath, warning, nil
			}
		}
		return "", "", fmt.Errorf("exhausted suffix attempts (tried _2 through _%d)", limit)

	case CollisionReplace:
		if _, err := os.Stat(path); err == nil {
			// For replace mode, verify it contains our expected base path from policy
			if !strings.Contains(path, config.PathBaseDir) {
				return "", "", fmt.Errorf("refusing to remove path outside %s: %s", config.PathBaseDir, path)
			}

			// Remove the existing directory
			if err := os.RemoveAll(path); err != nil {
				return "", "", fmt.Errorf("failed to remove existing path: %v", err)
			}
			return path, "replaced existing directory", nil
		}
		return path, "", nil

	default:
		return "", "", fmt.Errorf("invalid collision mode: %s", config.CollisionMode)
	}
}

func printJSONLine(result RegisterResult) {
	output, _ := json.Marshal(result)
	os.Stdout.Write(output)
	os.Stdout.Write([]byte("\n"))
}

// appendToJournalWithConfig appends a registration event to the journal with policy settings
func appendToJournalWithConfig(spec *RegisterSpec, result *RegisterResult, config *ResolvedConfig) error {
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
				"id":        spec.ID,
				"title":     spec.Title,
				"labels":    spec.Labels,
				"ok":        result.OK,
				"warnings":  result.Warnings,
				"spec_path": result.SpecPath,
			},
		},
	}

	// Add optional fields based on policy
	if config.JournalRecordSource {
		entry["input_source"] = config.InputSource
	}
	if config.JournalRecordInputBytes {
		entry["input_bytes"] = config.InputBytes
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