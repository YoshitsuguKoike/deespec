package model

import (
	"strings"
	"testing"
	"time"
)

// ==================== TaskID Tests ====================

func TestNewTaskID(t *testing.T) {
	id1 := NewTaskID()
	id2 := NewTaskID()

	if id1.String() == "" {
		t.Error("TaskID should not be empty")
	}

	if id1.String() == id2.String() {
		t.Error("Different TaskIDs should have different values")
	}

	// ULID format check (basic)
	if len(id1.String()) != 26 {
		t.Errorf("TaskID should be 26 characters (ULID format), got %d", len(id1.String()))
	}
}

func TestNewTaskIDFromString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Valid ID", "test-123", false},
		{"Empty ID", "", true},
		{"UUID format", "550e8400-e29b-41d4-a716-446655440000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := NewTaskIDFromString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTaskIDFromString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && id.String() != tt.input {
				t.Errorf("Expected ID %s, got %s", tt.input, id.String())
			}
		})
	}
}

func TestTaskID_Equals(t *testing.T) {
	id1, _ := NewTaskIDFromString("test-123")
	id2, _ := NewTaskIDFromString("test-123")
	id3, _ := NewTaskIDFromString("test-456")

	if !id1.Equals(id2) {
		t.Error("Same IDs should be equal")
	}

	if id1.Equals(id3) {
		t.Error("Different IDs should not be equal")
	}
}

// ==================== TaskType Tests ====================

func TestTaskType_String(t *testing.T) {
	tests := []struct {
		taskType TaskType
		expected string
	}{
		{TaskTypeEPIC, "EPIC"},
		{TaskTypePBI, "PBI"},
		{TaskTypeSBI, "SBI"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.taskType.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.taskType.String())
			}
		})
	}
}

func TestTaskType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		taskType TaskType
		valid    bool
	}{
		{"EPIC is valid", TaskTypeEPIC, true},
		{"PBI is valid", TaskTypePBI, true},
		{"SBI is valid", TaskTypeSBI, true},
		{"Invalid type", TaskType("INVALID"), false},
		{"Empty type", TaskType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.taskType.IsValid() != tt.valid {
				t.Errorf("Expected IsValid() = %v for %s", tt.valid, tt.taskType)
			}
		})
	}
}

// ==================== Status Tests ====================

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusPending, "PENDING"},
		{StatusPicked, "PICKED"},
		{StatusImplementing, "IMPLEMENTING"},
		{StatusReviewing, "REVIEWING"},
		{StatusDone, "DONE"},
		{StatusFailed, "FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.status.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.status.String())
			}
		})
	}
}

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		valid  bool
	}{
		{"Pending is valid", StatusPending, true},
		{"Picked is valid", StatusPicked, true},
		{"Implementing is valid", StatusImplementing, true},
		{"Reviewing is valid", StatusReviewing, true},
		{"Done is valid", StatusDone, true},
		{"Failed is valid", StatusFailed, true},
		{"Invalid status", Status("INVALID"), false},
		{"Empty status", Status(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.status.IsValid() != tt.valid {
				t.Errorf("Expected IsValid() = %v for %s", tt.valid, tt.status)
			}
		})
	}
}

func TestStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name       string
		from       Status
		to         Status
		canTransit bool
	}{
		// Valid transitions
		{"Pending to Picked", StatusPending, StatusPicked, true},
		{"Picked to Implementing", StatusPicked, StatusImplementing, true},
		{"Picked to Pending", StatusPicked, StatusPending, true},
		{"Implementing to Reviewing", StatusImplementing, StatusReviewing, true},
		{"Implementing to Failed", StatusImplementing, StatusFailed, true},
		{"Implementing to Pending", StatusImplementing, StatusPending, true},
		{"Reviewing to Done", StatusReviewing, StatusDone, true},
		{"Reviewing to Implementing", StatusReviewing, StatusImplementing, true},
		{"Reviewing to Failed", StatusReviewing, StatusFailed, true},
		{"Failed to Pending", StatusFailed, StatusPending, true},

		// Invalid transitions
		{"Pending to Implementing", StatusPending, StatusImplementing, false},
		{"Pending to Done", StatusPending, StatusDone, false},
		{"Done to anything", StatusDone, StatusPending, false},
		{"Done to Picked", StatusDone, StatusPicked, false},
		{"Picked to Done", StatusPicked, StatusDone, false},
		{"Failed to Done", StatusFailed, StatusDone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			if result != tt.canTransit {
				t.Errorf("CanTransitionTo(%s -> %s) = %v, want %v",
					tt.from, tt.to, result, tt.canTransit)
			}
		})
	}
}

// ==================== Step Tests ====================

func TestStep_String(t *testing.T) {
	tests := []struct {
		step     Step
		expected string
	}{
		{StepPick, "PICK"},
		{StepImplement, "IMPLEMENT"},
		{StepReview, "REVIEW"},
		{StepDone, "DONE"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.step.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.step.String())
			}
		})
	}
}

func TestStep_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		step  Step
		valid bool
	}{
		{"Pick is valid", StepPick, true},
		{"Implement is valid", StepImplement, true},
		{"Review is valid", StepReview, true},
		{"Done is valid", StepDone, true},
		{"Invalid step", Step("INVALID"), false},
		{"Empty step", Step(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.step.IsValid() != tt.valid {
				t.Errorf("Expected IsValid() = %v for %s", tt.valid, tt.step)
			}
		})
	}
}

// ==================== Turn Tests ====================

func TestNewTurn(t *testing.T) {
	turn := NewTurn()
	if turn.Value() != 0 {
		t.Errorf("NewTurn() should start at 0, got %d", turn.Value())
	}
}

