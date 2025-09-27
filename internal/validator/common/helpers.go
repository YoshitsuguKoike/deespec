package common

import (
	"fmt"
	"strings"
	"time"
)

// ValidateRFC3339NanoUTC validates that a timestamp string is RFC3339Nano UTC format with Z suffix
func ValidateRFC3339NanoUTC(ts string, fieldName string, issues *[]ValidationIssue) {
	if ts == "" {
		*issues = append(*issues, ValidationIssue{
			Type:    "error",
			Field:   fieldName,
			Message: "timestamp cannot be empty",
		})
		return
	}

	// Must end with Z (UTC)
	if !strings.HasSuffix(ts, "Z") {
		*issues = append(*issues, ValidationIssue{
			Type:    "error",
			Field:   fieldName,
			Message: "not RFC3339Nano UTC Z",
		})
	}

	// Parse as RFC3339Nano
	_, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		*issues = append(*issues, ValidationIssue{
			Type:    "error",
			Field:   fieldName,
			Message: fmt.Sprintf("invalid RFC3339Nano format: %v", err),
		})
	}
}

// ValidateRequiredKeys checks that all required keys are present and no forbidden keys exist
func ValidateRequiredKeys(data map[string]interface{}, required []string, forbidden []string, issues *[]ValidationIssue) {
	// Check required keys
	for _, key := range required {
		if _, exists := data[key]; !exists {
			*issues = append(*issues, ValidationIssue{
				Type:    "error",
				Field:   key,
				Message: fmt.Sprintf("missing required key: %s", key),
			})
		}
	}

	// Check forbidden keys
	for _, key := range forbidden {
		if _, exists := data[key]; exists {
			*issues = append(*issues, ValidationIssue{
				Type:    "error",
				Field:   key,
				Message: fmt.Sprintf("forbidden key present: %s", key),
			})
		}
	}
}

// ValidateIntValue validates an integer value with optional constraints
func ValidateIntValue(value interface{}, fieldName string, exactValue *int, minValue *int, issues *[]ValidationIssue) {
	intVal, ok := value.(float64) // JSON numbers are float64
	if !ok {
		*issues = append(*issues, ValidationIssue{
			Type:    "error",
			Field:   fieldName,
			Message: "must be an integer",
		})
		return
	}

	intValue := int(intVal)

	if exactValue != nil && intValue != *exactValue {
		*issues = append(*issues, ValidationIssue{
			Type:    "error",
			Field:   fieldName,
			Message: fmt.Sprintf("must be %d", *exactValue),
		})
	}

	if minValue != nil && intValue < *minValue {
		*issues = append(*issues, ValidationIssue{
			Type:    "error",
			Field:   fieldName,
			Message: fmt.Sprintf("must be >= %d", *minValue),
		})
	}
}

// ValidateEnumValue validates that a string value is within allowed enum values
func ValidateEnumValue(value interface{}, fieldName string, allowedValues map[string]bool, issues *[]ValidationIssue) {
	strVal, ok := value.(string)
	if !ok {
		*issues = append(*issues, ValidationIssue{
			Type:    "error",
			Field:   fieldName,
			Message: "must be a string",
		})
		return
	}

	if !allowedValues[strVal] {
		var allowedList []string
		for k := range allowedValues {
			allowedList = append(allowedList, k)
		}
		*issues = append(*issues, ValidationIssue{
			Type:    "error",
			Field:   fieldName,
			Message: fmt.Sprintf("invalid value: %s (must be one of: %s)", strVal, strings.Join(allowedList, "|")),
		})
	}
}

// ValidateBoolValue validates that a value is a boolean
func ValidateBoolValue(value interface{}, fieldName string, issues *[]ValidationIssue) {
	if _, ok := value.(bool); !ok {
		*issues = append(*issues, ValidationIssue{
			Type:    "error",
			Field:   fieldName,
			Message: "must be a boolean",
		})
	}
}

// ValidateStringValue validates that a value is a string
func ValidateStringValue(value interface{}, fieldName string, issues *[]ValidationIssue) {
	if _, ok := value.(string); !ok {
		*issues = append(*issues, ValidationIssue{
			Type:    "error",
			Field:   fieldName,
			Message: "must be a string",
		})
	}
}
