package filters

import "testing"

func TestBuiltinsLoad(t *testing.T) {
	got := Builtins()
	if len(got) != 18 {
		t.Fatalf("Builtins() returned %d filters, want 18", len(got))
	}
}

func TestBuiltinsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Builtins() {
		if f.ID == "" || f.Name == "" || f.Filter == "" {
			t.Errorf("filter %+v is missing a required field", f)
		}
		if seen[f.ID] {
			t.Errorf("duplicate filter id %q", f.ID)
		}
		seen[f.ID] = true

		switch f.Dialect {
		case DialectAD, DialectGeneric, DialectPOSIX:
		default:
			t.Errorf("filter %q has unknown dialect %q", f.ID, f.Dialect)
		}
		switch f.Scope {
		case ScopeBase, ScopeOne, ScopeSubtree:
		default:
			t.Errorf("filter %q has unknown scope %q", f.ID, f.Scope)
		}
		if !f.BuiltIn {
			t.Errorf("filter %q came from the embedded set but is not marked built-in", f.ID)
		}
	}
}

func TestBuiltinsSplitByDialect(t *testing.T) {
	counts := map[Dialect]int{}
	for _, f := range Builtins() {
		counts[f.Dialect]++
	}
	if counts[DialectAD] != 12 {
		t.Errorf("got %d Active Directory filters, want 12", counts[DialectAD])
	}
	if counts[DialectGeneric]+counts[DialectPOSIX] != 6 {
		t.Errorf("got %d portable filters, want 6", counts[DialectGeneric]+counts[DialectPOSIX])
	}
}

func TestBuiltinsAreACopy(t *testing.T) {
	// Callers must not be able to mutate the shipped set.
	first := Builtins()
	first[0].Name = "clobbered"
	if Builtins()[0].Name == "clobbered" {
		t.Error("Builtins() handed out a reference to the shared slice")
	}
}

// Every built-in should say what it is for. A filter with no description is a
// filter nobody will trust enough to run.
func TestBuiltinsAreDescribed(t *testing.T) {
	for _, f := range Builtins() {
		if f.Description == "" {
			t.Errorf("filter %q has no description", f.ID)
		}
		if len(f.Columns) == 0 {
			t.Errorf("filter %q lists no result columns", f.ID)
		}
	}
}