func TestNewTurnFromInt(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{"Zero is valid", 0, false},
		{"Positive is valid", 5, false},
		{"Large number", 100, false},
		{"Negative is invalid", -1, true},
		{"Large negative", -100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			turn, err := NewTurnFromInt(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTurnFromInt(%d) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if !tt.wantErr && turn.Value() != tt.value {
				t.Errorf("Expected value %d, got %d", tt.value, turn.Value())
			}
		})
	}
}

func TestTurn_Increment(t *testing.T) {
	turn := NewTurn()
	turn = turn.Increment()

	if turn.Value() != 1 {
		t.Errorf("After increment, expected 1, got %d", turn.Value())
	}

	turn = turn.Increment()
	if turn.Value() != 2 {
		t.Errorf("After second increment, expected 2, got %d", turn.Value())
	}
}

func TestTurn_Equals(t *testing.T) {
	turn1, _ := NewTurnFromInt(5)
	turn2, _ := NewTurnFromInt(5)
	turn3, _ := NewTurnFromInt(10)

	if !turn1.Equals(turn2) {
		t.Error("Turns with same value should be equal")
	}

	if turn1.Equals(turn3) {
		t.Error("Turns with different values should not be equal")
	}
}

func TestTurn_String(t *testing.T) {
	turn, _ := NewTurnFromInt(42)
	expected := "Turn 42"
	if turn.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, turn.String())
	}
}

// ==================== Attempt Tests ====================

func TestNewAttempt(t *testing.T) {
	attempt := NewAttempt()
	if attempt.Value() != 1 {
		t.Errorf("NewAttempt() should start at 1, got %d", attempt.Value())
	}
}

func TestNewAttemptFromInt(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{"One is valid", 1, false},
		{"Positive is valid", 5, false},
		{"Large number", 100, false},
		{"Zero is invalid", 0, true},
		{"Negative is invalid", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt, err := NewAttemptFromInt(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAttemptFromInt(%d) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if !tt.wantErr && attempt.Value() != tt.value {
				t.Errorf("Expected value %d, got %d", tt.value, attempt.Value())
			}
		})
	}
}

func TestAttempt_Increment(t *testing.T) {
	attempt := NewAttempt()
	attempt = attempt.Increment()

	if attempt.Value() != 2 {
		t.Errorf("After increment, expected 2, got %d", attempt.Value())
	}

	attempt = attempt.Increment()
	if attempt.Value() != 3 {
		t.Errorf("After second increment, expected 3, got %d", attempt.Value())
	}
}

func TestAttempt_Equals(t *testing.T) {
	attempt1, _ := NewAttemptFromInt(3)
	attempt2, _ := NewAttemptFromInt(3)
	attempt3, _ := NewAttemptFromInt(5)

	if !attempt1.Equals(attempt2) {
		t.Error("Attempts with same value should be equal")
	}

	if attempt1.Equals(attempt3) {
		t.Error("Attempts with different values should not be equal")
	}
}

func TestAttempt_String(t *testing.T) {
	attempt, _ := NewAttemptFromInt(7)
	expected := "Attempt 7"
	if attempt.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, attempt.String())
	}
}

// ==================== Timestamp Tests ====================

func TestNewTimestamp(t *testing.T) {
	before := time.Now()
	ts := NewTimestamp()
	after := time.Now()

	if ts.Value().Before(before) || ts.Value().After(after) {
		t.Error("Timestamp should be between before and after time")
	}
}

func TestNewTimestampFromTime(t *testing.T) {
	specificTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := NewTimestampFromTime(specificTime)

	if !ts.Value().Equal(specificTime) {
		t.Errorf("Expected time %v, got %v", specificTime, ts.Value())
	}
}

func TestTimestamp_Before(t *testing.T) {
	time1 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	time2 := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)

	ts1 := NewTimestampFromTime(time1)
	ts2 := NewTimestampFromTime(time2)

	if !ts1.Before(ts2) {
		t.Error("Earlier timestamp should be before later timestamp")
	}

	if ts2.Before(ts1) {
		t.Error("Later timestamp should not be before earlier timestamp")
	}
}

func TestTimestamp_After(t *testing.T) {
	time1 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	time2 := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)

	ts1 := NewTimestampFromTime(time1)
	ts2 := NewTimestampFromTime(time2)

	if !ts2.After(ts1) {
		t.Error("Later timestamp should be after earlier timestamp")
	}

	if ts1.After(ts2) {
		t.Error("Earlier timestamp should not be after later timestamp")
	}
}

func TestTimestamp_String(t *testing.T) {
	specificTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := NewTimestampFromTime(specificTime)

	str := ts.String()

	// RFC3339 format check
	if !strings.Contains(str, "2025-01-01") {
		t.Errorf("Timestamp string should contain date, got %s", str)
	}

	// Should be parseable as RFC3339
	_, err := time.Parse(time.RFC3339, str)
	if err != nil {
		t.Errorf("Timestamp string should be valid RFC3339 format, got %s: %v", str, err)
	}
}

func TestTimestamp_Ordering(t *testing.T) {
	// Test complete ordering
	time1 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	time2 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC) // Same
	time3 := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)

	ts1 := NewTimestampFromTime(time1)
	ts2 := NewTimestampFromTime(time2)
	ts3 := NewTimestampFromTime(time3)

	// ts1 and ts2 are equal
	if ts1.Before(ts2) || ts1.After(ts2) {
		t.Error("Equal timestamps should not be before or after each other")
	}

	// ts1 < ts3
	if !ts1.Before(ts3) {
		t.Error("ts1 should be before ts3")
	}

	// ts3 > ts1
	if !ts3.After(ts1) {
		t.Error("ts3 should be after ts1")
	}
}
