// Package browse walks the directory tree one level at a time.
package browse

import (
	"strconv"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// defaultPageSize matches Active Directory's own MaxPageSize. Asking for more
// gains nothing; asking for less costs round trips.
const defaultPageSize = 1000

// Entry is one node in the tree.
type Entry struct {
	DN  string
	RDN string
	// Classes are the entry's objectClass values, which decide its icon.
	Classes []string
	// NumSubordinates is how many children the entry has, or -1 when the
	// server does not publish that.
	NumSubordinates int
	// HasChildren is false only when the server said so. An unknown count
	// leaves the node expandable — refusing to expand something that does
	// have children is the worse mistake.
	HasChildren bool
}

// Page is one batch of children plus the cookie needed to ask for the next.
type Page struct {
	Entries []Entry
	// Cookie is empty when the listing is complete.
	Cookie []byte
}

// Children lists the immediate children of dn. Pass a nil cookie for the first
// page, then the cookie from the previous Page for each one after.
func Children(conn *ldap.Conn, dn string, size uint32, cookie []byte) (Page, error) {
	paging := ldap.NewControlPaging(pageSize(size))
	if len(cookie) > 0 {
		paging.SetCookie(cookie)
	}

	req := ldap.NewSearchRequest(
		dn,
		ldap.ScopeSingleLevel, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"objectClass", "numSubordinates"},
		[]ldap.Control{paging},
	)

	res, err := conn.Search(req)
	if err != nil {
		return Page{}, err
	}

	page := Page{Entries: make([]Entry, 0, len(res.Entries))}
	for _, e := range res.Entries {
		page.Entries = append(page.Entries, entryFrom(e))
	}

	if ctrl := ldap.FindControl(res.Controls, ldap.ControlTypePaging); ctrl != nil {
		if p, ok := ctrl.(*ldap.ControlPaging); ok {
			page.Cookie = p.Cookie
		}
	}
	return page, nil
}

func pageSize(n uint32) uint32 {
	if n == 0 {
		return defaultPageSize
	}
	return n
}

func entryFrom(e *ldap.Entry) Entry {
	out := Entry{
		DN:              e.DN,
		RDN:             firstRDN(e.DN),
		Classes:         e.GetAttributeValues("objectClass"),
		NumSubordinates: -1,
		HasChildren:     true,
	}

	if raw := e.GetAttributeValue("numSubordinates"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			out.NumSubordinates = n
			out.HasChildren = n > 0
		}
	}
	return out
}

// firstRDN returns the leftmost component of a DN, respecting the backslash
// escape that lets a comma appear inside a name.
func firstRDN(dn string) string {
	for i := 0; i < len(dn); i++ {
		if dn[i] == '\\' {
			i++ // skip whatever the backslash escapes
			continue
		}
		if dn[i] == ',' {
			return strings.TrimSpace(dn[:i])
		}
	}
	return dn
}
