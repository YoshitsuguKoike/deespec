package runner

import (
	"regexp"
	"strings"
)

// DecisionType represents the review decision type
type DecisionType string

const (
	// DecisionOK indicates the review passed
	DecisionOK DecisionType = "OK"
	// DecisionNeedsChanges indicates the review requires changes
	DecisionNeedsChanges DecisionType = "NEEDS_CHANGES"
	// DecisionPending indicates no clear decision was found
	DecisionPending DecisionType = "PENDING"
)

// ParseDecision parses the review output to determine the decision
// It scans from the bottom of the output for better reliability
func ParseDecision(output string, re *regexp.Regexp) DecisionType {
	if re == nil {
		return DecisionPending
	}

	// Split into lines and scan from bottom
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Scan from the last line backwards
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		matches := re.FindStringSubmatch(line)

		if len(matches) >= 2 {
			// Extract and normalize the captured value
			value := strings.ToUpper(strings.TrimSpace(matches[1]))

			switch value {
			case "OK":
				return DecisionOK
			case "NEEDS_CHANGES":
				return DecisionNeedsChanges
			}
		}
	}

	return DecisionPending
}

// ParseDecisionFromStep parses the review output using the step's compiled regex
func ParseDecisionFromStep(output string, compiledDecision *regexp.Regexp) DecisionType {
	return ParseDecision(output, compiledDecision)
}

// ExtractDecisionLine extracts the line that matched the decision pattern
// This is useful for logging and debugging
func ExtractDecisionLine(output string, re *regexp.Regexp) (string, bool) {
	if re == nil {
		return "", false
	}

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if re.MatchString(line) {
			return line, true
		}
	}

	return "", false
}
