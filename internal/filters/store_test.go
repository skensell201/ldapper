package filters

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestStore builds a store backed by a throwaway file.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "filters.json")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	return s
}

func TestStoreStartsWithTheBuiltins(t *testing.T) {
	s := newTestStore(t)
	if len(s.All()) != 18 {
		t.Errorf("All() returned %d filters, want the 18 built-ins", len(s.All()))
	}
}

func TestStoreEditingABuiltinMarksItModified(t *testing.T) {
	s := newTestStore(t)

	f, ok := s.Get("ad-stale-users-90d")
	if !ok {
		t.Fatal("Get() could not find ad-stale-users-90d")
	}
	f.Filter = "(&(objectCategory=person)(lastLogonTimestamp<={{now-30d:filetime}}))"
	if err := s.Save(f); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	got, _ := s.Get("ad-stale-users-90d")
	if !got.Modified {
		t.Error("the edited built-in is not marked Modified")
	}
	if !got.BuiltIn {
		t.Error("the edited filter lost its BuiltIn flag")
	}
	if got.Filter != f.Filter {
		t.Errorf("Filter = %q, want the edit to have stuck", got.Filter)
	}
	if len(s.All()) != 18 {
		t.Errorf("All() returned %d filters, want 18 — an edit must not add a filter", len(s.All()))
	}
}

func TestStoreResetUndoesAnEdit(t *testing.T) {
	s := newTestStore(t)
	original, _ := s.Get("ad-disabled-accounts")

	edited := original
	edited.Name = "Switched-off accounts"
	if err := s.Save(edited); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if err := s.Reset("ad-disabled-accounts"); err != nil {
		t.Fatalf("Reset() returned error: %v", err)
	}

	got, _ := s.Get("ad-disabled-accounts")
	if got.Name != original.Name {
		t.Errorf("Name = %q, want the shipped name %q back", got.Name, original.Name)
	}
	if got.Modified {
		t.Error("the filter is still marked Modified after Reset")
	}
}

func TestStoreCustomFilters(t *testing.T) {
	s := newTestStore(t)

	f := Filter{
		ID:      "my-vpn-users",
		Name:    "VPN group members",
		Filter:  "(memberOf=CN=VPN Users,DC=example,DC=com)",
		Scope:   ScopeSubtree,
		Dialect: DialectGeneric,
	}
	if err := s.Save(f); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	got, ok := s.Get("my-vpn-users")
	if !ok {
		t.Fatal("Get() could not find the saved custom filter")
	}
	if got.BuiltIn {
		t.Error("a custom filter is marked BuiltIn")
	}
	if len(s.All()) != 19 {
		t.Errorf("All() returned %d filters, want 19", len(s.All()))
	}
}

func TestStoreDeleteRemovesBothKinds(t *testing.T) {
	s := newTestStore(t)

	custom := Filter{ID: "mine", Name: "Mine", Filter: "(objectClass=*)", Scope: ScopeSubtree, Dialect: DialectGeneric}
	if err := s.Save(custom); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if err := s.Delete("mine"); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}
	if _, ok := s.Get("mine"); ok {
		t.Error("the deleted custom filter is still present")
	}

	if err := s.Delete("posix-accounts"); err != nil {
		t.Fatalf("Delete() on a built-in returned error: %v", err)
	}
	if _, ok := s.Get("posix-accounts"); ok {
		t.Error("the deleted built-in is still present")
	}
	if len(s.All()) != 17 {
		t.Errorf("All() returned %d filters, want 17", len(s.All()))
	}
}

func TestStoreDeletedBuiltinStaysDeletedAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	if err := s.Delete("posix-accounts"); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() on reopen returned error: %v", err)
	}
	if _, ok := reopened.Get("posix-accounts"); ok {
		t.Error("the deleted built-in came back after reopening the store")
	}
}

func TestStoreRestoreDefaultsKeepsCustomFilters(t *testing.T) {
	s := newTestStore(t)

	edited, _ := s.Get("ad-disabled-accounts")
	edited.Name = "Edited"
	if err := s.Save(edited); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if err := s.Delete("posix-accounts"); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}
	custom := Filter{ID: "mine", Name: "Mine", Filter: "(objectClass=*)", Scope: ScopeSubtree, Dialect: DialectGeneric}
	if err := s.Save(custom); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if err := s.RestoreDefaults(); err != nil {
		t.Fatalf("RestoreDefaults() returned error: %v", err)
	}

	if got, _ := s.Get("ad-disabled-accounts"); got.Name == "Edited" {
		t.Error("RestoreDefaults() left the edit in place")
	}
	if _, ok := s.Get("posix-accounts"); !ok {
		t.Error("RestoreDefaults() did not bring back the deleted built-in")
	}
	if _, ok := s.Get("mine"); !ok {
		t.Error("RestoreDefaults() removed a custom filter, which it must never do")
	}
}

func TestStorePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	custom := Filter{ID: "mine", Name: "Mine", Filter: "(objectClass=*)", Scope: ScopeSubtree, Dialect: DialectGeneric}
	if err := s.Save(custom); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() on reopen returned error: %v", err)
	}
	if _, ok := reopened.Get("mine"); !ok {
		t.Error("the custom filter did not survive a reopen")
	}
}

// Saving twice under one ID replaces the filter rather than duplicating it.
func TestStoreSaveIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	f := Filter{ID: "mine", Name: "First", Filter: "(objectClass=*)", Scope: ScopeSubtree, Dialect: DialectGeneric}

	if err := s.Save(f); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	f.Name = "Second"
	if err := s.Save(f); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if len(s.All()) != 19 {
		t.Errorf("All() returned %d filters, want 19 — the second save duplicated it", len(s.All()))
	}
	if got, _ := s.Get("mine"); got.Name != "Second" {
		t.Errorf("Name = %q, want the later save to win", got.Name)
	}
}

func TestStoreRejectsAFilterWithNoID(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(Filter{Name: "No id", Filter: "(objectClass=*)"}); err == nil {
		t.Error("Save() accepted a filter with no ID, want an error")
	}
}

func TestStoreFillsInDefaults(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(Filter{ID: "mine", Name: "Mine", Filter: "(objectClass=*)"}); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	got, _ := s.Get("mine")
	if got.Scope != ScopeSubtree {
		t.Errorf("Scope = %q, want subtree by default", got.Scope)
	}
	if got.Dialect != DialectGeneric {
		t.Errorf("Dialect = %q, want generic by default", got.Dialect)
	}
}

func TestNewStoreCreatesNothingUntilSomethingIsSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")
	if _, err := NewStore(path); err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("NewStore() wrote a file before the user changed anything")
	}
}

func TestNewStoreRejectsAMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("cannot write the fixture: %v", err)
	}
	if _, err := NewStore(path); err == nil {
		t.Error("NewStore() accepted a malformed file, want an error naming the path")
	}
}
