package app

import (
	"strings"
	"testing"
)

func TestBuildAlwaysSaysSomething(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	got := a.Build()
	if got.Version == "" {
		t.Error("Version is empty; a build has to be able to identify itself")
	}
}

// A build made without the stamp must say so rather than claim a release it
// is not. "dev" in a bug report is information; "v0.0.0" is a lie.
func TestUnstampedBuildSaysDev(t *testing.T) {
	if version != "dev" && !strings.HasPrefix(version, "v") {
		t.Errorf("version = %q, want either the default or a tag", version)
	}
}

func TestCommitIsShortened(t *testing.T) {
	got := buildInfo()
	if len(got.Commit) > 7 {
		t.Errorf("Commit = %q, want it shortened to seven characters", got.Commit)
	}
}
