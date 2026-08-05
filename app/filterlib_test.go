package app

import (
	"strings"
	"testing"
)

func TestValidateFilterShowsWhatItExpandsTo(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	const in = "(whenCreated<={{now-90d:generalized}})"
	got := a.ValidateFilter("nobody", in)
	if !got.Valid {
		t.Fatalf("Valid = false: %s", got.Error)
	}
	if got.Expanded == in {
		t.Errorf("Expanded = %q, want the substitution resolved", got.Expanded)
	}
	if !strings.HasSuffix(strings.TrimSuffix(got.Expanded, ")"), "Z") {
		t.Errorf("Expanded = %q, want a GeneralizedTime", got.Expanded)
	}
}

func TestValidateFilterReportsWhatIsWrong(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	got := a.ValidateFilter("nobody", "(objectClass=user")
	if got.Valid {
		t.Error("Valid = true for an unbalanced filter")
	}
	if got.Error == "" {
		t.Error("Error is empty")
	}
}

func TestSaveFilterRejectsAnInvalidOne(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if msg := a.SaveFilter(FilterInput{ID: "mine", Name: "Mine", Filter: "(objectClass=user"}); msg == "" {
		t.Error("SaveFilter() stored a filter that cannot be compiled")
	}
}

// A filter using {{me}} only compiles while connected. Refusing it at save
// time is better than letting it fail the first time somebody reaches for it.
func TestSaveFilterRejectsMe(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if msg := a.SaveFilter(FilterInput{ID: "mine", Name: "Mine", Filter: "(manager={{me}})"}); msg == "" {
		t.Error("SaveFilter() stored a filter that only compiles while bound")
	}
}

func TestEditingABuiltinAndResettingIt(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if msg := a.SaveFilter(FilterInput{
		ID: "ad-disabled-accounts", Name: "Switched off",
		Filter: "(objectClass=user)", Scope: "subtree", Dialect: "ad",
	}); msg != "" {
		t.Fatalf("SaveFilter() = %q", msg)
	}

	var found bool
	for _, f := range a.ListFilters("nobody") {
		if f.ID != "ad-disabled-accounts" {
			continue
		}
		found = true
		if !f.Modified {
			t.Error("the edited built-in is not marked modified")
		}
		if f.Name != "Switched off" {
			t.Errorf("Name = %q, want the edit", f.Name)
		}
	}
	if !found {
		t.Fatal("the edited built-in disappeared from the library")
	}

	if msg := a.ResetFilter("ad-disabled-accounts"); msg != "" {
		t.Fatalf("ResetFilter() = %q", msg)
	}
	for _, f := range a.ListFilters("nobody") {
		if f.ID == "ad-disabled-accounts" && f.Modified {
			t.Error("the filter is still modified after Reset")
		}
	}
}

func TestDeleteAndRestore(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if msg := a.SaveFilter(FilterInput{ID: "mine", Name: "Mine", Filter: "(objectClass=*)"}); msg != "" {
		t.Fatalf("SaveFilter() = %q", msg)
	}
	if msg := a.DeleteFilter("posix-accounts"); msg != "" {
		t.Fatalf("DeleteFilter() = %q", msg)
	}

	if has(a.ListFilters("nobody"), "posix-accounts") {
		t.Error("the deleted built-in is still listed")
	}

	if msg := a.RestoreDefaultFilters(); msg != "" {
		t.Fatalf("RestoreDefaultFilters() = %q", msg)
	}

	if !has(a.ListFilters("nobody"), "mine") {
		t.Error("RestoreDefaultFilters() removed a custom filter, which it must never do")
	}
	if !has(a.ListFilters("nobody"), "posix-accounts") {
		t.Error("RestoreDefaultFilters() did not bring back the deleted built-in")
	}
}

func has(list []FilterSummary, id string) bool {
	for _, f := range list {
		if f.ID == id {
			return true
		}
	}
	return false
}
