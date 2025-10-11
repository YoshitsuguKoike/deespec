package execution

import (
	"testing"
)

func TestDecisionString(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		expected string
	}{
		{"Pending to string", DecisionPending, "PENDING"},
		{"NeedsChanges to string", DecisionNeedsChanges, "NEEDS_CHANGES"},
		{"Succeeded to string", DecisionSucceeded, "SUCCEEDED"},
		{"Failed to string", DecisionFailed, "FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.decision.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.decision.String())
			}
		})
	}
}

func TestDecisionIsValid(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		expected bool
	}{
		{"Pending is valid", DecisionPending, true},
		{"NeedsChanges is valid", DecisionNeedsChanges, true},
		{"Succeeded is valid", DecisionSucceeded, true},
		{"Failed is valid", DecisionFailed, true},
		{"Invalid decision", Decision("INVALID"), false},
		{"Empty string is invalid", Decision(""), false},
		{"Random string is invalid", Decision("RANDOM"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.decision.IsValid() != tt.expected {
				t.Errorf("Expected IsValid() to be %v for %s, got %v", tt.expected, tt.decision, tt.decision.IsValid())
			}
		})
	}
}

func TestDecisionIsFinal(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		expected bool
	}{
		{"Succeeded is final", DecisionSucceeded, true},
		{"Failed is final", DecisionFailed, true},
		{"Pending is not final", DecisionPending, false},
		{"NeedsChanges is not final", DecisionNeedsChanges, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.decision.IsFinal() != tt.expected {
				t.Errorf("Expected IsFinal() to be %v for %s, got %v", tt.expected, tt.decision, tt.decision.IsFinal())
			}
		})
	}
}

func TestDecisionIsApproved(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		expected bool
	}{
		{"Succeeded is approved", DecisionSucceeded, true},
		{"Failed is not approved", DecisionFailed, false},
		{"Pending is not approved", DecisionPending, false},
		{"NeedsChanges is not approved", DecisionNeedsChanges, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.decision.IsApproved() != tt.expected {
				t.Errorf("Expected IsApproved() to be %v for %s, got %v", tt.expected, tt.decision, tt.decision.IsApproved())
			}
		})
	}
}

func TestDecisionRequiresRetry(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		expected bool
	}{
		{"NeedsChanges requires retry", DecisionNeedsChanges, true},
		{"Succeeded does not require retry", DecisionSucceeded, false},
		{"Failed does not require retry", DecisionFailed, false},
		{"Pending does not require retry", DecisionPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.decision.RequiresRetry() != tt.expected {
				t.Errorf("Expected RequiresRetry() to be %v for %s, got %v", tt.expected, tt.decision, tt.decision.RequiresRetry())
			}
		})
	}
}

func TestDecisionIsPending(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		expected bool
	}{
		{"Pending is pending", DecisionPending, true},
		{"NeedsChanges is not pending", DecisionNeedsChanges, false},
		{"Succeeded is not pending", DecisionSucceeded, false},
		{"Failed is not pending", DecisionFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.decision.IsPending() != tt.expected {
				t.Errorf("Expected IsPending() to be %v for %s, got %v", tt.expected, tt.decision, tt.decision.IsPending())
			}
		})
	}
}

