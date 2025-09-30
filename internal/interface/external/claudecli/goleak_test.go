package claudecli

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain runs goleak verification for all tests in this package
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestPackageLeaks verifies no goroutines are leaked
func TestPackageLeaks(t *testing.T) {
	defer goleak.VerifyNone(t)
	// This test will detect any goroutine leaks in the package
}
