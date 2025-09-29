package execution

// ExecutionStatus represents the high-level status of the execution
type ExecutionStatus string

const (
	StatusReady        ExecutionStatus = "READY"      // Ready to start
	StatusWIP          ExecutionStatus = "WIP"        // Work In Progress (implementation)
	StatusReview       ExecutionStatus = "REVIEW"     // Under review
	StatusReviewAndWIP ExecutionStatus = "REVIEW&WIP" // Special dual status for forced implementation
	StatusDone         ExecutionStatus = "DONE"       // Completed
	StatusUnknown      ExecutionStatus = "UNKNOWN"    // Unknown status
)

// String returns the string representation of the status
func (s ExecutionStatus) String() string {
	return string(s)
}

// IsActive returns true if the execution is still active (not done)
func (s ExecutionStatus) IsActive() bool {
	return s != StatusDone && s != StatusUnknown
}

// IsWIP returns true if the status indicates work in progress
func (s ExecutionStatus) IsWIP() bool {
	return s == StatusWIP || s == StatusReviewAndWIP
}

// IsReview returns true if the status indicates review phase
func (s ExecutionStatus) IsReview() bool {
	return s == StatusReview || s == StatusReviewAndWIP
}

// IsReady returns true if the status is ready
func (s ExecutionStatus) IsReady() bool {
	return s == StatusReady
}

// IsDone returns true if the status is done
func (s ExecutionStatus) IsDone() bool {
	return s == StatusDone
}

// IsValid returns true if the status is valid
func (s ExecutionStatus) IsValid() bool {
	switch s {
	case StatusReady, StatusWIP, StatusReview, StatusReviewAndWIP, StatusDone:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if transition to another status is allowed
func (s ExecutionStatus) CanTransitionTo(next ExecutionStatus) bool {
	validTransitions := map[ExecutionStatus][]ExecutionStatus{
		StatusReady:        {StatusWIP},
		StatusWIP:          {StatusReview},
		StatusReview:       {StatusWIP, StatusDone, StatusReviewAndWIP},
		StatusReviewAndWIP: {StatusReview},
		StatusDone:         {}, // No transitions from done
	}

	allowed, exists := validTransitions[s]
	if !exists {
		return false
	}

	for _, validNext := range allowed {
		if validNext == next {
			return true
		}
	}

	return false
}

// Priority returns the priority of the status for sorting
func (s ExecutionStatus) Priority() int {
	switch s {
	case StatusReviewAndWIP:
		return 1 // Highest priority - critical state
	case StatusReview:
		return 2
	case StatusWIP:
		return 3
	case StatusReady:
		return 4
	case StatusDone:
		return 5
	default:
		return 99
	}
}
