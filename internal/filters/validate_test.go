package filters

import (
	"strings"
	"testing"
)

func TestValidateAcceptsGoodFilters(t *testing.T) {
	e := Expander{Now: fixed}
	good := []string{
		"(objectClass=*)",
		"(&(objectClass=user)(sAMAccountName=a.volkova))",
		"(|(objectClass=group)(objectClass=groupOfNames))",
		"(&(objectClass=user)(!(mail=*)))",
		"(&(objectCategory=person)(lastLogonTimestamp<={{now-90d:filetime}}))",
		"(userAccountControl:1.2.840.113556.1.4.803:=2)",
	}
	for _, in := range good {
		t.Run(in, func(t *testing.T) {
			if err := Validate(in, e); err != nil {
				t.Errorf("Validate(%q) = %v, want nil", in, err)
			}
		})
	}
}

func TestValidateRejectsBadFilters(t *testing.T) {
	e := Expander{Now: fixed}
	bad := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"unbalanced", "(objectClass=user"},
		{"no parentheses", "objectClass=user"},
		{"bad substitution", "(whenCreated<={{yesterday}})"},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.in, e); err == nil {
				t.Errorf("Validate(%q) = nil, want an error", tt.in)
			}
		})
	}
}

func TestValidateEveryBuiltin(t *testing.T) {
	// The shipped set must be valid. This test is the reason a typo in
	// builtin.json cannot reach a release.
	e := Expander{Now: fixed, BindDN: "CN=test,DC=example,DC=com"}
	for _, f := range Builtins() {
		if err := Validate(f.Filter, e); err != nil {
			t.Errorf("built-in filter %q is invalid: %v", f.ID, err)
		}
	}
}

func TestValidateErrorNamesTheProblem(t *testing.T) {
	e := Expander{Now: fixed}
	err := Validate("(objectClass=user", e)
	if err == nil {
		t.Fatal("Validate() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "(objectClass=user") {
		t.Errorf("error %q does not quote the offending filter", err)
	}
}
