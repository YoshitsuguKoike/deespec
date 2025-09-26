package state

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/validator/common"
)

// ValidateStateFile validates a state.json file
func ValidateStateFile(filePath string) (*common.ValidationResult, error) {
	result := common.NewValidationResult()

	// Check if file exists
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File not found - add as warning
			fileResult := common.FileResult{
				File: filePath,
				Issues: []common.ValidationIssue{
					{
						Type:    "warn",
						Message: "file not found",
					},
				},
			}
			result.AddFileResult(fileResult)
			return result, nil
		}
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Parse JSON
	var stateData map[string]interface{}
	if err := json.Unmarshal(data, &stateData); err != nil {
		fileResult := common.FileResult{
			File: filePath,
			Issues: []common.ValidationIssue{
				{
					Type:    "error",
					Message: fmt.Sprintf("invalid JSON: %v", err),
				},
			},
		}
		result.AddFileResult(fileResult)
		return result, nil
	}

	// Validate schema
	issues := validateStateSchema(stateData)
	fileResult := common.FileResult{
		File:   filePath,
		Issues: issues,
	}
	result.AddFileResult(fileResult)

	return result, nil
}

// validateStateSchema validates the state.json schema
func validateStateSchema(data map[string]interface{}) []common.ValidationIssue {
	var issues []common.ValidationIssue

	// Required keys: version, step, turn, meta.updated_at
	// Forbidden keys: current
	requiredKeys := []string{"version", "step", "turn", "meta.updated_at"}
	forbiddenKeys := []string{"current"}

	common.ValidateRequiredKeys(data, requiredKeys, forbiddenKeys, &issues)

	// Validate individual fields if present
	if version, exists := data["version"]; exists {
		exactValue := 1
		common.ValidateIntValue(version, "version", &exactValue, nil, &issues)
	}

	if step, exists := data["step"]; exists {
		common.ValidateEnumValue(step, "step", common.ValidSteps, &issues)
	}

	if turn, exists := data["turn"]; exists {
		minValue := 0
		common.ValidateIntValue(turn, "turn", nil, &minValue, &issues)
	}

	if metaUpdatedAt, exists := data["meta.updated_at"]; exists {
		if tsString, ok := metaUpdatedAt.(string); ok {
			common.ValidateRFC3339NanoUTC(tsString, "meta.updated_at", &issues)
		} else {
			issues = append(issues, common.ValidationIssue{
				Type:    "error",
				Field:   "meta.updated_at",
				Message: "must be a string",
			})
		}
	}

	return issues
}