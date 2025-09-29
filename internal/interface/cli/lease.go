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
func ParseRFC3339NanoUTC(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

// FormatRFC3339NanoUTC formats a time as UTC RFC3339Nano
func FormatRFC3339NanoUTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
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

	// Set new lease expiration
	newExpiry := FormatRFC3339NanoUTC(NowUTC().Add(ttl))
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
