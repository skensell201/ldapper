// Package search runs filtered searches and hands results back as they arrive.
package search

import (
	"context"
	"errors"

	"github.com/go-ldap/ldap/v3"
	"github.com/skensell201/ldapper/internal/decode"
)

// defaultBatchSize is how many results accumulate before the callback is
// called. Small enough to feel immediate, large enough not to thrash the UI.
const defaultBatchSize = 50

// Request is one search.
type Request struct {
	Base   string
	Filter string
	// Scope is "base", "one" or "subtree". Empty means subtree.
	Scope string
	// Attributes to fetch. Empty means every attribute the server will give us.
	Attributes []string
	// BatchSize is how many results to gather before each callback.
	BatchSize int
}

func (r Request) withDefaults() Request {
	if r.Scope == "" {
		r.Scope = "subtree"
	}
	if r.BatchSize <= 0 {
		r.BatchSize = defaultBatchSize
	}
	return r
}

// Result is one matching entry with its values already decoded.
type Result struct {
	DN         string
	Attributes map[string][]decode.Value
}

// Stats describes how the search ended.
type Stats struct {
	// Matched is how many entries were returned.
	Matched int
	// Truncated is true when the server stopped before the search was
	// complete. The results already delivered are still valid.
	Truncated bool
	// TruncateReason says why, in a sentence fit to show a person.
	TruncateReason string
}

// Stream runs a search, calling onBatch as results arrive. Returning an error
// from onBatch stops the search. Cancelling ctx stops it too.
func Stream(ctx context.Context, conn *ldap.Conn, req Request, onBatch func([]Result) error) (Stats, error) {
	req = req.withDefaults()

	ldapReq := ldap.NewSearchRequest(
		req.Base,
		scopeOf(req.Scope), ldap.NeverDerefAliases, 0, 0, false,
		req.Filter,
		req.Attributes,
		nil,
	)

	var (
		stats Stats
		batch = make([]Result, 0, req.BatchSize)
	)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := onBatch(batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	res := conn.SearchAsync(ctx, ldapReq, req.BatchSize)
	for res.Next() {
		entry := res.Entry()
		if entry == nil {
			continue
		}

		batch = append(batch, resultFrom(entry))
		stats.Matched++

		if len(batch) >= req.BatchSize {
			if err := flush(); err != nil {
				return stats, err
			}
		}
	}

	if err := res.Err(); err != nil {
		// A truncated search still delivered real results. Flush them and
		// report the truncation rather than throwing the work away.
		if reason, truncated := truncationOf(err); truncated {
			stats.Truncated, stats.TruncateReason = true, reason
			if err := flush(); err != nil {
				return stats, err
			}
			return stats, nil
		}
		return stats, err
	}

	if err := flush(); err != nil {
		return stats, err
	}
	return stats, nil
}

func scopeOf(s string) int {
	switch s {
	case "base":
		return ldap.ScopeBaseObject
	case "one":
		return ldap.ScopeSingleLevel
	default:
		return ldap.ScopeWholeSubtree
	}
}

func resultFrom(e *ldap.Entry) Result {
	out := Result{DN: e.DN, Attributes: make(map[string][]decode.Value, len(e.Attributes))}
	for _, attr := range e.Attributes {
		out.Attributes[attr.Name] = decode.Attribute(attr.Name, attr.ByteValues)
	}
	return out
}

// truncationOf reports whether err means "the server stopped early" rather
// than "the search failed".
func truncationOf(err error) (string, bool) {
	var lerr *ldap.Error
	if !errors.As(err, &lerr) {
		return "", false
	}
	switch lerr.ResultCode {
	case ldap.LDAPResultSizeLimitExceeded:
		return "The server reached its size limit — these are the first results, not all of them.", true
	case ldap.LDAPResultTimeLimitExceeded:
		return "The server reached its time limit — these are the results it found before stopping.", true
	default:
		return "", false
	}
}
