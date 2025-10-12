package pbi

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// GenerateID generates a new PBI ID using ULID
// Format: ULID (e.g., 01JB6X8Y2K9FQR4T3VWHGP5M2C)
func GenerateID(repo Repository) (string, error) {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	return id.String(), nil
}
