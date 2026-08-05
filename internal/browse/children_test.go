package browse

import (
	"testing"

	"github.com/go-ldap/ldap/v3"
)

func TestEntryFromLDAP(t *testing.T) {
	e := &ldap.Entry{
		DN: "CN=Anna Volkova,OU=Users,DC=corp,DC=example,DC=com",
		Attributes: []*ldap.EntryAttribute{
			{Name: "objectClass", Values: []string{"top", "person", "user"}},
			{Name: "numSubordinates", Values: []string{"0"}},
		},
	}

	got := entryFrom(e)
	if got.DN != e.DN {
		t.Errorf("DN = %q, want the full distinguished name", got.DN)
	}
	if got.RDN != "CN=Anna Volkova" {
		t.Errorf("RDN = %q, want CN=Anna Volkova", got.RDN)
	}
	if got.NumSubordinates != 0 {
		t.Errorf("NumSubordinates = %d, want 0", got.NumSubordinates)
	}
	if got.HasChildren {
		t.Error("HasChildren = true for an entry that reported no subordinates")
	}
	if len(got.Classes) != 3 {
		t.Errorf("Classes = %v, want all three", got.Classes)
	}
}

func TestEntryWithChildren(t *testing.T) {
	e := &ldap.Entry{
		DN: "OU=Users,DC=example,DC=com",
		Attributes: []*ldap.EntryAttribute{
			{Name: "objectClass", Values: []string{"organizationalUnit"}},
			{Name: "numSubordinates", Values: []string{"3414"}},
		},
	}
	got := entryFrom(e)
	if got.NumSubordinates != 3414 {
		t.Errorf("NumSubordinates = %d, want 3414", got.NumSubordinates)
	}
	if !got.HasChildren {
		t.Error("HasChildren = false for an entry with 3414 subordinates")
	}
}

func TestEntryWithUnknownChildCount(t *testing.T) {
	// OpenLDAP does not publish numSubordinates. An unknown count must not be
	// mistaken for zero, or the tree would refuse to expand anything.
	e := &ldap.Entry{
		DN:         "ou=people,dc=example,dc=com",
		Attributes: []*ldap.EntryAttribute{{Name: "objectClass", Values: []string{"organizationalUnit"}}},
	}

	got := entryFrom(e)
	if got.NumSubordinates != -1 {
		t.Errorf("NumSubordinates = %d, want -1 for unknown", got.NumSubordinates)
	}
	if !got.HasChildren {
		t.Error("HasChildren = false for an entry whose child count is unknown; it must stay expandable")
	}
}

func TestEntryWithUnparsableChildCount(t *testing.T) {
	e := &ldap.Entry{
		DN:         "ou=people,dc=example,dc=com",
		Attributes: []*ldap.EntryAttribute{{Name: "numSubordinates", Values: []string{"lots"}}},
	}
	if got := entryFrom(e); got.NumSubordinates != -1 || !got.HasChildren {
		t.Errorf("entryFrom() = %+v, want the unreadable count treated as unknown", got)
	}
}

func TestRDN(t *testing.T) {
	tests := []struct {
		name string
		dn   string
		want string
	}{
		{"a leaf", "CN=Anna Volkova,OU=Users,DC=example,DC=com", "CN=Anna Volkova"},
		{"a root", "DC=corp,DC=example,DC=com", "DC=corp"},
		{"an escaped comma", `CN=Volkova\, Anna,OU=Users,DC=example,DC=com`, `CN=Volkova\, Anna`},
		{"a single component", "dc=example", "dc=example"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstRDN(tt.dn); got != tt.want {
				t.Errorf("firstRDN(%q) = %q, want %q", tt.dn, got, tt.want)
			}
		})
	}
}

func TestPageSizeFloor(t *testing.T) {
	if got := pageSize(0); got != defaultPageSize {
		t.Errorf("pageSize(0) = %d, want the default %d", got, defaultPageSize)
	}
	if got := pageSize(250); got != 250 {
		t.Errorf("pageSize(250) = %d, want it left alone", got)
	}
}
