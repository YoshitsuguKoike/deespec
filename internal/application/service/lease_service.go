package service

import (
	"time"
)

// Default TTL for lease (8 minutes as per spec)
const DefaultLeaseTTL = 8 * time.Minute

// LeaseService handles lease management for work-in-progress tasks
type LeaseService struct{}

// NewLeaseService creates a new lease service
func NewLeaseService() *LeaseService {
	return &LeaseService{}
}

// NowUTC returns the current UTC time
func (s *LeaseService) NowUTC() time.Time {
	return time.Now().UTC()
}

// ParseRFC3339NanoUTC parses a UTC RFC3339Nano timestamp
// This function handles both UTC and local timezone formats
func (s *LeaseService) ParseRFC3339NanoUTC(str string) (time.Time, error) {
	if str == "" {
		return time.Time{}, nil
	}
	// Parse the time (handles both UTC and timezone offset formats)
	t, err := time.Parse(time.RFC3339Nano, str)
	if err != nil {
		return time.Time{}, err
	}
	// Always return as UTC for consistent comparison
	return t.UTC(), nil
}

// FormatRFC3339NanoUTC formats a time as UTC RFC3339Nano
func (s *LeaseService) FormatRFC3339NanoUTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// FormatRFC3339NanoLocal formats a time in local timezone with offset
// Example: 2025-09-30T15:30:00.123456789+09:00 (JST)
func (s *LeaseService) FormatRFC3339NanoLocal(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	// Convert to local time and format with timezone offset
	return t.Local().Format(time.RFC3339Nano)
}

// RenewLease updates the lease expiration time if there's a current task
// Returns true if the lease was updated
func (s *LeaseService) RenewLease(wip string, currentLeaseExpiresAt string, ttl time.Duration) (newLeaseExpiresAt string, updated bool) {
	if wip == "" {
		// No task in progress, clear lease
		if currentLeaseExpiresAt != "" {
			return "", true
		}
		return currentLeaseExpiresAt, false
	}

	// Set new lease expiration (stored in local timezone with offset)
	// This maintains compatibility while showing local time
	newExpiry := s.FormatRFC3339NanoLocal(s.NowUTC().Add(ttl))
	if currentLeaseExpiresAt != newExpiry {
		return newExpiry, true
	}
	return currentLeaseExpiresAt, false
}

// LeaseExpired checks if the lease has expired
func (s *LeaseService) LeaseExpired(leaseExpiresAt string) bool {
	if leaseExpiresAt == "" {
		// No lease set
		return false
	}

	expiresAt, err := s.ParseRFC3339NanoUTC(leaseExpiresAt)
	if err != nil {
		// Invalid timestamp, treat as expired
		return true
	}

	return s.NowUTC().After(expiresAt)
}

// ClearLease removes the lease when a task completes
// Returns the cleared value and true if the lease was cleared
func (s *LeaseService) ClearLease(currentLeaseExpiresAt string) (newLeaseExpiresAt string, cleared bool) {
	if currentLeaseExpiresAt != "" {
		return "", true
	}
	return currentLeaseExpiresAt, false
}
