package common

import (
	"testing"
)

func TestNextStatusTransition(t *testing.T) {
	tests := []struct {
		name           string
		currentStatus  string
		decision       string
		attempt        int
		expectedStatus string
	}{
		// READY transitions
		{
			name:           "READY to WIP",
			currentStatus:  "READY",
			decision:       "",
			attempt:        1,
			expectedStatus: "WIP",
		},
		{
			name:           "Empty status to WIP",
			currentStatus:  "",
			decision:       "",
			attempt:        1,
			expectedStatus: "WIP",
		},

		// WIP transitions
		{
			name:           "WIP to REVIEW",
			currentStatus:  "WIP",
			decision:       "",
			attempt:        1,
			expectedStatus: "REVIEW",
		},

		// REVIEW transitions
		{
			name:           "REVIEW to DONE when SUCCEEDED",
			currentStatus:  "REVIEW",
			decision:       "SUCCEEDED",
			attempt:        1,
			expectedStatus: "DONE",
		},
		{
			name:           "REVIEW to WIP when NEEDS_CHANGES (attempt 1)",
			currentStatus:  "REVIEW",
			decision:       "NEEDS_CHANGES",
			attempt:        1,
			expectedStatus: "WIP",
		},
		{
			name:           "REVIEW to WIP when FAILED (attempt 2)",
			currentStatus:  "REVIEW",
			decision:       "FAILED",
			attempt:        2,
			expectedStatus: "WIP",
		},
		{
			name:           "REVIEW to REVIEW&WIP when attempt >= 3 (NEEDS_CHANGES)",
			currentStatus:  "REVIEW",
			decision:       "NEEDS_CHANGES",
			attempt:        3,
			expectedStatus: "REVIEW&WIP",
		},
		{
			name:           "REVIEW to REVIEW&WIP when attempt >= 3 (FAILED)",
			currentStatus:  "REVIEW",
			decision:       "FAILED",
			attempt:        3,
			expectedStatus: "REVIEW&WIP",
		},
		{
			name:           "REVIEW to REVIEW&WIP when attempt > 3",
			currentStatus:  "REVIEW",
			decision:       "NEEDS_CHANGES",
			attempt:        5,
			expectedStatus: "REVIEW&WIP",
		},

		// REVIEW&WIP transitions
		{
			name:           "REVIEW&WIP to DONE",
			currentStatus:  "REVIEW&WIP",
			decision:       "",
			attempt:        3,
			expectedStatus: "DONE",
		},

		// DONE transitions
		{
			name:           "DONE stays DONE",
			currentStatus:  "DONE",
			decision:       "",
			attempt:        0,
			expectedStatus: "DONE",
		},

		// Unknown status
		{
			name:           "Unknown status defaults to READY",
			currentStatus:  "UNKNOWN",
			decision:       "",
			attempt:        1,
			expectedStatus: "READY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nextStatusTransition(tt.currentStatus, tt.decision, tt.attempt)
			if result != tt.expectedStatus {
				t.Errorf("nextStatusTransition(%q, %q, %d) = %q, want %q",
					tt.currentStatus, tt.decision, tt.attempt, result, tt.expectedStatus)
			}
		})
	}
}
