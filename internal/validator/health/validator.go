package health

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/validator/common"
)

// ValidateHealthFile validates a health.json file
func ValidateHealthFile(filePath string) (*common.ValidationResult, error) {
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
	var healthData map[string]interface{}
	if err := json.Unmarshal(data, &healthData); err != nil {
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
	issues := validateHealthSchema(healthData)
	fileResult := common.FileResult{
		File:   filePath,
		Issues: issues,
	}
	result.AddFileResult(fileResult)

	return result, nil
}

// validateHealthSchema validates the health.json schema
func validateHealthSchema(data map[string]interface{}) []common.ValidationIssue {
	var issues []common.ValidationIssue

	// Required keys: ts, turn, step, ok, error
	requiredKeys := []string{"ts", "turn", "step", "ok", "error"}
	forbiddenKeys := []string{} // No forbidden keys for health.json

	common.ValidateRequiredKeys(data, requiredKeys, forbiddenKeys, &issues)

	// Validate individual fields if present
	if ts, exists := data["ts"]; exists {
		if tsString, ok := ts.(string); ok {
			common.ValidateRFC3339NanoUTC(tsString, "ts", &issues)
		} else {
			issues = append(issues, common.ValidationIssue{
				Type:    "error",
				Field:   "ts",
				Message: "must be a string",
			})
		}
	}

	if turn, exists := data["turn"]; exists {
		minValue := 0
		common.ValidateIntValue(turn, "turn", nil, &minValue, &issues)
	}

	if step, exists := data["step"]; exists {
		common.ValidateEnumValue(step, "step", common.ValidSteps, &issues)
	}

	if ok, exists := data["ok"]; exists {
		common.ValidateBoolValue(ok, "ok", &issues)
	}

	if errorMsg, exists := data["error"]; exists {
		common.ValidateStringValue(errorMsg, "error", &issues)
	}

	// Cross-field validation: ok/error consistency
	validateOkErrorConsistency(data, &issues)

	return issues
}

// validateOkErrorConsistency checks if ok and error fields are consistent
func validateOkErrorConsistency(data map[string]interface{}, issues *[]common.ValidationIssue) {
	okVal, hasOk := data["ok"]
	errorVal, hasError := data["error"]

	if !hasOk || !hasError {
		return // Can't validate consistency if either field is missing
	}

	ok, okIsBool := okVal.(bool)
	errorMsg, errorIsString := errorVal.(string)

	if !okIsBool || !errorIsString {
		return // Type errors already reported
	}

	// Rule: ok=true should have error="", ok=false should have error!=""
	if ok && errorMsg != "" {
		*issues = append(*issues, common.ValidationIssue{
			Type:    "warn",
			Field:   "ok",
			Message: fmt.Sprintf("ok=true but error=\"%s\" (expected empty error)", errorMsg),
		})
	} else if !ok && errorMsg == "" {
		*issues = append(*issues, common.ValidationIssue{
			Type:    "warn",
			Field:   "ok",
			Message: "ok=false but error is empty (expected non-empty error)",
		})
	}
}
