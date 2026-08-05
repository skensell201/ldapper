package app

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/skensell201/ldapper/internal/browse"
	"github.com/skensell201/ldapper/internal/ldaperr"
	"github.com/skensell201/ldapper/internal/search"
)

// ChildrenResult is one page of a node's children.
type ChildrenResult struct {
	NodePage
	Error string `json:"error,omitempty"`
}

// EntryResult is one entry's attributes.
type EntryResult struct {
	Detail EntryDetail `json:"detail"`
	Error  string      `json:"error,omitempty"`
}

// Children lists one page of a node's children. Pass an empty cookie for the
// first page and the cookie from the previous result for each one after.
func (a *App) Children(profileID, dn string, pageSize int, cookie string) ChildrenResult {
	c, ok := a.live(profileID)
	if !ok {
		return ChildrenResult{Error: "That connection is not open. Connect first."}
	}

	previous, err := decodeCookie(cookie)
	if err != nil {
		return ChildrenResult{Error: "The paging cursor was not readable. Collapse the node and open it again."}
	}

	if pageSize < 0 {
		pageSize = 0
	}
	page, err := browse.Children(c.conn.Conn, dn, uint32(pageSize), previous)
	if err != nil {
		return ChildrenResult{Error: ldaperr.Explain(err)}
	}

	nodes := make([]Node, 0, len(page.Entries))
	for _, e := range page.Entries {
		nodes = append(nodes, nodeFrom(e))
	}

	return ChildrenResult{NodePage: NodePage{
		Nodes:  nodes,
		Cookie: encodeCookie(page.Cookie),
	}}
}

// Entry reads every attribute of one object.
func (a *App) Entry(profileID, dn string) EntryResult {
	c, ok := a.live(profileID)
	if !ok {
		return EntryResult{Error: "That connection is not open. Connect first."}
	}

	var found *search.Result
	_, err := search.Stream(context.Background(), c.conn.Conn, search.Request{
		Base:   dn,
		Filter: "(objectClass=*)",
		Scope:  "base",
	}, func(batch []search.Result) error {
		if len(batch) > 0 && found == nil {
			r := batch[0]
			found = &r
		}
		return nil
	})
	if err != nil {
		return EntryResult{Error: ldaperr.Explain(err)}
	}
	if found == nil {
		return EntryResult{Error: fmt.Sprintf("Nothing was returned for %s. It may have been moved or deleted.", dn)}
	}

	var classes []string
	for _, v := range found.Attributes["objectClass"] {
		classes = append(classes, v.Raw)
	}

	return EntryResult{Detail: EntryDetail{
		DN:   found.DN,
		RDN:  firstRDN(found.DN),
		Icon: iconFor(classes),
		Rows: rowsFrom(*found),
	}}
}

// firstRDN mirrors what the browse package does, so a detail pane opened
// directly by DN labels itself the same way the tree would.
func firstRDN(dn string) string {
	for i := 0; i < len(dn); i++ {
		if dn[i] == '\\' {
			i++
			continue
		}
		if dn[i] == ',' {
			return dn[:i]
		}
	}
	return dn
}

// encodeCookie makes a paging cursor safe to carry through JSON. Servers put
// arbitrary bytes in these, including sequences that are not valid UTF-8.
func encodeCookie(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

func decodeCookie(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(s)
}
