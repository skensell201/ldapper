package app

import (
	"testing"

	"github.com/skensell201/ldapper/internal/decode"
	"github.com/skensell201/ldapper/internal/search"
)

func TestSearchRowFromResult(t *testing.T) {
	got := searchRow(search.Result{
		DN: "CN=Anna Volkova,OU=Users,DC=example,DC=com",
		Attributes: map[string][]decode.Value{
			"cn":          {{Raw: "Anna Volkova"}},
			"objectClass": {{Raw: "top"}, {Raw: "user"}},
		},
	}, []string{"cn", "objectClass", "mail"})

	if got.DN != "CN=Anna Volkova,OU=Users,DC=example,DC=com" {
		t.Errorf("DN = %q", got.DN)
	}
	if got.Icon != "user" {
		t.Errorf("Icon = %q, want user — the row draws the same glyph as the tree", got.Icon)
	}
	if len(got.Cells) != 3 {
		t.Fatalf("got %d cells, want one per requested column", len(got.Cells))
	}
	if got.Cells[0] != "Anna Volkova" {
		t.Errorf("cell 0 = %q", got.Cells[0])
	}
	// Multi-valued attributes join, so a column stays one line.
	if got.Cells[1] != "top; user" {
		t.Errorf("cell 1 = %q, want the values joined", got.Cells[1])
	}
	// A column the entry does not carry is empty, not missing: the table has
	// fixed columns and a short row would shift everything after it.
	if got.Cells[2] != "" {
		t.Errorf("cell 2 = %q, want empty for an absent attribute", got.Cells[2])
	}
}

func TestSearchRowFillsTheDNColumn(t *testing.T) {
	got := searchRow(search.Result{DN: "cn=x,dc=example,dc=com"}, []string{"distinguishedName"})
	if got.Cells[0] != "cn=x,dc=example,dc=com" {
		t.Errorf("cell = %q, want the DN", got.Cells[0])
	}
}

func TestSearchColumnsAlwaysIncludeTheDN(t *testing.T) {
	got := columnsFor(nil)
	if len(got) == 0 || got[len(got)-1] != "distinguishedName" {
		t.Errorf("columnsFor(nil) = %v, want the DN as the last column", got)
	}

	got = columnsFor([]string{"cn", "distinguishedName"})
	var n int
	for _, c := range got {
		if c == "distinguishedName" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("columnsFor() = %v, want distinguishedName exactly once", got)
	}
	if got[len(got)-1] != "distinguishedName" {
		t.Errorf("columnsFor() = %v, want the DN last", got)
	}
}

func TestSearchColumnsDropEmptyNames(t *testing.T) {
	got := columnsFor([]string{"cn", "", "mail"})
	for _, c := range got {
		if c == "" {
			t.Errorf("columnsFor() = %v, want no empty column names", got)
		}
	}
}

func TestStartSearchWithoutAConnection(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	got := a.StartSearch(SearchInput{ProfileID: "nobody", Filter: "(objectClass=*)"})
	if got.Error == "" {
		t.Error("Error is empty; searching an unopened connection must say so")
	}
	if got.Started {
		t.Error("Started = true with no connection")
	}
}

func TestStartSearchRejectsABadFilter(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()
	a.setLive("dc01", &liveConn{})

	got := a.StartSearch(SearchInput{ProfileID: "dc01", Filter: "(objectClass=user"})
	if got.Error == "" {
		t.Error("Error is empty for an unbalanced filter; it must never reach the server")
	}
	if got.Started {
		t.Error("Started = true for a filter that will not compile")
	}
}

func TestStartSearchRejectsAnUnknownSubstitution(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()
	a.setLive("dc01", &liveConn{})

	got := a.StartSearch(SearchInput{ProfileID: "dc01", Filter: "(whenCreated<={{yesterday}})"})
	if got.Error == "" {
		t.Error("Error is empty; an unknown substitution would return nothing and look like no matches")
	}
}

func TestStopSearchIsSafeWhenNothingIsRunning(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()
	a.StopSearch("nobody") // must not panic
}

// The registry must not leak: a connection that comes and goes repeatedly
// would otherwise accumulate cancel functions forever.
func TestSearchCancelRegistryIsCleared(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	a.setSearchCancel("dc01", func() {})
	a.stopSearch("dc01")

	a.mu.RLock()
	n := len(a.searches)
	a.mu.RUnlock()

	if n != 0 {
		t.Errorf("the registry holds %d entries after stopping the only search", n)
	}
}

// emit is called from a goroutine with no window in tests. It must not panic.
func TestEmitWithoutAWindow(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()
	a.emit(EventSearchBatch, SearchBatch{})
}