func TestParseDecision(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Decision
	}{
		// Success variants
		{"Parse OK", "OK", DecisionSucceeded},
		{"Parse APPROVED", "APPROVED", DecisionSucceeded},
		{"Parse PASS", "PASS", DecisionSucceeded},
		{"Parse PASSED", "PASSED", DecisionSucceeded},
		{"Parse SUCCEEDED", "SUCCEEDED", DecisionSucceeded},
		{"Parse SUCCESS", "SUCCESS", DecisionSucceeded},
		{"Parse lowercase ok", "ok", DecisionSucceeded},
		{"Parse mixed case Approved", "Approved", DecisionSucceeded},

		// Needs changes variants
		{"Parse NEEDS_CHANGES", "NEEDS_CHANGES", DecisionNeedsChanges},
		{"Parse NEEDS CHANGES", "NEEDS CHANGES", DecisionNeedsChanges},
		{"Parse FAIL", "FAIL", DecisionNeedsChanges},
		{"Parse REJECT", "REJECT", DecisionNeedsChanges},
		{"Parse REJECTED", "REJECTED", DecisionNeedsChanges},
		{"Parse lowercase fail", "fail", DecisionNeedsChanges},

		// Failed variants
		{"Parse FAILED", "FAILED", DecisionFailed},
		{"Parse FAILURE", "FAILURE", DecisionFailed},
		{"Parse lowercase failed", "failed", DecisionFailed},

		// Pending variants
		{"Parse PENDING", "PENDING", DecisionPending},
		{"Parse empty string", "", DecisionPending},
		{"Parse whitespace", "   ", DecisionPending},

		// Unknown defaults to needs changes
		{"Parse unknown value", "UNKNOWN", DecisionNeedsChanges},
		{"Parse random string", "RANDOM", DecisionNeedsChanges},
		{"Parse with extra spaces", "  OK  ", DecisionSucceeded},
		{"Parse with tabs", "\tOK\t", DecisionSucceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDecision(tt.input)
			if result != tt.expected {
				t.Errorf("Expected ParseDecision(%q) to be %s, got %s", tt.input, tt.expected, result)
			}
		})
	}
}

func TestDecisionToJournalDecision(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		expected string
	}{
		{"Succeeded to journal", DecisionSucceeded, "OK"},
		{"NeedsChanges to journal", DecisionNeedsChanges, "NEEDS_CHANGES"},
		{"Failed to journal", DecisionFailed, "NEEDS_CHANGES"},
		{"Pending to journal", DecisionPending, "PENDING"},
		{"Invalid decision to journal", Decision("INVALID"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.decision.ToJournalDecision()
			if result != tt.expected {
				t.Errorf("Expected ToJournalDecision() to be %q for %s, got %q", tt.expected, tt.decision, result)
			}
		})
	}
}

func TestDecisionCanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		from     Decision
		to       Decision
		expected bool
	}{
		// From Pending
		{"Pending to NeedsChanges", DecisionPending, DecisionNeedsChanges, true},
		{"Pending to Succeeded", DecisionPending, DecisionSucceeded, true},
		{"Pending to Failed", DecisionPending, DecisionFailed, true},
		{"Pending to Pending", DecisionPending, DecisionPending, false},

		// From NeedsChanges
		{"NeedsChanges to Succeeded", DecisionNeedsChanges, DecisionSucceeded, true},
		{"NeedsChanges to Failed", DecisionNeedsChanges, DecisionFailed, true},
		{"NeedsChanges to Pending", DecisionNeedsChanges, DecisionPending, true},
		{"NeedsChanges to NeedsChanges", DecisionNeedsChanges, DecisionNeedsChanges, false},

		// From Final decisions (Succeeded)
		{"Succeeded to any", DecisionSucceeded, DecisionNeedsChanges, false},
		{"Succeeded to Failed", DecisionSucceeded, DecisionFailed, false},
		{"Succeeded to Pending", DecisionSucceeded, DecisionPending, false},
		{"Succeeded to Succeeded", DecisionSucceeded, DecisionSucceeded, false},

		// From Final decisions (Failed)
		{"Failed to any", DecisionFailed, DecisionNeedsChanges, false},
		{"Failed to Succeeded", DecisionFailed, DecisionSucceeded, false},
		{"Failed to Pending", DecisionFailed, DecisionPending, false},
		{"Failed to Failed", DecisionFailed, DecisionFailed, false},

		// From Invalid decision
		{"Invalid to any", Decision("INVALID"), DecisionNeedsChanges, false},
		{"Invalid to Succeeded", Decision("INVALID"), DecisionSucceeded, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			if result != tt.expected {
				t.Errorf("Expected %s.CanTransitionTo(%s) to be %v, got %v", tt.from, tt.to, tt.expected, result)
			}
		})
	}
}

