// Package buildinfo holds build metadata injected via -ldflags at release time.
package buildinfo

var (
	Version = "dev"     // -X .../internal/buildinfo.Version
	Commit  = "none"    // -X .../internal/buildinfo.Commit
	Date    = "unknown" // -X .../internal/buildinfo.Date
)
