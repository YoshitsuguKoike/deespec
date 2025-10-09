package common

import (
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/service"
)

// Default TTL for lease (8 minutes as per spec)
const DefaultLeaseTTL = service.DefaultLeaseTTL

// Global lease service instance
var leaseService = service.NewLeaseService()

// NowUTC returns the current UTC time
func NowUTC() time.Time {
	return leaseService.NowUTC()
}

// ParseRFC3339NanoUTC parses a UTC RFC3339Nano timestamp
// This function handles both UTC and local timezone formats
func ParseRFC3339NanoUTC(s string) (time.Time, error) {
	return leaseService.ParseRFC3339NanoUTC(s)
}

// FormatRFC3339NanoUTC formats a time as UTC RFC3339Nano
func FormatRFC3339NanoUTC(t time.Time) string {
	return leaseService.FormatRFC3339NanoUTC(t)
}

// FormatRFC3339NanoLocal formats a time in local timezone with offset
// Example: 2025-09-30T15:30:00.123456789+09:00 (JST)
func FormatRFC3339NanoLocal(t time.Time) string {
	return leaseService.FormatRFC3339NanoLocal(t)
}

// RenewLease updates the lease expiration time if there's a current task
func RenewLease(st *State, ttl time.Duration) bool {
	newLeaseExpiresAt, updated := leaseService.RenewLease(st.WIP, st.LeaseExpiresAt, ttl)
	if updated {
		st.LeaseExpiresAt = newLeaseExpiresAt
	}
	return updated
}

// LeaseExpired checks if the lease has expired
func LeaseExpired(st *State) bool {
	return leaseService.LeaseExpired(st.LeaseExpiresAt)
}

// ClearLease removes the lease when a task completes
func ClearLease(st *State) bool {
	newLeaseExpiresAt, cleared := leaseService.ClearLease(st.LeaseExpiresAt)
	if cleared {
		st.LeaseExpiresAt = newLeaseExpiresAt
	}
	return cleared
}
