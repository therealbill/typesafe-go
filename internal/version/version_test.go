package version

import (
	"runtime/debug"
	"testing"
)

func withBuildInfo(t *testing.T, bi *debug.BuildInfo, ok bool) {
	t.Helper()
	prev := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return bi, ok }
	t.Cleanup(func() { readBuildInfo = prev })
}

func withLdflags(t *testing.T, v, c string) {
	t.Helper()
	pv, pc := Version, Commit
	Version, Commit = v, c
	t.Cleanup(func() { Version, Commit = pv, pc })
}

func TestGetPrefersLdflags(t *testing.T) {
	withLdflags(t, "v9.9.9", "abc1234")
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v0.0.1"}}, true)
	got := Get()
	if got.Version != "v9.9.9" || got.Commit != "abc1234" || got.Source != "ldflags" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetUsesModuleVersion(t *testing.T) {
	withLdflags(t, "dev", "none")
	withBuildInfo(t, &debug.BuildInfo{
		Main:     debug.Module{Version: "v0.1.0"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef0123"}, {Key: "vcs.modified", Value: "false"}},
	}, true)
	got := Get()
	if got.Version != "v0.1.0" || got.Commit != "0123456" || got.Modified || got.Source != "module" {
		t.Fatalf("got %+v", got)
	}
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20260929120000-0123456789ab"}}, true)
	got = Get()
	if got.Version != "v0.0.0-20260929120000-0123456789ab" || got.Commit != "none" || got.Source != "module" {
		t.Fatalf("pseudo-version: got %+v", got)
	}
}

func TestGetUsesVCSForDevelBuilds(t *testing.T) {
	withLdflags(t, "dev", "none")
	withBuildInfo(t, &debug.BuildInfo{
		Main:     debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "fedcba9876543210"}, {Key: "vcs.modified", Value: "true"}},
	}, true)
	got := Get()
	if got.Version != "(devel)" || got.Commit != "fedcba9" || !got.Modified || got.Source != "vcs" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetUnknown(t *testing.T) {
	withLdflags(t, "dev", "none")
	withBuildInfo(t, nil, false)
	if got := Get(); got.Version != "dev" || got.Commit != "none" || got.Source != "unknown" {
		t.Fatalf("no build info: got %+v", got)
	}
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true)
	if got := Get(); got.Version != "dev" || got.Source != "unknown" {
		t.Fatalf("devel without vcs: got %+v", got)
	}
}
