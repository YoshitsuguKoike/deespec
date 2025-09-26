package journal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ValidateFile validates a journal NDJSON file and returns detailed results
func (v *Validator) ValidateFile(reader io.Reader) (*ValidationResult, error) {
	result := &ValidationResult{
		Version:     1,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		File:        v.filePath,
		Lines:       []LineResult{},
		Summary:     Summary{},
	}

	scanner := bufio.NewScanner(reader)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		lineResult := v.validateLine(line, lineNumber)
		result.Lines = append(result.Lines, lineResult)

		// Update summary
		result.Summary.Lines++
		hasError := false
		hasWarn := false

		for _, issue := range lineResult.Issues {
			switch issue.Type {
			case "error":
				hasError = true
			case "warn":
				hasWarn = true
			}
		}

		if hasError {
			result.Summary.Error++
		} else if hasWarn {
			result.Summary.Warn++
		} else {
			result.Summary.OK++
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return result, nil
}

// validateLine validates a single NDJSON line
func (v *Validator) validateLine(line string, lineNumber int) LineResult {
	result := LineResult{
		Line:   lineNumber,
		Issues: []ValidationIssue{},
	}

	// Parse as JSON
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(line), &rawData); err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Message: fmt.Sprintf("invalid JSON: %v", err),
		})
		return result
	}

	// Check for exactly 7 keys
	if len(rawData) != 7 {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Message: fmt.Sprintf("expected exactly 7 keys, found %d", len(rawData)),
		})
		return result
	}

	// Required keys
	requiredKeys := []string{"ts", "turn", "step", "decision", "elapsed_ms", "error", "artifacts"}
	for _, key := range requiredKeys {
		if _, exists := rawData[key]; !exists {
			result.Issues = append(result.Issues, ValidationIssue{
				Type:    "error",
				Field:   key,
				Message: fmt.Sprintf("missing required key: %s", key),
			})
		}
	}

	// If we have key count errors, still validate individual fields that exist
	hasKeyCountError := false
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "expected exactly 7 keys") {
			hasKeyCountError = true
			break
		}
	}

	if hasKeyCountError {
		// Validate fields that are present
		if ts, ok := rawData["ts"]; ok {
			if tsStr, isString := ts.(string); isString {
				v.validateTimestamp(tsStr, &result)
			}
		}
		if turn, ok := rawData["turn"]; ok {
			if turnInt, isNumber := turn.(float64); isNumber {
				v.validateTurn(int(turnInt), &result)
			}
		}
		if step, ok := rawData["step"]; ok {
			if stepStr, isString := step.(string); isString {
				v.validateStep(stepStr, &result)
			}
		}
		if decision, ok := rawData["decision"]; ok {
			if decisionStr, isString := decision.(string); isString {
				v.validateDecision(decisionStr, &result)
			}
		}
		if elapsedMS, ok := rawData["elapsed_ms"]; ok {
			if elapsedInt, isNumber := elapsedMS.(float64); isNumber {
				v.validateElapsedMS(int(elapsedInt), &result)
			}
		}
		if errorMsg, ok := rawData["error"]; ok {
			if errorStr, isString := errorMsg.(string); isString {
				v.validateError(errorStr, &result)
			}
		}
		if artifacts, ok := rawData["artifacts"]; ok {
			if artifactsSlice, isSlice := artifacts.([]interface{}); isSlice {
				if turnVal, hasTurn := rawData["turn"]; hasTurn {
					if turnInt, isNumber := turnVal.(float64); isNumber {
						v.validateArtifacts(artifactsSlice, int(turnInt), &result)
					}
				}
			}
		}
		return result
	}

	// Return early if there are other structural errors
	if len(result.Issues) > 0 {
		return result
	}

	// Parse into structured type for detailed validation
	var entry JournalEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Message: fmt.Sprintf("type validation failed: %v", err),
		})
		return result
	}

	// Validate individual fields
	v.validateTimestamp(entry.Timestamp, &result)
	v.validateTurn(entry.Turn, &result)
	v.validateStep(entry.Step, &result)
	v.validateDecision(entry.Decision, &result)
	v.validateElapsedMS(entry.ElapsedMS, &result)
	v.validateError(entry.Error, &result)
	v.validateArtifacts(entry.Artifacts, entry.Turn, &result)

	return result
}

