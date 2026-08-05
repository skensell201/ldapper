package search

import (
	"errors"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

func TestScopeMapping(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"base", ldap.ScopeBaseObject},
		{"one", ldap.ScopeSingleLevel},
		{"subtree", ldap.ScopeWholeSubtree},
		{"", ldap.ScopeWholeSubtree}, // subtree is the sensible default
		{"nonsense", ldap.ScopeWholeSubtree},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := scopeOf(tt.in); got != tt.want {
				t.Errorf("scopeOf(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestResultFromEntry(t *testing.T) {
	e := &ldap.Entry{
		DN: "CN=Anna Volkova,OU=Users,DC=example,DC=com",
		Attributes: []*ldap.EntryAttribute{
			{Name: "cn", Values: []string{"Anna Volkova"}, ByteValues: [][]byte{[]byte("Anna Volkova")}},
			{Name: "objectClass", Values: []string{"top", "user"}, ByteValues: [][]byte{[]byte("top"), []byte("user")}},
		},
	}

	got := resultFrom(e)
	if got.DN != e.DN {
		t.Errorf("DN = %q, want the entry's DN", got.DN)
	}
	if len(got.Attributes["objectClass"]) != 2 {
		t.Errorf("objectClass has %d values, want 2", len(got.Attributes["objectClass"]))
	}
	if got.Attributes["cn"][0].Raw != "Anna Volkova" {
		t.Errorf("cn = %q, want the value as the server sent it", got.Attributes["cn"][0].Raw)
	}
}

// The search layer hands back decoded values, so a caller never has to know
// which attributes need special treatment.
func TestResultDecodesValues(t *testing.T) {
	e := &ldap.Entry{
		DN: "CN=Anna Volkova,DC=example,DC=com",
		Attributes: []*ldap.EntryAttribute{
			{Name: "userAccountControl", ByteValues: [][]byte{[]byte("66048")}},
		},
	}
	got := resultFrom(e)
	decoded := got.Attributes["userAccountControl"][0].Decoded
	if len(decoded) != 2 || decoded[0] != "NORMAL_ACCOUNT" {
		t.Errorf("Decoded = %v, want the account flags named", decoded)
	}
}

func TestTruncationIsNotAFailure(t *testing.T) {
	tests := []struct {
		name string
		code uint16
	}{
		{"size limit", ldap.LDAPResultSizeLimitExceeded},
		{"time limit", ldap.LDAPResultTimeLimitExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, truncated := truncationOf(&ldap.Error{ResultCode: tt.code})
			if !truncated {
				t.Fatalf("truncationOf() = false, want %s to count as truncation", tt.name)
			}
			if reason == "" {
				t.Error("truncationOf() gave no reason")
			}
		})
	}
}

func TestOtherErrorsAreNotTruncation(t *testing.T) {
	if _, truncated := truncationOf(errors.New("connection reset")); truncated {
		t.Error("truncationOf() treated a network error as truncation")
	}
	if _, truncated := truncationOf(&ldap.Error{ResultCode: ldap.LDAPResultInsufficientAccessRights}); truncated {
		t.Error("truncationOf() treated a permission error as truncation")
	}
	if _, truncated := truncationOf(nil); truncated {
		t.Error("truncationOf(nil) reported truncation")
	}
}

func TestRequestDefaults(t *testing.T) {
	r := Request{Base: "DC=example,DC=com", Filter: "(objectClass=*)"}.withDefaults()
	if r.Scope != "subtree" {
		t.Errorf("Scope = %q, want subtree", r.Scope)
	}
	if r.BatchSize == 0 {
		t.Error("BatchSize = 0, want a non-zero default")
	}
}

func TestRequestKeepsExplicitValues(t *testing.T) {
	r := Request{Scope: "one", BatchSize: 5}.withDefaults()
	if r.Scope != "one" || r.BatchSize != 5 {
		t.Errorf("withDefaults() = %+v, want the caller's values untouched", r)
	}
}
