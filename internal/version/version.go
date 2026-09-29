// Package version reports how the binary was built.
package version

import "runtime/debug"

// Version is set by -ldflags at release time and by make build. "dev" means
// it was not set.
var Version = "dev"

// Commit is the short git commit set by -ldflags. "none" means it was not set.
var Commit = "none"

// Info is the resolved build information.
type Info struct {
	// Version is a tag such as v0.1.0, a pseudo-version, "(devel)" for a
	// build from a clone, or "dev" when nothing is known.
	Version string `json:"version"`
	// Commit is the short revision, or "none".
	Commit string `json:"commit"`
	// Modified is true when the build came from a tree with uncommitted changes.
	Modified bool `json:"modified"`
	// Source says where Version came from: ldflags, module, vcs, or unknown.
	Source string `json:"source"`
}

var readBuildInfo = debug.ReadBuildInfo

// Get resolves build information. ldflags win; otherwise the module version
// recorded by go install or go build in module mode is used; otherwise the
// VCS revision embedded in a build from a clone; otherwise "dev".
func Get() Info {
	if Version != "dev" {
		return Info{Version: Version, Commit: Commit, Source: "ldflags"}
	}
	bi, ok := readBuildInfo()
	if !ok || bi == nil {
		return Info{Version: Version, Commit: Commit, Source: "unknown"}
	}
	rev, modified := "", false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	short := rev
	if len(short) > 7 {
		short = short[:7]
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		if short == "" {
			short = "none"
		}
		return Info{Version: v, Commit: short, Modified: modified, Source: "module"}
	}
	if rev != "" {
		return Info{Version: "(devel)", Commit: short, Modified: modified, Source: "vcs"}
	}
	return Info{Version: Version, Commit: Commit, Source: "unknown"}
}