// validateTimestampFromRaw validates timestamp from raw interface (used for structural errors)
func (v *Validator) validateTimestampFromRaw(ts string, result *LineResult) {
	v.validateTimestamp(ts, result)
}

// validateTimestamp validates the timestamp field
func (v *Validator) validateTimestamp(ts string, result *LineResult) {
	if ts == "" {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Field:   "ts",
			Message: "timestamp cannot be empty",
		})
		return
	}

	// Must end with Z (UTC)
	if !strings.HasSuffix(ts, "Z") {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Field:   "ts",
			Message: "timestamp must be UTC (end with Z)",
		})
	}

	// Parse as RFC3339Nano
	_, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Field:   "ts",
			Message: fmt.Sprintf("invalid RFC3339Nano format: %v", err),
		})
	}
}

// validateTurn validates the turn field and checks monotonicity
func (v *Validator) validateTurn(turn int, result *LineResult) {
	if turn < 0 {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Field:   "turn",
			Message: "turn must be >= 0",
		})
		return
	}

	// Check monotonicity (warn if decreasing)
	if v.previousTurn >= 0 && turn < v.previousTurn {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "warn",
			Field:   "turn",
			Message: fmt.Sprintf("turn decreased from %d to %d (non-monotonic)", v.previousTurn, turn),
		})
	}

	v.previousTurn = turn
}

// validateStep validates the step field
func (v *Validator) validateStep(step string, result *LineResult) {
	if !ValidSteps[step] {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Field:   "step",
			Message: fmt.Sprintf("invalid step value: %s (must be plan|implement|test|review|done)", step),
		})
	}
}

// validateDecision validates the decision field
func (v *Validator) validateDecision(decision string, result *LineResult) {
	if !ValidDecisions[decision] {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Field:   "decision",
			Message: fmt.Sprintf("invalid decision value: %s (must be OK|NEEDS_CHANGES|PENDING)", decision),
		})
	}
}

// validateElapsedMS validates the elapsed_ms field
func (v *Validator) validateElapsedMS(elapsedMS int, result *LineResult) {
	if elapsedMS < 0 {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Field:   "elapsed_ms",
			Message: "elapsed_ms must be >= 0",
		})
	}
}

// validateError validates the error field (accepts empty string)
func (v *Validator) validateError(errorMsg string, result *LineResult) {
	// Error field can be empty string, no validation needed
}

// validateArtifacts validates the artifacts field and turn consistency
func (v *Validator) validateArtifacts(artifacts []interface{}, turn int, result *LineResult) {
	if artifacts == nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Field:   "artifacts",
			Message: "artifacts cannot be nil (use empty array [])",
		})
		return
	}

	// If empty array, that's OK (no artifacts)
	if len(artifacts) == 0 {
		return
	}

	// Check turn consistency for string artifacts
	turnPattern := fmt.Sprintf("/turn%d/", turn)
	hasMatchingArtifact := false

	for i, artifact := range artifacts {
		switch art := artifact.(type) {
		case string:
			// Check if artifact path contains /turn<turn>/
			if strings.Contains(art, turnPattern) {
				hasMatchingArtifact = true
			}
		case map[string]interface{}:
			// Object artifacts are allowed (future extension), skip turn check
		default:
			result.Issues = append(result.Issues, ValidationIssue{
				Type:    "error",
				Field:   "artifacts",
				Message: fmt.Sprintf("artifact[%d] must be string or object, got %T", i, artifact),
			})
		}
	}

	// If we have string artifacts but none match the turn, that's an error
	hasStringArtifacts := false
	for _, artifact := range artifacts {
		if _, isString := artifact.(string); isString {
			hasStringArtifacts = true
			break
		}
	}

	if hasStringArtifacts && !hasMatchingArtifact {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:    "error",
			Field:   "artifacts",
			Message: fmt.Sprintf("no artifact path contains /turn%d/ (turn consistency check)", turn),
		})
	}
}