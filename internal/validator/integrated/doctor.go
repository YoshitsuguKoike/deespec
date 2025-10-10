package integrated

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/validator/common"
	healthValidator "github.com/YoshitsuguKoike/deespec/internal/validator/health"
	journalValidator "github.com/YoshitsuguKoike/deespec/internal/validator/journal"
	stateValidator "github.com/YoshitsuguKoike/deespec/internal/validator/state"
)

// IntegratedReport represents the complete validation report from all components
type IntegratedReport struct {
	Version     int                                 `json:"version"`
	GeneratedAt string                              `json:"generated_at"`
	Components  map[string]*common.ValidationResult `json:"components"`
	Summary     IntegratedSummary                   `json:"summary"`
}

// IntegratedSummary contains aggregated validation statistics
type IntegratedSummary struct {
	Components int `json:"components"`
	OK         int `json:"ok"`
	Warn       int `json:"warn"`
	Error      int `json:"error"`
}

// ComponentStatus represents the status of each component for text output
type ComponentStatus struct {
	State   string
	Health  string
	Journal string
}

// DoctorConfig contains configuration for doctor validation
type DoctorConfig struct {
	BasePath    string
	StatePath   string
	HealthPath  string
	JournalPath string
}

// NewIntegratedReport creates a new integrated report
func NewIntegratedReport() *IntegratedReport {
	return &IntegratedReport{
		Version:     1,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Components:  make(map[string]*common.ValidationResult),
		Summary:     IntegratedSummary{},
	}
}

// RunIntegratedValidation performs all validations and returns an integrated report
func RunIntegratedValidation(config *DoctorConfig) (*IntegratedReport, error) {
	report := NewIntegratedReport()

	// Validate state
	stateResult, err := stateValidator.ValidateStateFile(config.StatePath)
	if err != nil && stateResult == nil {
		stateResult = common.NewValidationResult()
		fileResult := common.FileResult{
			File: filepath.Base(config.StatePath),
			Issues: []common.ValidationIssue{{
				Type:    "error",
				Message: fmt.Sprintf("validation error: %v", err),
			}},
		}
		stateResult.AddFileResult(fileResult)
	}
	report.Components["state"] = stateResult

	// Validate health
	healthResult, err := healthValidator.ValidateHealthFile(config.HealthPath)
	if err != nil && healthResult == nil {
		healthResult = common.NewValidationResult()
		fileResult := common.FileResult{
			File: filepath.Base(config.HealthPath),
			Issues: []common.ValidationIssue{{
				Type:    "error",
				Message: fmt.Sprintf("validation error: %v", err),
			}},
		}
		healthResult.AddFileResult(fileResult)
	}
	report.Components["health"] = healthResult

	// Validate journal
	journalResult := validateJournal(config.JournalPath)
	report.Components["journal"] = journalResult

	// Calculate integrated summary
	report.Summary = calculateIntegratedSummary(report.Components)

	return report, nil
}

// validateJournal converts journal validation to common format
func validateJournal(journalPath string) *common.ValidationResult {
	result := common.NewValidationResult()

	// Check if file exists
	file, err := os.Open(journalPath)
	if err != nil {
		fileResult := common.FileResult{
			File:   filepath.Base(journalPath),
			Issues: []common.ValidationIssue{},
		}

		if os.IsNotExist(err) {
			fileResult.Issues = append(fileResult.Issues, common.ValidationIssue{
				Type:    "warn",
				Message: "file not found",
			})
			result.Summary.Warn = 1
		} else {
			fileResult.Issues = append(fileResult.Issues, common.ValidationIssue{
				Type:    "error",
				Message: fmt.Sprintf("cannot read file: %v", err),
			})
			result.Summary.Error = 1
		}

		result.Files = append(result.Files, fileResult)
		result.Summary.Files = 1
		return result
	}
	defer file.Close()

	// Use journal validator
	validator := &journalValidator.Validator{
		// Set the file path for the validator
	}
	journalResult, err := validator.ValidateFile(file)
	if err != nil {
		fileResult := common.FileResult{
			File: filepath.Base(journalPath),
			Issues: []common.ValidationIssue{{
				Type:    "error",
				Message: fmt.Sprintf("validation error: %v", err),
			}},
		}
		result.AddFileResult(fileResult)
		return result
	}

	// Convert journal result to common format
	fileResult := common.FileResult{
		File:   filepath.Base(journalPath),
		Issues: []common.ValidationIssue{},
	}

	// Aggregate issues from all lines
	errorCount := 0
	warnCount := 0
	for _, line := range journalResult.Lines {
		for _, issue := range line.Issues {
			// Convert journal issue to common issue
			commonIssue := common.ValidationIssue{
				Type:    issue.Type,
				Field:   fmt.Sprintf("/line/%d", line.Line),
				Message: issue.Message,
			}
			fileResult.Issues = append(fileResult.Issues, commonIssue)

			switch issue.Type {
			case "error":
				errorCount++
			case "warn":
				warnCount++
			}
		}
	}

	// If no issues found, add OK status
	if len(fileResult.Issues) == 0 {
		fileResult.Issues = append(fileResult.Issues, common.ValidationIssue{
			Type:    "ok",
			Message: "all journal entries valid",
		})
		result.Summary.OK = 1
	} else if errorCount > 0 {
		result.Summary.Error = 1
	} else if warnCount > 0 {
		result.Summary.Warn = 1
	}

	result.Files = append(result.Files, fileResult)
	result.Summary.Files = 1

	return result
}

// calculateIntegratedSummary aggregates summaries from all components
func calculateIntegratedSummary(components map[string]*common.ValidationResult) IntegratedSummary {
	summary := IntegratedSummary{
		Components: len(components),
	}

	for _, component := range components {
		if component != nil {
			summary.OK += component.Summary.OK
			summary.Warn += component.Summary.Warn
			summary.Error += component.Summary.Error
		}
	}

	return summary
}

// GetComponentStatus determines the status string for each component
func GetComponentStatus(report *IntegratedReport) ComponentStatus {
	status := ComponentStatus{}

	if state := report.Components["state"]; state != nil {
		status.State = getStatusString(state.Summary)
	}

	if health := report.Components["health"]; health != nil {
		status.Health = getStatusString(health.Summary)
	}

	if journal := report.Components["journal"]; journal != nil {
		status.Journal = getStatusString(journal.Summary)
	}

	return status
}

func getStatusString(summary common.Summary) string {
	if summary.Error > 0 {
		return "error"
	} else if summary.Warn > 0 {
		return "warn"
	}
	return "ok"
}

// ValidateSummaryConsistency checks if summary totals are consistent
func ValidateSummaryConsistency(report *IntegratedReport) error {
	for name, component := range report.Components {
		if component == nil {
			continue
		}

		// Count actual issues
		actualOK := 0
		actualWarn := 0
		actualError := 0

		for _, file := range component.Files {
			hasError := false
			hasWarn := false

			for _, issue := range file.Issues {
				switch issue.Type {
				case "error":
					hasError = true
				case "warn":
					hasWarn = true
				}
			}

			if hasError {
				actualError++
			} else if hasWarn {
				actualWarn++
			} else if len(file.Issues) > 0 {
				// Has issues but all are "ok"
				actualOK++
			}
		}

		// For components with simple counting logic
		if component.Summary.Files > 0 {
			total := component.Summary.OK + component.Summary.Warn + component.Summary.Error
			if total != component.Summary.Files && total > 0 {
				return fmt.Errorf("component %s: summary mismatch - files=%d but ok+warn+error=%d",
					name, component.Summary.Files, total)
			}
		}
	}

	return nil
}
