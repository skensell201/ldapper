package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDirIsUnderTheUserConfigDirectory(t *testing.T) {
	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir() returned error: %v", err)
	}
	if !strings.HasSuffix(got, appName) {
		t.Errorf("Dir() = %q, want it to end in %q", got, appName)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("Dir() = %q, want an absolute path", got)
	}
}

func TestFilePathsSitSideBySide(t *testing.T) {
	filters, err := FiltersPath()
	if err != nil {
		t.Fatalf("FiltersPath() returned error: %v", err)
	}
	connections, err := ConnectionsPath()
	if err != nil {
		t.Fatalf("ConnectionsPath() returned error: %v", err)
	}

	if filepath.Dir(filters) != filepath.Dir(connections) {
		t.Errorf("the two files are in different directories: %q and %q", filters, connections)
	}
	if filepath.Base(filters) != "filters.json" {
		t.Errorf("FiltersPath() = %q, want it to end in filters.json", filters)
	}
	if filepath.Base(connections) != "connections.json" {
		t.Errorf("ConnectionsPath() = %q, want it to end in connections.json", connections)
	}
}

// An override keeps tests and portable installs off the real configuration,
// and it is used exactly as given — no application name appended.
func TestOverrideIsUsedVerbatim(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(dirEnv, dir)

	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir() returned error: %v", err)
	}
	if got != dir {
		t.Errorf("Dir() = %q, want the override %q", got, dir)
	}

	filters, err := FiltersPath()
	if err != nil {
		t.Fatalf("FiltersPath() returned error: %v", err)
	}
	if filters != filepath.Join(dir, "filters.json") {
		t.Errorf("FiltersPath() = %q, want it under the override", filters)
	}
}

func TestEmptyOverrideIsIgnored(t *testing.T) {
	t.Setenv(dirEnv, "")

	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir() returned error: %v", err)
	}
	if !strings.HasSuffix(got, appName) {
		t.Errorf("Dir() = %q, want the platform location when the override is empty", got)
	}
}
