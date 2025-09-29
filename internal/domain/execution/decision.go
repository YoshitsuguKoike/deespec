package execution

import "strings"

// Decision represents the review decision
type Decision string

const (
	DecisionPending      Decision = "PENDING"       // Not yet decided
	DecisionNeedsChanges Decision = "NEEDS_CHANGES" // Needs changes, retry
	DecisionSucceeded    Decision = "SUCCEEDED"     // Final success status
	DecisionFailed       Decision = "FAILED"        // Final failure status
)

// String returns the string representation of the decision
func (d Decision) String() string {
	return string(d)
}

// IsValid returns true if the decision is valid
func (d Decision) IsValid() bool {
	switch d {
	case DecisionPending, DecisionNeedsChanges, DecisionSucceeded, DecisionFailed:
		return true
	default:
		return false
	}
}

// IsFinal returns true if this is a final decision (succeeded/failed)
func (d Decision) IsFinal() bool {
	return d == DecisionSucceeded || d == DecisionFailed
}

// IsApproved returns true if the decision is approved (SUCCEEDED)
func (d Decision) IsApproved() bool {
	return d == DecisionSucceeded
}

// RequiresRetry returns true if the decision requires retry
func (d Decision) RequiresRetry() bool {
	return d == DecisionNeedsChanges
}

// IsPending returns true if the decision is pending
func (d Decision) IsPending() bool {
	return d == DecisionPending
}

// ParseDecision parses a string into a Decision
func ParseDecision(s string) Decision {
	normalized := strings.ToUpper(strings.TrimSpace(s))

	switch normalized {
	case "OK", "APPROVED", "PASS", "PASSED", "SUCCEEDED", "SUCCESS":
		return DecisionSucceeded
	case "NEEDS_CHANGES", "NEEDS CHANGES", "FAIL", "REJECT", "REJECTED":
		return DecisionNeedsChanges
	case "FAILED", "FAILURE":
		return DecisionFailed
	case "PENDING", "":
		return DecisionPending
	default:
		// Default to needs changes for unknown values
		return DecisionNeedsChanges
	}
}

// ToJournalDecision converts to the journal format decision
func (d Decision) ToJournalDecision() string {
	switch d {
	case DecisionSucceeded:
		return "OK"
	case DecisionNeedsChanges, DecisionFailed:
		return "NEEDS_CHANGES"
	case DecisionPending:
		return "PENDING"
	default:
		return ""
	}
}

// CanTransitionTo checks if transition to another decision is allowed
func (d Decision) CanTransitionTo(next Decision) bool {
	// From pending, can go to other decisions
	if d == DecisionPending {
		return next == DecisionNeedsChanges || next == DecisionSucceeded || next == DecisionFailed
	}

	// From needs changes, can go to succeeded or failed
	if d == DecisionNeedsChanges {
		return next == DecisionSucceeded || next == DecisionFailed || next == DecisionPending
	}

	// Final decisions cannot transition
	if d.IsFinal() {
		return false
	}

	return false
}
