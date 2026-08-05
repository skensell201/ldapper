package filters

import (
	"testing"
	"time"
)

// fixed is the instant every test in this file pretends it is:
// 2026-08-05 12:00:00 UTC, which is 1785931200 in Unix seconds.
var fixed = time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

func TestExpandFileTime(t *testing.T) {
	e := Expander{Now: fixed}

	// FILETIME = (1785931200 + 11644473600) * 10000000 = 134304048000000000.
	got, err := e.Expand("(whenCreated<={{now:filetime}})")
	if err != nil {
		t.Fatalf("Expand() returned error: %v", err)
	}
	if got != "(whenCreated<=134304048000000000)" {
		t.Errorf("Expand() = %q, want the FILETIME substituted", got)
	}
}

func TestExpandRelativeOffsets(t *testing.T) {
	e := Expander{Now: fixed}
	tests := []struct {
		in   string
		want string
	}{
		// 90 days earlier is 2026-05-07 12:00:00 UTC.
		{"{{now-90d:generalized}}", "20260507120000Z"},
		{"{{now-6h:generalized}}", "20260805060000Z"},
		{"{{now-30m:generalized}}", "20260805113000Z"},
		{"{{now+1d:generalized}}", "20260806120000Z"},
		// No format given defaults to generalized.
		{"{{now}}", "20260805120000Z"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := e.Expand(tt.in)
			if err != nil {
				t.Fatalf("Expand(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Expand(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExpandBindDN(t *testing.T) {
	e := Expander{Now: fixed, BindDN: "CN=Anna Volkova,OU=Users,DC=corp,DC=example,DC=com"}
	got, err := e.Expand("(manager={{me}})")
	if err != nil {
		t.Fatalf("Expand() returned error: %v", err)
	}
	want := "(manager=CN=Anna Volkova,OU=Users,DC=corp,DC=example,DC=com)"
	if got != want {
		t.Errorf("Expand() = %q, want %q", got, want)
	}
}

func TestExpandLeavesPlainFiltersAlone(t *testing.T) {
	e := Expander{Now: fixed}
	in := "(&(objectClass=user)(sAMAccountName=a.volkova))"
	got, err := e.Expand(in)
	if err != nil {
		t.Fatalf("Expand() returned error: %v", err)
	}
	if got != in {
		t.Errorf("Expand() = %q, want the filter unchanged", got)
	}
}

func TestExpandHandlesSeveralSubstitutions(t *testing.T) {
	e := Expander{Now: fixed}
	got, err := e.Expand("(&(whenCreated>={{now-1d:generalized}})(whenChanged<={{now:generalized}}))")
	if err != nil {
		t.Fatalf("Expand() returned error: %v", err)
	}
	want := "(&(whenCreated>=20260804120000Z)(whenChanged<=20260805120000Z))"
	if got != want {
		t.Errorf("Expand() = %q, want %q", got, want)
	}
}

func TestExpandRejectsBadSubstitutions(t *testing.T) {
	e := Expander{Now: fixed}
	tests := []string{
		"{{tomorrow}}",         // unknown variable
		"{{now-90y:filetime}}", // unknown unit
		"{{now:epoch}}",        // unknown format
		"{{me:filetime}}",      // me takes no format
		"{{}}",                 // empty
	}

	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			if _, err := e.Expand(in); err == nil {
				t.Errorf("Expand(%q) succeeded, want an error", in)
			}
		})
	}
}

func TestExpandRejectsMeWithoutABind(t *testing.T) {
	e := Expander{Now: fixed}
	if _, err := e.Expand("(manager={{me}})"); err == nil {
		t.Error("Expand() with no bind DN succeeded, want an error")
	}
}
