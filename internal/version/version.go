// Package version reports which build of codec is running. Release
// builds have the values below stamped in by the linker (see
// .goreleaser.yaml); every other build falls back to what the Go
// toolchain recorded about the module and the git checkout.
package version

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
)

// pseudoVersion matches the timestamp-and-hash tail Go puts on
// versions of untagged commits: -YYYYMMDDhhmmss-abcdefabcdef.
var pseudoVersion = regexp.MustCompile(`\d{14}-[0-9a-f]{12}$`)

// Set at link time by release builds, e.g.
//
//	go build -ldflags "-X github.com/mahasenabheetha/codec/v2/internal/version.Version=v1.0.0"
//
// They are vars rather than consts because -X can only overwrite
// package-level string variables.
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// Info is the resolved identity of the running build.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"date,omitempty"`
	Dirty   bool   `json:"dirty,omitempty"` // built from uncommitted changes
}

// Get resolves the build identity: linker-stamped values win, and
// anything left empty is filled from the toolchain's build info.
func Get() Info {
	stamped := Info{Version: Version, Commit: Commit, Date: Date}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return stamped
	}
	return resolve(stamped, bi)
}

// resolve is the pure half of Get, split out so the fallback rules
// can be table-tested without depending on how the test binary was
// built.
func resolve(info Info, bi *debug.BuildInfo) Info {
	// `go install ...@v1.0.0`, or a `go build` on a tagged commit,
	// records a real version. Untagged builds record "(devel)" or a
	// pseudo-version (v0.0.0-20260819145859-587c127e510b), which only
	// repeats the commit shown separately, so those stay "dev". The
	// "+dirty" suffix is dropped because Dirty reports it already.
	if mv := strings.TrimSuffix(bi.Main.Version, "+dirty"); info.Version == "dev" &&
		mv != "" && mv != "(devel)" && !pseudoVersion.MatchString(mv) {
		info.Version = mv
	}

	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if info.Commit == "" {
				info.Commit = s.Value
			}
		case "vcs.time":
			if info.Date == "" {
				info.Date = s.Value
			}
		case "vcs.modified":
			info.Dirty = s.Value == "true"
		}
	}
	return info
}

// String renders Info for humans, e.g.
// "v1.0.0 (commit 587c127, built 2026-09-26T10:00:00Z)".
func (i Info) String() string {
	var details []string
	if i.Commit != "" {
		details = append(details, "commit "+shortCommit(i.Commit))
	}
	if i.Date != "" {
		details = append(details, "built "+i.Date)
	}
	if i.Dirty {
		details = append(details, "modified")
	}
	if len(details) == 0 {
		return i.Version
	}
	return fmt.Sprintf("%s (%s)", i.Version, strings.Join(details, ", "))
}

// shortCommit trims a full SHA to the 7 characters git itself shows.
func shortCommit(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
