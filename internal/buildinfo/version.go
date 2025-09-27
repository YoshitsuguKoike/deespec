// Package buildinfo contains build-time information embedded via ldflags
package buildinfo

// Version is the application version, set at build time via ldflags
// Example: go build -ldflags "-X github.com/YoshitsuguKoike/deespec/internal/buildinfo.Version=v1.0.0"
var Version = "dev"

// GetVersion returns the current version, with "dev" as default for development builds
func GetVersion() string {
	if Version == "" {
		return "dev"
	}
	return Version
}
