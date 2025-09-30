package cli

import (
	"time"
)

// Default TTL for lease (8 minutes as per spec)
const DefaultLeaseTTL = 8 * time.Minute

// NowUTC returns the current UTC time
func NowUTC() time.Time {
	return time.Now().UTC()
}

// ParseRFC3339NanoUTC parses a UTC RFC3339Nano timestamp
// This function handles both UTC and local timezone formats
func ParseRFC3339NanoUTC(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	// Parse the time (handles both UTC and timezone offset formats)
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, err
	}
	// Always return as UTC for consistent comparison
	return t.UTC(), nil
}

// FormatRFC3339NanoUTC formats a time as UTC RFC3339Nano
func FormatRFC3339NanoUTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// FormatRFC3339NanoLocal formats a time in local timezone with offset
// Example: 2025-09-30T15:30:00.123456789+09:00 (JST)
func FormatRFC3339NanoLocal(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	// Convert to local time and format with timezone offset
	return t.Local().Format(time.RFC3339Nano)
}

// RenewLease updates the lease expiration time if there's a current task
func RenewLease(st *State, ttl time.Duration) bool {
	if st.WIP == "" {
		// No task in progress, clear lease
		if st.LeaseExpiresAt != "" {
			st.LeaseExpiresAt = ""
			return true
		}
		return false
	}

	// Set new lease expiration (stored in local timezone with offset)
	// This maintains compatibility while showing local time
	newExpiry := FormatRFC3339NanoLocal(NowUTC().Add(ttl))
	if st.LeaseExpiresAt != newExpiry {
		st.LeaseExpiresAt = newExpiry
		return true
	}
	return false
}

// LeaseExpired checks if the lease has expired
func LeaseExpired(st *State) bool {
	if st.LeaseExpiresAt == "" {
		// No lease set
		return false
	}

	expiresAt, err := ParseRFC3339NanoUTC(st.LeaseExpiresAt)
	if err != nil {
		// Invalid timestamp, treat as expired
		return true
	}

	return NowUTC().After(expiresAt)
}

// ClearLease removes the lease when a task completes
func ClearLease(st *State) bool {
	if st.LeaseExpiresAt != "" {
		st.LeaseExpiresAt = ""
		return true
	}
	return false
}
