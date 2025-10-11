package lock

import (
	"testing"
)

// ==================== LockID Tests ====================

func TestNewLockID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"Valid ID", "sbi-123", false},
		{"Valid UUID", "550e8400-e29b-41d4-a716-446655440000", false},
		{"Valid path", "path/to/state.json", false},
		{"Empty ID", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := NewLockID(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLockID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && id.String() != tt.value {
				t.Errorf("Expected ID %s, got %s", tt.value, id.String())
			}
		})
	}
}

func TestLockID_String(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"Simple ID", "test-123"},
		{"UUID format", "550e8400-e29b-41d4-a716-446655440000"},
		{"Path format", "var/lock/state.json"},
		{"Special characters", "lock:sbi:970fee40-8cc8-49a0-af57-22b720fc678a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := NewLockID(tt.value)
			if err != nil {
				t.Fatalf("NewLockID() unexpected error: %v", err)
			}
			if id.String() != tt.value {
				t.Errorf("String() = %v, want %v", id.String(), tt.value)
			}
		})
	}
}

func TestLockID_Equals(t *testing.T) {
	id1, _ := NewLockID("test-123")
	id2, _ := NewLockID("test-123")
	id3, _ := NewLockID("test-456")

	tests := []struct {
		name   string
		id1    LockID
		id2    LockID
		equals bool
	}{
		{"Same IDs should be equal", id1, id2, true},
		{"Different IDs should not be equal", id1, id3, false},
		{"Same reference should be equal", id1, id1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.id1.Equals(tt.id2); result != tt.equals {
				t.Errorf("Equals() = %v, want %v", result, tt.equals)
			}
		})
	}
}

func TestLockID_EmptyValidation(t *testing.T) {
	_, err := NewLockID("")
	if err == nil {
		t.Error("NewLockID(\"\") should return error for empty ID")
	}

	expectedMsg := "lock ID cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("Error message = %v, want %v", err.Error(), expectedMsg)
	}
}

func TestLockID_Immutability(t *testing.T) {
	// LockID is a value object and should be immutable
	value := "original-value"
	id, _ := NewLockID(value)

	// Get the string representation
	str1 := id.String()

	// Get it again
	str2 := id.String()

	// Should be the same
	if str1 != str2 {
		t.Error("LockID String() should return consistent values")
	}

	if str1 != value {
		t.Errorf("LockID value changed: got %s, want %s", str1, value)
	}
}

func TestLockID_LongValues(t *testing.T) {
	// Test with very long lock IDs
	tests := []struct {
		name  string
		value string
	}{
		{"Long UUID path", "very/long/path/to/state/file/with/uuid/550e8400-e29b-41d4-a716-446655440000/state.json"},
		{"Repeated pattern", "lock-" + string(make([]byte, 1000))},
		{"256 chars", string(make([]byte, 256))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == "" {
				return // Skip empty test
			}
			id, err := NewLockID(tt.value)
			if err != nil {
				t.Fatalf("NewLockID() unexpected error: %v", err)
			}
			if id.String() != tt.value {
				t.Errorf("String() length mismatch: got %d, want %d", len(id.String()), len(tt.value))
			}
		})
	}
}

func TestLockID_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"Unicode", "ロック-ID-テスト"},
		{"Mixed special chars", "lock:id/with\\special@chars#123"},
		{"Whitespace", "lock with spaces and\ttabs"},
		{"Newlines", "lock\nwith\nnewlines"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := NewLockID(tt.value)
			if err != nil {
				t.Fatalf("NewLockID() unexpected error: %v", err)
			}
			if id.String() != tt.value {
				t.Errorf("String() = %v, want %v", id.String(), tt.value)
			}
		})
	}
}

// ==================== Error Tests ====================

func TestErrLockNotFound(t *testing.T) {
	if ErrLockNotFound == nil {
		t.Error("ErrLockNotFound should not be nil")
	}

	expectedMsg := "lock not found"
	if ErrLockNotFound.Error() != expectedMsg {
		t.Errorf("ErrLockNotFound.Error() = %v, want %v", ErrLockNotFound.Error(), expectedMsg)
	}
}

// ==================== Benchmark Tests ====================

func BenchmarkNewLockID(b *testing.B) {
	value := "test-lock-id-123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewLockID(value)
	}
}

func BenchmarkLockID_String(b *testing.B) {
	id, _ := NewLockID("test-lock-id-123")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
}

func BenchmarkLockID_Equals(b *testing.B) {
	id1, _ := NewLockID("test-lock-id-123")
	id2, _ := NewLockID("test-lock-id-123")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id1.Equals(id2)
	}
}
