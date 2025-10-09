package common

import (
	"testing"
	"time"
)

func TestLeaseExpired(t *testing.T) {
	tests := []struct {
		name     string
		state    *State
		expected bool
	}{
		{
			name: "no lease set",
			state: &State{
				LeaseExpiresAt: "",
			},
			expected: false,
		},
		{
			name: "lease not expired",
			state: &State{
				LeaseExpiresAt: FormatRFC3339NanoUTC(NowUTC().Add(1 * time.Hour)),
			},
			expected: false,
		},
		{
			name: "lease expired",
			state: &State{
				LeaseExpiresAt: FormatRFC3339NanoUTC(NowUTC().Add(-1 * time.Hour)),
			},
			expected: true,
		},
		{
			name: "invalid lease format",
			state: &State{
				LeaseExpiresAt: "invalid",
			},
			expected: true, // Treat as expired
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LeaseExpired(tt.state)
			if result != tt.expected {
				t.Errorf("LeaseExpired() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestRenewLease(t *testing.T) {
	tests := []struct {
		name          string
		state         *State
		expectChanged bool
		expectLease   bool
	}{
		{
			name: "no task - clear lease",
			state: &State{
				WIP:            "",
				LeaseExpiresAt: "2025-01-01T00:00:00Z",
			},
			expectChanged: true,
			expectLease:   false,
		},
		{
			name: "task with no lease - set lease",
			state: &State{
				WIP:            "SBI-001",
				LeaseExpiresAt: "",
			},
			expectChanged: true,
			expectLease:   true,
		},
		{
			name: "task with existing lease - update",
			state: &State{
				WIP:            "SBI-001",
				LeaseExpiresAt: "2025-01-01T00:00:00Z",
			},
			expectChanged: true,
			expectLease:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := RenewLease(tt.state, 8*time.Minute)
			if changed != tt.expectChanged {
				t.Errorf("RenewLease() changed = %v, expected %v", changed, tt.expectChanged)
			}

			hasLease := tt.state.LeaseExpiresAt != ""
			if hasLease != tt.expectLease {
				t.Errorf("Lease set = %v, expected %v", hasLease, tt.expectLease)
			}

			if tt.expectLease && hasLease {
				// Verify lease is in the future
				expires, err := ParseRFC3339NanoUTC(tt.state.LeaseExpiresAt)
				if err != nil {
					t.Errorf("Failed to parse lease: %v", err)
				}
				if !expires.After(NowUTC()) {
					t.Error("Lease should be in the future")
				}
			}
		})
	}
}

func TestClearLease(t *testing.T) {
	state := &State{
		WIP:            "SBI-001",
		LeaseExpiresAt: "2025-01-01T00:00:00Z",
	}

	changed := ClearLease(state)
	if !changed {
		t.Error("Expected ClearLease to return true when clearing existing lease")
	}

	if state.LeaseExpiresAt != "" {
		t.Errorf("Expected lease to be cleared, got %s", state.LeaseExpiresAt)
	}

	// Test clearing already empty lease
	changed = ClearLease(state)
	if changed {
		t.Error("Expected ClearLease to return false when lease already empty")
	}
}

func TestFormatRFC3339NanoUTC(t *testing.T) {
	// Test zero time
	result := FormatRFC3339NanoUTC(time.Time{})
	if result != "" {
		t.Errorf("Expected empty string for zero time, got %s", result)
	}

	// Test valid time
	testTime := time.Date(2025, 9, 26, 12, 30, 45, 123456789, time.UTC)
	result = FormatRFC3339NanoUTC(testTime)
	expected := "2025-09-26T12:30:45.123456789Z"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestParseRFC3339NanoUTC(t *testing.T) {
	// Test empty string
	result, err := ParseRFC3339NanoUTC("")
	if err != nil {
		t.Errorf("Unexpected error for empty string: %v", err)
	}
	if !result.IsZero() {
		t.Error("Expected zero time for empty string")
	}

	// Test valid timestamp
	input := "2025-09-26T12:30:45.123456789Z"
	result, err = ParseRFC3339NanoUTC(input)
	if err != nil {
		t.Errorf("Failed to parse valid timestamp: %v", err)
	}

	if result.Year() != 2025 || result.Month() != 9 || result.Day() != 26 {
		t.Errorf("Parsed time incorrect: %v", result)
	}

	// Test invalid timestamp
	_, err = ParseRFC3339NanoUTC("invalid")
	if err == nil {
		t.Error("Expected error for invalid timestamp")
	}
}
