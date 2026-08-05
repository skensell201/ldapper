package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/skensell201/ldapper/internal/browse"
	"github.com/skensell201/ldapper/internal/decode"
	"github.com/skensell201/ldapper/internal/search"
)

func TestNodeFromBrowseEntry(t *testing.T) {
	got := nodeFrom(browse.Entry{
		DN:              "CN=Anna Volkova,OU=Users,DC=example,DC=com",
		RDN:             "CN=Anna Volkova",
		Classes:         []string{"top", "person", "user"},
		NumSubordinates: 0,
		HasChildren:     false,
	})

	if got.DN != "CN=Anna Volkova,OU=Users,DC=example,DC=com" {
		t.Errorf("DN = %q", got.DN)
	}
	if got.Label != "Anna Volkova" {
		t.Errorf("Label = %q, want the RDN's value without its attribute name", got.Label)
	}
	if got.Icon != "user" {
		t.Errorf("Icon = %q, want user", got.Icon)
	}
	if got.HasChildren {
		t.Error("HasChildren = true for a leaf")
	}
}

func TestNodeLabelKeepsAnUnusualRDN(t *testing.T) {
	got := nodeFrom(browse.Entry{RDN: "no-equals-sign"})
	if got.Label != "no-equals-sign" {
		t.Errorf("Label = %q, want the RDN unchanged when it has no attribute name", got.Label)
	}
}

// An unknown child count reaches the interface as -1, not 0: the tree draws a
// different affordance for "no children" than for "we do not know yet".
func TestNodeCarriesAnUnknownChildCount(t *testing.T) {
	got := nodeFrom(browse.Entry{NumSubordinates: -1, HasChildren: true})
	if got.ChildCount != -1 {
		t.Errorf("ChildCount = %d, want -1", got.ChildCount)
	}
}

func TestAttributeRowsAreSorted(t *testing.T) {
	rows := rowsFrom(search.Result{
		DN: "CN=x,DC=example,DC=com",
		Attributes: map[string][]decode.Value{
			"zebra":       {{Raw: "z"}},
			"alpha":       {{Raw: "a"}},
			"objectClass": {{Raw: "top"}, {Raw: "user"}},
		},
	})

	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	// Sorting is what makes the table stable between selections; Go's map
	// order would otherwise reshuffle it on every click.
	if rows[0].Name != "alpha" || rows[2].Name != "zebra" {
		t.Errorf("rows are not sorted: %v", []string{rows[0].Name, rows[1].Name, rows[2].Name})
	}
	if rows[1].Name != "objectClass" || len(rows[1].Values) != 2 {
		t.Errorf("objectClass row = %+v, want both values", rows[1])
	}
}

func TestAttributeValueCarriesBothForms(t *testing.T) {
	rows := rowsFrom(search.Result{
		Attributes: map[string][]decode.Value{
			"userAccountControl": {{Raw: "66048", Decoded: []string{"NORMAL_ACCOUNT", "DONT_EXPIRE_PASSWORD"}}},
		},
	})
	v := rows[0].Values[0]
	if v.Raw != "66048" {
		t.Errorf("Raw = %q", v.Raw)
	}
	if len(v.Decoded) != 2 {
		t.Errorf("Decoded = %v, want both flags", v.Decoded)
	}
}

func TestRowsFromAnEmptyEntry(t *testing.T) {
	if got := rowsFrom(search.Result{}); len(got) != 0 {
		t.Errorf("rowsFrom() = %v, want no rows", got)
	}
}

// Every DTO has to survive a round trip through JSON, because that is how it
// reaches the interface.
func TestDTOsMarshal(t *testing.T) {
	for _, v := range []any{
		Node{}, AttributeRow{}, AttributeValue{}, EntryDetail{},
		NodePage{}, ConnectionState{}, CertPrompt{},
	} {
		if _, err := json.Marshal(v); err != nil {
			t.Errorf("%T does not marshal: %v", v, err)
		}
	}
}

// Field names reach TypeScript verbatim, so they have to be the lowerCamelCase
// a TypeScript author expects rather than Go's exported capitals.
func TestJSONNamesAreLowerCamelCase(t *testing.T) {
	data, err := json.Marshal(Node{DN: "dc=example"})
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	for _, want := range []string{`"dn"`, `"label"`, `"childCount"`, `"hasChildren"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("Node marshals to %s, want a %s field", data, want)
		}
	}
}

// A directory returns a comma inside a name as \, or \2C depending on the
// server. Either one shown to a person is the wire format, not the name.
func TestNodeLabelUnescapes(t *testing.T) {
	tests := []struct {
		name string
		rdn  string
		want string
	}{
		{"hex escape", `CN=Volkova\2C Anna`, "Volkova, Anna"},
		{"character escape", `CN=Volkova\, Anna`, "Volkova, Anna"},
		{"escaped backslash", `CN=back\\slash`, `back\slash`},
		{"escaped plus", `CN=one\+two`, "one+two"},
		{"lower-case hex", `CN=a\2cb`, "a,b"},
		{"nothing to undo", "CN=Anna Volkova", "Anna Volkova"},
		{"a trailing backslash is left alone", `CN=odd\`, `odd\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeFrom(browse.Entry{RDN: tt.rdn}).Label; got != tt.want {
				t.Errorf("Label = %q, want %q", got, tt.want)
			}
		})
	}
}

// The DN itself is never unescaped: it is an identifier, and it goes back to
// the server exactly as it arrived.
func TestNodeKeepsTheDNEscaped(t *testing.T) {
	const dn = `CN=Volkova\2C Anna,OU=Users,DC=example,DC=com`
	got := nodeFrom(browse.Entry{DN: dn, RDN: `CN=Volkova\2C Anna`})

	if got.DN != dn {
		t.Errorf("DN = %q, want it untouched", got.DN)
	}
	if got.RDN != `CN=Volkova\2C Anna` {
		t.Errorf("RDN = %q, want the escaped form kept for copying", got.RDN)
	}
}
