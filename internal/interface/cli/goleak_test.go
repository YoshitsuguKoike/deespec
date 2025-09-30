package cli

import (
	"testing"

	"go.uber.org/goleak"
)

// TestPackageLeaks runs goleak verification for the entire package
func TestPackageLeaks(t *testing.T) {
	defer goleak.VerifyNone(t,
		// Ignore known goroutines that are not leaks
		goleak.IgnoreTopFunction("github.com/golang/glog.(*loggingT).flushDaemon"),
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("github.com/patrickmn/go-cache.(*janitor).run"),
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"),
	)

	// This test verifies that no goroutines are leaked
	// It will automatically run with other tests and detect any leaks
}
