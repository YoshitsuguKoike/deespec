package execution

// ExecutionStep represents the detailed step in the execution flow (1-10)
type ExecutionStep string

const (
	StepReady                  ExecutionStep = "ready"                    // Step 1
	StepImplementTry           ExecutionStep = "implement_try"            // Step 2
	StepFirstReview            ExecutionStep = "first_review"             // Step 3
	StepImplementSecondTry     ExecutionStep = "implement_second_try"     // Step 4
	StepSecondReview           ExecutionStep = "second_review"            // Step 5
	StepImplementThirdTry      ExecutionStep = "implement_third_try"      // Step 6
	StepThirdReview            ExecutionStep = "third_review"             // Step 7
	StepReviewerForceImplement ExecutionStep = "reviewer_force_implement" // Step 8
	StepImplementerReview      ExecutionStep = "implementer_review"       // Step 9
	StepDone                   ExecutionStep = "done"                     // Step 10
)

// String returns the string representation of the step
func (s ExecutionStep) String() string {
	return string(s)
}

// ToNumber returns the numeric representation of the step (1-10)
func (s ExecutionStep) ToNumber() int {
	switch s {
	case StepReady:
		return 1
	case StepImplementTry:
		return 2
	case StepFirstReview:
		return 3
	case StepImplementSecondTry:
		return 4
	case StepSecondReview:
		return 5
	case StepImplementThirdTry:
		return 6
	case StepThirdReview:
		return 7
	case StepReviewerForceImplement:
		return 8
	case StepImplementerReview:
		return 9
	case StepDone:
		return 10
	default:
		return 0
	}
}

// ToStatus converts the step to its corresponding status
func (s ExecutionStep) ToStatus() ExecutionStatus {
	switch s {
	case StepReady:
		return StatusReady
	case StepImplementTry, StepImplementSecondTry, StepImplementThirdTry:
		return StatusWIP
	case StepFirstReview, StepSecondReview, StepThirdReview, StepImplementerReview:
		return StatusReview
	case StepReviewerForceImplement:
		return StatusReviewAndWIP // Special status for dual role
	case StepDone:
		return StatusDone
	default:
		return StatusUnknown
	}
}

// IsImplementation returns true if this is an implementation step
func (s ExecutionStep) IsImplementation() bool {
	return s == StepImplementTry ||
		s == StepImplementSecondTry ||
		s == StepImplementThirdTry ||
		s == StepReviewerForceImplement
}

// IsReview returns true if this is a review step
func (s ExecutionStep) IsReview() bool {
	return s == StepFirstReview ||
		s == StepSecondReview ||
		s == StepThirdReview ||
		s == StepImplementerReview ||
		s == StepReviewerForceImplement // Has review component
}

// CanTransitionTo validates if transition to the next step is allowed
func (s ExecutionStep) CanTransitionTo(next ExecutionStep) bool {
	validTransitions := map[ExecutionStep][]ExecutionStep{
		StepReady:                  {StepImplementTry},
		StepImplementTry:           {StepFirstReview},
		StepFirstReview:            {StepImplementSecondTry, StepDone},
		StepImplementSecondTry:     {StepSecondReview},
		StepSecondReview:           {StepImplementThirdTry, StepDone},
		StepImplementThirdTry:      {StepThirdReview},
		StepThirdReview:            {StepReviewerForceImplement, StepDone},
		StepReviewerForceImplement: {StepImplementerReview},
		StepImplementerReview:      {StepDone},
		StepDone:                   {}, // No transitions from done
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

// GetAttemptNumber returns the attempt number for implementation steps
func (s ExecutionStep) GetAttemptNumber() int {
	switch s {
	case StepImplementTry:
		return 1
	case StepImplementSecondTry:
		return 2
	case StepImplementThirdTry:
		return 3
	case StepReviewerForceImplement:
		return 4 // Special forced implementation
	default:
		return 0
	}
}

// IsValid returns true if the step is a valid execution step
func (s ExecutionStep) IsValid() bool {
	switch s {
	case StepReady, StepImplementTry, StepFirstReview,
		StepImplementSecondTry, StepSecondReview,
		StepImplementThirdTry, StepThirdReview,
		StepReviewerForceImplement, StepImplementerReview,
		StepDone:
		return true
	default:
		return false
	}
}
