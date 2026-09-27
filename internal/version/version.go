// Package version holds build information set at link time.
package version

// Version is the module version, overridden by -ldflags at release time.
var Version = "dev"

// Commit is the short git commit, overridden by -ldflags at release time.
var Commit = "none"
