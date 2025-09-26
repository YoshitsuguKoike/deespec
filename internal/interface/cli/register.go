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

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Validation constants
const (
	MaxIDLength    = 64
	MaxTitleLength = 200
	MaxLabelCount  = 32
	MaxInputSize   = 64 * 1024 // 64KB
)

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

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new SBI specification",
		Long:  "Register a new SBI specification from stdin or file input",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegisterWithFlags(cmd, args, stdinFlag, fileFlag)
		},
	}

	cmd.Flags().BoolVar(&stdinFlag, "stdin", false, "Read input from stdin")
	cmd.Flags().StringVar(&fileFlag, "file", "", "Read input from file")

	return cmd
}

var exitFunc = os.Exit

func runRegisterWithFlags(cmd *cobra.Command, args []string, stdinFlag bool, fileFlag string) error {
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

	// Calculate spec path
	specPath := calculateSpecPath(spec.ID, spec.Title)

	// Log warnings to stderr
	for _, warning := range validationResult.Warnings {
		stderrLog.Printf("WARN: %s\n", warning)
	}

	// Success result
	result := RegisterResult{
		OK:       true,
		ID:       spec.ID,
		SpecPath: specPath,
		Warnings: validationResult.Warnings,
	}
	printJSONLine(result)

	// Append to journal
	if err := appendToJournal(spec.ID, true, validationResult.Warnings); err != nil {
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

func calculateSpecPath(id, title string) string {
	// Slugify title: lowercase and replace spaces with hyphens
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove non-alphanumeric characters except hyphens
	slug = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(slug, "")
	// Remove leading/trailing hyphens and collapse multiple hyphens
	slug = regexp.MustCompile(`^-+|-+$`).ReplaceAllString(slug, "")
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")

	return fmt.Sprintf(".deespec/specs/sbi/%s_%s", id, slug)
}

func printJSONLine(result RegisterResult) {
	output, _ := json.Marshal(result)
	os.Stdout.Write(output)
	os.Stdout.Write([]byte("\n"))
}

// appendToJournal appends a registration event to the journal
func appendToJournal(id string, ok bool, warnings []string) error {
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
				"type":     "register",
				"id":       id,
				"ok":       ok,
				"warnings": warnings,
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