func TestDecisionConstants(t *testing.T) {
	// Test that constants have expected values
	if DecisionPending != "PENDING" {
		t.Errorf("DecisionPending constant has wrong value: %s", DecisionPending)
	}
	if DecisionNeedsChanges != "NEEDS_CHANGES" {
		t.Errorf("DecisionNeedsChanges constant has wrong value: %s", DecisionNeedsChanges)
	}
	if DecisionSucceeded != "SUCCEEDED" {
		t.Errorf("DecisionSucceeded constant has wrong value: %s", DecisionSucceeded)
	}
	if DecisionFailed != "FAILED" {
		t.Errorf("DecisionFailed constant has wrong value: %s", DecisionFailed)
	}
}

func TestDecisionRoundTrip(t *testing.T) {
	// Test that parsing and converting back works correctly
	tests := []struct {
		name     string
		input    string
		expected Decision
	}{
		{"Round trip OK", "OK", DecisionSucceeded},
		{"Round trip NEEDS_CHANGES", "NEEDS_CHANGES", DecisionNeedsChanges},
		{"Round trip FAILED", "FAILED", DecisionFailed},
		{"Round trip PENDING", "PENDING", DecisionPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseDecision(tt.input)
			if parsed != tt.expected {
				t.Errorf("Expected ParseDecision(%q) to be %s, got %s", tt.input, tt.expected, parsed)
			}

			// For succeeded, verify it converts to OK in journal
			if parsed == DecisionSucceeded {
				journal := parsed.ToJournalDecision()
				if journal != "OK" {
					t.Errorf("Expected journal format to be OK, got %s", journal)
				}
			}
		})
	}
}

func TestDecisionEdgeCases(t *testing.T) {
	// Test edge cases and boundary conditions
	t.Run("Parse with Unicode", func(t *testing.T) {
		// Unicode characters should be treated as unknown
		result := ParseDecision("成功")
		if result != DecisionNeedsChanges {
			t.Errorf("Expected Unicode to default to NEEDS_CHANGES, got %s", result)
		}
	})

	t.Run("Parse very long string", func(t *testing.T) {
		longString := string(make([]byte, 10000))
		result := ParseDecision(longString)
		if result != DecisionNeedsChanges {
			t.Errorf("Expected long string to default to NEEDS_CHANGES, got %s", result)
		}
	})

	t.Run("Case insensitive parsing", func(t *testing.T) {
		inputs := []string{"ok", "OK", "Ok", "oK"}
		for _, input := range inputs {
			result := ParseDecision(input)
			if result != DecisionSucceeded {
				t.Errorf("Expected %q to parse as SUCCEEDED, got %s", input, result)
			}
		}
	})
}

func TestDecisionStateTransitions(t *testing.T) {
	// Test a complete workflow of state transitions
	t.Run("Complete workflow", func(t *testing.T) {
		// Start pending
		decision := DecisionPending
		if !decision.IsPending() {
			t.Error("Expected decision to be pending")
		}

		// Can transition to needs changes
		if !decision.CanTransitionTo(DecisionNeedsChanges) {
			t.Error("Should be able to transition from PENDING to NEEDS_CHANGES")
		}

		// Transition to needs changes
		decision = DecisionNeedsChanges
		if !decision.RequiresRetry() {
			t.Error("Expected decision to require retry")
		}

		// Can transition to succeeded
		if !decision.CanTransitionTo(DecisionSucceeded) {
			t.Error("Should be able to transition from NEEDS_CHANGES to SUCCEEDED")
		}

		// Transition to succeeded
		decision = DecisionSucceeded
		if !decision.IsFinal() {
			t.Error("Expected decision to be final")
		}
		if !decision.IsApproved() {
			t.Error("Expected decision to be approved")
		}

		// Cannot transition from final state
		if decision.CanTransitionTo(DecisionPending) {
			t.Error("Should not be able to transition from final state")
		}
	})
}
