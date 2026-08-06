package app

import "runtime/debug"

// version is stamped at build time:
//
//	-ldflags "-X github.com/skensell201/ldapper/app.version=v0.4.0"
//
// A build made without it says so rather than claiming a release it is not.
var version = "dev"

// BuildInfo is what the status bar shows and what a bug report should quote.
type BuildInfo struct {
	Version string `json:"version"`
	// Commit is the revision Go stamps into the binary automatically. Empty
	// when the source was not a git checkout.
	Commit string `json:"commit"`
	// Modified is true when the working tree had uncommitted changes, which
	// makes the commit alone misleading.
	Modified bool `json:"modified"`
}

// Build reports which Ldapper this is.
func (a *App) Build() BuildInfo { return buildInfo() }

func buildInfo() BuildInfo {
	out := BuildInfo{Version: version}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return out
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) > 7 {
				out.Commit = s.Value[:7]
			} else {
				out.Commit = s.Value
			}
		case "vcs.modified":
			out.Modified = s.Value == "true"
		}
	}
	return out
}
