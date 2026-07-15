// Package meta provides centralized application metadata for Dockyard.
//
// Version is injected at build time via -ldflags "-X ...".
// If unset, it falls back to "v0.0.0-dev".
package meta

var (
	// Version is the compile-time set version of Dockyard.
	Version = "v0.0.0-dev"

	// UserAgent is the HTTP client identifier derived from Version.
	UserAgent string
)

func init() {
	UserAgent = "Dockyard/" + Version
}
