package app

import (
	"context"
	"strings"
	"time"

	"github.com/skensell201/ldapper/internal/filters"
	"github.com/skensell201/ldapper/internal/ldaperr"
	"github.com/skensell201/ldapper/internal/search"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Event names the interface subscribes to.
const (
	EventSearchBatch = "search:batch"
	EventSearchDone  = "search:done"
)

// SearchInput is what the toolbar sends.
type SearchInput struct {
	ProfileID string   `json:"profileId"`
	Filter    string   `json:"filter"`
	Scope     string   `json:"scope"`
	Base      string   `json:"base"`
	Columns   []string `json:"columns"`
}

// SearchRow is one line of the result table.
type SearchRow struct {
	DN    string   `json:"dn"`
	Icon  string   `json:"icon"`
	Cells []string `json:"cells"`
}

// SearchBatch is one event's worth of results.
type SearchBatch struct {
	Rows []SearchRow `json:"rows"`
	// Matched is the running total, so the counter does not have to be
	// derived from a growing array on the other side.
	Matched int `json:"matched"`
}

// SearchDone ends a search, however it ended.
type SearchDone struct {
	Matched   int    `json:"matched"`
	Truncated bool   `json:"truncated"`
	Reason    string `json:"reason,omitempty"`
	Error     string `json:"error,omitempty"`
	Cancelled bool   `json:"cancelled"`
	// Columns is what the table should show, echoed back because the engine
	// may have added the distinguished name.
	Columns []string `json:"columns"`
	// Elapsed is in milliseconds.
	Elapsed int64 `json:"elapsed"`
}

// StartResult says whether the search began. Everything after that arrives as
// events.
type StartResult struct {
	Started bool     `json:"started"`
	Error   string   `json:"error,omitempty"`
	Columns []string `json:"columns"`
	// Expanded is the filter as it will reach the server, so the interface can
	// show what {{now-90d:filetime}} turned into.
	Expanded string `json:"expanded,omitempty"`
}

// StartSearch runs a search in the background, emitting batches as they
// arrive. Starting one cancels whatever was already running on that
// connection.
func (a *App) StartSearch(in SearchInput) StartResult {
	c, ok := a.live(in.ProfileID)
	if !ok {
		return StartResult{Error: "That connection is not open. Connect first."}
	}

	bindDN := ""
	if c.conn != nil {
		bindDN = c.conn.BindDN
	}
	expander := filters.Expander{Now: time.Now(), BindDN: bindDN}

	if err := filters.Validate(in.Filter, expander); err != nil {
		return StartResult{Error: err.Error()}
	}
	expanded, err := expander.Expand(in.Filter)
	if err != nil {
		return StartResult{Error: err.Error()}
	}

	base := in.Base
	if base == "" {
		base = c.info.RootDN()
	}
	columns := columnsFor(in.Columns)

	// One search at a time per connection.
	a.stopSearch(in.ProfileID)

	ctx, cancel := context.WithCancel(context.Background())
	a.setSearchCancel(in.ProfileID, cancel)

	go a.runSearch(ctx, c, in.ProfileID, expanded, in.Scope, base, columns)

	return StartResult{Started: true, Columns: columns, Expanded: expanded}
}

func (a *App) runSearch(ctx context.Context, c *liveConn, profileID, filter, scope, base string, columns []string) {
	defer a.clearSearchCancel(profileID)

	started := time.Now()
	var matched int

	stats, err := search.Stream(ctx, c.conn.Conn, search.Request{
		Base:       base,
		Filter:     filter,
		Scope:      scope,
		Attributes: columns,
	}, func(batch []search.Result) error {
		rows := make([]SearchRow, 0, len(batch))
		for _, r := range batch {
			rows = append(rows, searchRow(r, columns))
		}
		matched += len(rows)
		a.emit(EventSearchBatch, SearchBatch{Rows: rows, Matched: matched})
		return nil
	})

	done := SearchDone{
		Matched:   stats.Matched,
		Truncated: stats.Truncated,
		Reason:    stats.TruncateReason,
		Columns:   columns,
		Elapsed:   time.Since(started).Milliseconds(),
	}
	switch {
	case ctx.Err() != nil:
		// A cancelled search is not a failure; it is what Stop does.
		done.Cancelled = true
	case err != nil:
		done.Error = ldaperr.Explain(err)
	}
	a.emit(EventSearchDone, done)
}

// StopSearch cancels whatever is running on a connection.
func (a *App) StopSearch(profileID string) { a.stopSearch(profileID) }

// searchRow flattens one result into the fixed columns of the table.
func searchRow(r search.Result, columns []string) SearchRow {
	var classes []string
	for _, v := range r.Attributes["objectClass"] {
		classes = append(classes, v.Raw)
	}

	cells := make([]string, 0, len(columns))
	for _, name := range columns {
		if name == "distinguishedName" {
			cells = append(cells, r.DN)
			continue
		}
		parts := make([]string, 0, len(r.Attributes[name]))
		for _, v := range r.Attributes[name] {
			parts = append(parts, v.Raw)
		}
		// An absent attribute becomes an empty cell rather than a missing one:
		// the table has fixed columns, and a short row shifts everything after.
		cells = append(cells, strings.Join(parts, "; "))
	}

	return SearchRow{DN: r.DN, Icon: iconFor(classes), Cells: cells}
}

// columnsFor makes sure the distinguished name is the last column and appears
// exactly once. Without it a result cannot be identified.
func columnsFor(requested []string) []string {
	out := make([]string, 0, len(requested)+1)
	for _, c := range requested {
		if c != "distinguishedName" && c != "" {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		out = append(out, "cn", "objectClass")
	}
	return append(out, "distinguishedName")
}

// emit sends an event. It tolerates a nil context so the package stays
// testable without a window.
func (a *App) emit(name string, data any) {
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, name, data)
}
