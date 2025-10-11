package execution

import (
	"testing"
)

// TestExecutionStatusString verifies the String() method
func TestExecutionStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected string
	}{
		{"Ready status", StatusReady, "READY"},
		{"WIP status", StatusWIP, "WIP"},
		{"Review status", StatusReview, "REVIEW"},
		{"ReviewAndWIP status", StatusReviewAndWIP, "REVIEW&WIP"},
		{"Done status", StatusDone, "DONE"},
		{"Unknown status", StatusUnknown, "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestExecutionStatusIsActive verifies the IsActive() method
func TestExecutionStatusIsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected bool
	}{
		{"Ready is active", StatusReady, true},
		{"WIP is active", StatusWIP, true},
		{"Review is active", StatusReview, true},
		{"ReviewAndWIP is active", StatusReviewAndWIP, true},
		{"Done is not active", StatusDone, false},
		{"Unknown is not active", StatusUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsActive()
			if result != tt.expected {
				t.Errorf("Expected IsActive() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExecutionStatusIsWIP verifies the IsWIP() method
func TestExecutionStatusIsWIP(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected bool
	}{
		{"WIP is WIP", StatusWIP, true},
		{"ReviewAndWIP is WIP", StatusReviewAndWIP, true},
		{"Ready is not WIP", StatusReady, false},
		{"Review is not WIP", StatusReview, false},
		{"Done is not WIP", StatusDone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsWIP()
			if result != tt.expected {
				t.Errorf("Expected IsWIP() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExecutionStatusIsReview verifies the IsReview() method
func TestExecutionStatusIsReview(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected bool
	}{
		{"Review is Review", StatusReview, true},
		{"ReviewAndWIP is Review", StatusReviewAndWIP, true},
		{"Ready is not Review", StatusReady, false},
		{"WIP is not Review", StatusWIP, false},
		{"Done is not Review", StatusDone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsReview()
			if result != tt.expected {
				t.Errorf("Expected IsReview() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExecutionStatusIsReady verifies the IsReady() method
func TestExecutionStatusIsReady(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected bool
	}{
		{"Ready is Ready", StatusReady, true},
		{"WIP is not Ready", StatusWIP, false},
		{"Review is not Ready", StatusReview, false},
		{"Done is not Ready", StatusDone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsReady()
			if result != tt.expected {
				t.Errorf("Expected IsReady() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExecutionStatusIsDone verifies the IsDone() method
func TestExecutionStatusIsDone(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected bool
	}{
		{"Done is Done", StatusDone, true},
		{"Ready is not Done", StatusReady, false},
		{"WIP is not Done", StatusWIP, false},
		{"Review is not Done", StatusReview, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsDone()
			if result != tt.expected {
				t.Errorf("Expected IsDone() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExecutionStatusIsValid verifies the IsValid() method
func TestExecutionStatusIsValid(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected bool
	}{
		{"Ready is valid", StatusReady, true},
		{"WIP is valid", StatusWIP, true},
		{"Review is valid", StatusReview, true},
		{"ReviewAndWIP is valid", StatusReviewAndWIP, true},
		{"Done is valid", StatusDone, true},
		{"Unknown is invalid", StatusUnknown, false},
		{"Invalid custom status", ExecutionStatus("INVALID"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsValid()
			if result != tt.expected {
				t.Errorf("Expected IsValid() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExecutionStatusCanTransitionTo verifies the CanTransitionTo() method
func TestExecutionStatusCanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		current  ExecutionStatus
		next     ExecutionStatus
		expected bool
	}{
		// Valid transitions
		{"Ready to WIP", StatusReady, StatusWIP, true},
		{"WIP to Review", StatusWIP, StatusReview, true},
		{"Review to WIP", StatusReview, StatusWIP, true},
		{"Review to Done", StatusReview, StatusDone, true},
		{"Review to ReviewAndWIP", StatusReview, StatusReviewAndWIP, true},
		{"ReviewAndWIP to Review", StatusReviewAndWIP, StatusReview, true},

		// Invalid transitions
		{"Ready to Review", StatusReady, StatusReview, false},
		{"Ready to Done", StatusReady, StatusDone, false},
		{"WIP to Done", StatusWIP, StatusDone, false},
		{"WIP to ReviewAndWIP", StatusWIP, StatusReviewAndWIP, false},
		{"Done to anything", StatusDone, StatusReady, false},
		{"Done to WIP", StatusDone, StatusWIP, false},
		{"Done to Review", StatusDone, StatusReview, false},
		{"ReviewAndWIP to Done", StatusReviewAndWIP, StatusDone, false},
		{"Unknown to Ready", StatusUnknown, StatusReady, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.current.CanTransitionTo(tt.next)
			if result != tt.expected {
				t.Errorf("Expected CanTransitionTo(%s, %s) = %v, got %v",
					tt.current, tt.next, tt.expected, result)
			}
		})
	}
}

// TestExecutionStatusPriority verifies the Priority() method
func TestExecutionStatusPriority(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected int
	}{
		{"ReviewAndWIP has highest priority", StatusReviewAndWIP, 1},
		{"Review has priority 2", StatusReview, 2},
		{"WIP has priority 3", StatusWIP, 3},
		{"Ready has priority 4", StatusReady, 4},
		{"Done has priority 5", StatusDone, 5},
		{"Unknown has lowest priority", StatusUnknown, 99},
		{"Invalid status has lowest priority", ExecutionStatus("INVALID"), 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.Priority()
			if result != tt.expected {
				t.Errorf("Expected Priority() = %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestExecutionStatusPriorityOrdering verifies that priorities are correctly ordered
func TestExecutionStatusPriorityOrdering(t *testing.T) {
	statuses := []ExecutionStatus{
		StatusReviewAndWIP,
		StatusReview,
		StatusWIP,
		StatusReady,
		StatusDone,
	}

	// Verify that priorities are in ascending order
	for i := 0; i < len(statuses)-1; i++ {
		currentPriority := statuses[i].Priority()
		nextPriority := statuses[i+1].Priority()

		if currentPriority >= nextPriority {
			t.Errorf("Priority ordering broken: %s (priority %d) should be higher than %s (priority %d)",
				statuses[i], currentPriority, statuses[i+1], nextPriority)
		}
	}
}

// TestExecutionStatusTransitionChain verifies a complete transition chain
func TestExecutionStatusTransitionChain(t *testing.T) {
	transitions := []struct {
		from ExecutionStatus
		to   ExecutionStatus
	}{
		{StatusReady, StatusWIP},
		{StatusWIP, StatusReview},
		{StatusReview, StatusWIP},          // Retry
		{StatusWIP, StatusReview},          // Second review
		{StatusReview, StatusReviewAndWIP}, // Force implementation
		{StatusReviewAndWIP, StatusReview}, // Implementer review
		{StatusReview, StatusDone},         // Complete
	}

	current := StatusReady
	for i, transition := range transitions {
		if current != transition.from {
			t.Fatalf("Step %d: Expected to be in state %s, but was in %s",
				i, transition.from, current)
		}

		if !current.CanTransitionTo(transition.to) {
			t.Errorf("Step %d: Transition from %s to %s should be allowed",
				i, current, transition.to)
		}

		current = transition.to
	}

	if !current.IsDone() {
		t.Errorf("Expected final state to be Done, got %s", current)
	}
}

// TestExecutionStatusExhaustiveTransitions verifies all possible transitions are documented
func TestExecutionStatusExhaustiveTransitions(t *testing.T) {
	allStatuses := []ExecutionStatus{
		StatusReady,
		StatusWIP,
		StatusReview,
		StatusReviewAndWIP,
		StatusDone,
		StatusUnknown,
	}

	// Test all combinations to ensure no undocumented transitions
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			// This just ensures CanTransitionTo doesn't panic
			_ = from.CanTransitionTo(to)
		}
	}
}

// TestExecutionStatusInvalidTransitions verifies that invalid transitions are rejected
func TestExecutionStatusInvalidTransitions(t *testing.T) {
	// Test that Ready cannot skip directly to Done
	if StatusReady.CanTransitionTo(StatusDone) {
		t.Error("Ready should not be able to transition directly to Done")
	}

	// Test that WIP cannot skip directly to Done
	if StatusWIP.CanTransitionTo(StatusDone) {
		t.Error("WIP should not be able to transition directly to Done")
	}

	// Test that Done cannot transition to anything
	for _, status := range []ExecutionStatus{StatusReady, StatusWIP, StatusReview, StatusReviewAndWIP} {
		if StatusDone.CanTransitionTo(status) {
			t.Errorf("Done should not be able to transition to %s", status)
		}
	}

	// Test that Unknown cannot transition to anything
	for _, status := range []ExecutionStatus{StatusReady, StatusWIP, StatusReview, StatusReviewAndWIP, StatusDone} {
		if StatusUnknown.CanTransitionTo(status) {
			t.Errorf("Unknown should not be able to transition to %s", status)
		}
	}
}

// TestExecutionStatusCombinedPredicates verifies combined behavior of predicates
func TestExecutionStatusCombinedPredicates(t *testing.T) {
	tests := []struct {
		name           string
		status         ExecutionStatus
		shouldBeWIP    bool
		shouldBeReview bool
	}{
		{"WIP only WIP", StatusWIP, true, false},
		{"Review only Review", StatusReview, false, true},
		{"ReviewAndWIP both", StatusReviewAndWIP, true, true},
		{"Ready neither", StatusReady, false, false},
		{"Done neither", StatusDone, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.status.IsWIP() != tt.shouldBeWIP {
				t.Errorf("Expected IsWIP() = %v, got %v", tt.shouldBeWIP, tt.status.IsWIP())
			}
			if tt.status.IsReview() != tt.shouldBeReview {
				t.Errorf("Expected IsReview() = %v, got %v", tt.shouldBeReview, tt.status.IsReview())
			}
		})
	}
}

// TestExecutionStatusEmptyString tests behavior with empty status string
func TestExecutionStatusEmptyString(t *testing.T) {
	empty := ExecutionStatus("")

	if empty.IsValid() {
		t.Error("Empty status should not be valid")
	}

	if empty.Priority() != 99 {
		t.Errorf("Empty status should have lowest priority (99), got %d", empty.Priority())
	}

	if empty.CanTransitionTo(StatusReady) {
		t.Error("Empty status should not be able to transition to any status")
	}
}

// TestExecutionStatusConstants verifies that constants have expected values
func TestExecutionStatusConstants(t *testing.T) {
	tests := []struct {
		status   ExecutionStatus
		expected string
	}{
		{StatusReady, "READY"},
		{StatusWIP, "WIP"},
		{StatusReview, "REVIEW"},
		{StatusReviewAndWIP, "REVIEW&WIP"},
		{StatusDone, "DONE"},
		{StatusUnknown, "UNKNOWN"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("Constant value mismatch: expected %s, got %s", tt.expected, string(tt.status))
		}
	}
}

// BenchmarkExecutionStatusString benchmarks the String method
func BenchmarkExecutionStatusString(b *testing.B) {
	status := StatusReview
	for i := 0; i < b.N; i++ {
		_ = status.String()
	}
}

// BenchmarkExecutionStatusIsActive benchmarks the IsActive method
func BenchmarkExecutionStatusIsActive(b *testing.B) {
	status := StatusReview
	for i := 0; i < b.N; i++ {
		_ = status.IsActive()
	}
}

// BenchmarkExecutionStatusCanTransitionTo benchmarks the CanTransitionTo method
func BenchmarkExecutionStatusCanTransitionTo(b *testing.B) {
	from := StatusReview
	to := StatusDone
	for i := 0; i < b.N; i++ {
		_ = from.CanTransitionTo(to)
	}
}

// BenchmarkExecutionStatusPriority benchmarks the Priority method
func BenchmarkExecutionStatusPriority(b *testing.B) {
	status := StatusReviewAndWIP
	for i := 0; i < b.N; i++ {
		_ = status.Priority()
	}
}
