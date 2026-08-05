// Package app is the surface Ldapper's interface calls. It holds the live
// connections and translates domain types into the shapes TypeScript sees.
// It deliberately contains no directory logic: that lives in internal/, where
// it is tested without a window in the way.
package app

import (
	"sort"
	"strconv"
	"strings"

	"github.com/skensell201/ldapper/internal/browse"
	"github.com/skensell201/ldapper/internal/search"
)

// Node is one row of the directory tree.
type Node struct {
	DN string `json:"dn"`
	// Label is what the row shows: the RDN with its attribute name stripped,
	// so a row reads "Anna Volkova" rather than "CN=Anna Volkova".
	Label string `json:"label"`
	// RDN is the full relative name, shown on hover and used when copying.
	RDN  string `json:"rdn"`
	Icon string `json:"icon"`
	// ChildCount is -1 when the server does not publish one.
	ChildCount  int  `json:"childCount"`
	HasChildren bool `json:"hasChildren"`
}

// NodePage is one batch of children plus the cursor for the next.
type NodePage struct {
	Nodes []Node `json:"nodes"`
	// Cookie is base64 so it survives the trip through JSON. Empty means the
	// listing is complete.
	Cookie string `json:"cookie"`
}

// AttributeValue is one value: what the server stores, and what it means.
type AttributeValue struct {
	Raw     string   `json:"raw"`
	Decoded []string `json:"decoded,omitempty"`
}

// AttributeRow is one attribute of an entry.
type AttributeRow struct {
	Name   string           `json:"name"`
	Values []AttributeValue `json:"values"`
}

// EntryDetail is everything the right-hand pane shows.
type EntryDetail struct {
	DN   string         `json:"dn"`
	RDN  string         `json:"rdn"`
	Icon string         `json:"icon"`
	Rows []AttributeRow `json:"rows"`
}

// ConnectionState is what the utility bar and status bar display.
type ConnectionState struct {
	ProfileID         string   `json:"profileId"`
	Connected         bool     `json:"connected"`
	Host              string   `json:"host"`
	BoundAs           string   `json:"boundAs"`
	RootDN            string   `json:"rootDN"`
	IsActiveDirectory bool     `json:"isActiveDirectory"`
	SupportsPaging    bool     `json:"supportsPaging"`
	Dialects          []string `json:"dialects"`
	Encryption        string   `json:"encryption"`
}

// CertPrompt is what the certificate dialog needs in order to let somebody
// make an informed decision.
type CertPrompt struct {
	Fingerprint string `json:"fingerprint"`
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	NotAfter    string `json:"notAfter"`
}

func nodeFrom(e browse.Entry) Node {
	return Node{
		DN:          e.DN,
		Label:       labelOf(e.RDN),
		RDN:         e.RDN,
		Icon:        iconFor(e.Classes),
		ChildCount:  e.NumSubordinates,
		HasChildren: e.HasChildren,
	}
}

// labelOf strips the attribute name from an RDN and undoes RFC 4514 escaping:
// CN=Volkova\2C Anna becomes Volkova, Anna. An RDN with no equals sign is
// left alone.
//
// The unescaping matters more than it looks. A directory returns a comma
// inside a name as either \, or \2C depending on the server, and showing
// either one to a person is showing them the wire format instead of the name.
func labelOf(rdn string) string {
	if i := strings.IndexByte(rdn, '='); i >= 0 {
		rdn = rdn[i+1:]
	}
	return unescapeDN(rdn)
}

// unescapeDN resolves the two escape forms RFC 4514 allows: a backslash
// before a special character, and a backslash before two hex digits.
func unescapeDN(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		// \XX, a hex pair.
		if i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		// \c, the character itself.
		b.WriteByte(s[i+1])
		i++
	}
	return b.String()
}

// rowsFrom turns a search result into the attribute table, sorted by name.
// Sorting is what keeps the table stable: Go's map order would otherwise
// reshuffle every row on each selection.
func rowsFrom(r search.Result) []AttributeRow {
	rows := make([]AttributeRow, 0, len(r.Attributes))
	for name, values := range r.Attributes {
		row := AttributeRow{Name: name, Values: make([]AttributeValue, 0, len(values))}
		for _, v := range values {
			row.Values = append(row.Values, AttributeValue{Raw: v.Raw, Decoded: v.Decoded})
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}
