package common

import "time"

// ValidationIssue represents a single validation issue
type ValidationIssue struct {
	Type    string `json:"type"`    // "ok", "warn", "error"
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// FileResult represents validation result for a single file
type FileResult struct {
	File   string            `json:"file"`
	Issues []ValidationIssue `json:"issues"`
}

// ValidationResult represents the complete validation result
type ValidationResult struct {
	Version     int          `json:"version"`
	GeneratedAt string       `json:"generated_at"`
	Files       []FileResult `json:"files"`
	Summary     Summary      `json:"summary"`
}

// Summary contains validation statistics
type Summary struct {
	Files int `json:"files"`
	OK    int `json:"ok"`
	Warn  int `json:"warn"`
	Error int `json:"error"`
}

// NewValidationResult creates a new validation result
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Version:     1,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Files:       []FileResult{},
		Summary:     Summary{},
	}
}

// AddFileResult adds a file result and updates summary
func (vr *ValidationResult) AddFileResult(fileResult FileResult) {
	vr.Files = append(vr.Files, fileResult)
	vr.Summary.Files++

	hasError := false
	hasWarn := false

	for _, issue := range fileResult.Issues {
		switch issue.Type {
		case "error":
			hasError = true
		case "warn":
			hasWarn = true
		}
	}

	if hasError {
		vr.Summary.Error++
	} else if hasWarn {
		vr.Summary.Warn++
	} else {
		vr.Summary.OK++
	}
}

// ValidSteps defines the allowed step values
var ValidSteps = map[string]bool{
	"plan":      true,
	"implement": true,
	"test":      true,
	"review":    true,
	"done":      true,
}