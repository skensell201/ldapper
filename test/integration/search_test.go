//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/skensell201/ldapper/app"
)

// StartSearch emits its results as events, which need a window. Without one
// the run still happens and still finishes — this checks the parts that do not
// depend on a window: validation, expansion, and the columns it settles on.
func TestFacadeStartSearchValidatesBeforeRunning(t *testing.T) {
	a, id := facade(t)

	got := a.StartSearch(app.SearchInput{ProfileID: id, Filter: "(objectClass=inetOrgPerson"})
	if got.Started {
		t.Error("Started = true for an unbalanced filter")
	}
	if got.Error == "" {
		t.Error("Error is empty; the filter must never reach the server")
	}
}

func TestFacadeStartSearchExpandsSubstitutions(t *testing.T) {
	a, id := facade(t)

	got := a.StartSearch(app.SearchInput{
		ProfileID: id,
		Filter:    "(createTimestamp<={{now:generalized}})",
		Scope:     "subtree",
	})
	if !got.Started {
		t.Fatalf("Started = false: %s", got.Error)
	}
	if strings.Contains(got.Expanded, "{{") {
		t.Errorf("Expanded = %q, still holds a substitution", got.Expanded)
	}
	if got.Columns[len(got.Columns)-1] != "distinguishedName" {
		t.Errorf("Columns = %v, want the DN last", got.Columns)
	}

	a.StopSearch(id)
}

// Stopping a search that is running must not hang or panic.
func TestFacadeStopSearch(t *testing.T) {
	a, id := facade(t)

	if got := a.StartSearch(app.SearchInput{ProfileID: id, Filter: "(objectClass=*)"}); !got.Started {
		t.Fatalf("Started = false: %s", got.Error)
	}
	a.StopSearch(id)
	a.StopSearch(id) // twice is not an error
}

func TestFacadeExportsLDIF(t *testing.T) {
	a, id := facade(t)
	path := filepath.Join(t.TempDir(), "people.ldif")

	if msg := a.ExportSearch(app.ExportInput{
		ProfileID: id,
		Path:      path,
		Filter:    "(objectClass=inetOrgPerson)",
		Scope:     "subtree",
		Base:      rootDN,
		Columns:   []string{"cn", "uid"},
	}); msg != "" {
		t.Fatalf("ExportSearch() = %q", msg)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	if !strings.HasPrefix(text, "version: 1") {
		t.Errorf("the file is not LDIF:\n%s", text[:min(200, len(text))])
	}
	if n := strings.Count(text, "\ndn: "); n != seededPeople {
		t.Errorf("the export holds %d entries, want %d", n, seededPeople)
	}
	if !strings.Contains(text, "cn: Anna Volkova") {
		t.Error("the export is missing a seeded person")
	}
}

func TestFacadeExportsCSV(t *testing.T) {
	a, id := facade(t)
	path := filepath.Join(t.TempDir(), "people.csv")

	if msg := a.ExportSearch(app.ExportInput{
		ProfileID: id,
		Path:      path,
		Filter:    "(objectClass=inetOrgPerson)",
		Scope:     "subtree",
		Base:      rootDN,
		Columns:   []string{"cn", "uid"},
	}); msg != "" {
		t.Fatalf("ExportSearch() = %q", msg)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")

	if lines[0] != "dn,cn,uid,distinguishedName" {
		t.Errorf("header = %q", lines[0])
	}
	if len(lines)-1 != seededPeople {
		t.Errorf("the export holds %d rows, want %d", len(lines)-1, seededPeople)
	}
}

func TestFacadeExportsOneEntry(t *testing.T) {
	a, id := facade(t)
	path := filepath.Join(t.TempDir(), "anna.ldif")

	if msg := a.ExportEntry(id, "cn=Anna Volkova,"+peopleDN, path); msg != "" {
		t.Fatalf("ExportEntry() = %q", msg)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "mail: a.volkova@example.com") {
		t.Errorf("the export is missing the address:\n%s", data)
	}
}

func TestFacadeExportRejectsAnUnknownFormat(t *testing.T) {
	a, id := facade(t)

	msg := a.ExportSearch(app.ExportInput{
		ProfileID: id,
		Path:      filepath.Join(t.TempDir(), "people.txt"),
		Filter:    "(objectClass=*)",
	})
	if msg == "" {
		t.Error("ExportSearch() accepted an extension it cannot write")
	}
}

// The filter editor's check runs against a live connection, so {{me}} resolves
// to whatever the connection is bound as.
func TestFacadeValidateFilterUsesTheBoundIdentity(t *testing.T) {
	a, id := facade(t)

	got := a.ValidateFilter(id, "(manager={{me}})")
	if !got.Valid {
		t.Fatalf("Valid = false: %s", got.Error)
	}
	if !strings.Contains(got.Expanded, adminDN) {
		t.Errorf("Expanded = %q, want it to name the bound identity", got.Expanded)
	}

	// With no connection it must fail rather than quietly produce nothing.
	if a.ValidateFilter("nobody", "(manager={{me}})").Valid {
		t.Error("Valid = true for {{me}} with nothing bound")
	}
}

func TestFacadeSavedFilterSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LDAPPER_CONFIG_DIR", dir)

	first, err := app.New()
	if err != nil {
		t.Fatal(err)
	}
	if msg := first.SaveFilter(app.FilterInput{
		ID: "mine", Name: "Mine", Filter: "(objectClass=inetOrgPerson)", Scope: "subtree",
	}); msg != "" {
		t.Fatalf("SaveFilter() = %q", msg)
	}
	first.Shutdown(nil)

	second, err := app.New()
	if err != nil {
		t.Fatal(err)
	}
	defer second.Shutdown(nil)

	var found bool
	for _, f := range second.ListFilters("") {
		if f.ID == "mine" {
			found = true
		}
	}
	if !found {
		t.Error("the saved filter did not survive a restart")
	}
	_ = time.Now
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
