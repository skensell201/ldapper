# Ldapper Search and Filters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish v1 — search that streams as it runs, the filter library with its editor, and export to LDIF and CSV.

**Architecture:** The engine already does all three; none of this adds directory logic. What it adds is an event bridge, because a search returning forty thousand objects cannot be one return value. Go emits batches as Wails events and the interface appends them; the same bridge is what a reconnect indicator will use later.

**Tech Stack:** As plan 2, plus the Wails runtime's `EventsEmit` and `SaveFileDialog`.

**Scope:** Plan 3 of 3. After this, the v1 boundaries from the spec are met.

**Depends on:** `main` at v0.1.0.

**Spec:** `docs/superpowers/specs/2026-08-05-ldapper-design.md`
**Mockup:** `docs/design/mockup-v1-superlist.html`, screens 02 and 03

---

## Decisions

| Decision | Why |
|---|---|
| **Results stream as events, not as a return value** | A subtree search can match tens of thousands of objects. Returning them in one call means the window is frozen until the last one arrives; emitting batches means the first rows are on screen in under a second. |
| **One search at a time per connection** | Starting a second search cancels the first. Two searches writing into one result table is a race with no useful outcome, and the alternative — a tab per search — is not v1. |
| **Export writes server-side, straight to the chosen path** | The alternative is carrying the whole result set into JavaScript and back. A subtree export is exactly the case where that runs out of memory. |
| **The filter editor validates on every keystroke** | An LDAP filter is easy to get subtly wrong, and the server's answer to a wrong one is a protocol error with no hint. Compiling it locally as you type is nearly free. |

---

## File Structure

| File | Responsibility |
|---|---|
| `app/search.go` | StartSearch, StopSearch, the events they emit |
| `app/filterlib.go` | SaveFilter, ResetFilter, DeleteFilter, RestoreDefaults, ValidateFilter |
| `app/export.go` | ExportSearch and ExportEntry, both writing to a chosen path |
| `frontend/src/panes/Results.tsx` | The streamed result table |
| `frontend/src/panes/Library.tsx` | Filter list and editor |
| `frontend/src/Toolbar.tsx` | Filter box, scope, base, run/stop, export |
| `frontend/src/store.ts` | Search state, event subscriptions, filter library state |

---

## Task 1: Search that streams

**Files:**
- Create: `app/search.go`
- Test: `app/search_test.go`

- [ ] **Step 1: Write the failing test**

```go
package app

import (
	"testing"

	"github.com/skensell201/ldapper/internal/decode"
	"github.com/skensell201/ldapper/internal/search"
)

func TestSearchRowFromResult(t *testing.T) {
	got := searchRow(search.Result{
		DN: "CN=Anna Volkova,OU=Users,DC=example,DC=com",
		Attributes: map[string][]decode.Value{
			"cn":          {{Raw: "Anna Volkova"}},
			"objectClass": {{Raw: "top"}, {Raw: "user"}},
		},
	}, []string{"cn", "objectClass", "mail"})

	if got.DN != "CN=Anna Volkova,OU=Users,DC=example,DC=com" {
		t.Errorf("DN = %q", got.DN)
	}
	if got.Icon != "user" {
		t.Errorf("Icon = %q, want user — the row draws the same glyph as the tree", got.Icon)
	}
	if len(got.Cells) != 3 {
		t.Fatalf("got %d cells, want one per requested column", len(got.Cells))
	}
	if got.Cells[0] != "Anna Volkova" {
		t.Errorf("cell 0 = %q", got.Cells[0])
	}
	// Multi-valued attributes join, so a column stays one line.
	if got.Cells[1] != "top; user" {
		t.Errorf("cell 1 = %q, want the values joined", got.Cells[1])
	}
	// A column the entry does not carry is empty, not missing: the table has
	// fixed columns and a short row would shift everything after it.
	if got.Cells[2] != "" {
		t.Errorf("cell 2 = %q, want empty for an absent attribute", got.Cells[2])
	}
}

func TestSearchColumnsAlwaysIncludeTheDN(t *testing.T) {
	got := columnsFor(nil)
	if len(got) == 0 || got[len(got)-1] != "distinguishedName" {
		t.Errorf("columnsFor(nil) = %v, want the DN as the last column", got)
	}

	got = columnsFor([]string{"cn", "distinguishedName"})
	var n int
	for _, c := range got {
		if c == "distinguishedName" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("columnsFor() = %v, want distinguishedName exactly once", got)
	}
}

func TestStartSearchWithoutAConnection(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	got := a.StartSearch(SearchInput{ProfileID: "nobody", Filter: "(objectClass=*)"})
	if got.Error == "" {
		t.Error("Error is empty; searching an unopened connection must say so")
	}
}

func TestStartSearchRejectsABadFilter(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()
	a.setLive("dc01", &liveConn{})

	got := a.StartSearch(SearchInput{ProfileID: "dc01", Filter: "(objectClass=user"})
	if got.Error == "" {
		t.Error("Error is empty for an unbalanced filter; it must never reach the server")
	}
}

func TestStopSearchIsSafeWhenNothingIsRunning(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()
	a.StopSearch("nobody") // must not panic
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./app/ -run Search -v`
Expected: FAIL — `undefined: searchRow`.

- [ ] **Step 3: Implement it**

```go
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

	expander := filters.Expander{Now: time.Now(), BindDN: c.conn.BindDN}
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

	// One search at a time: two of them writing into one table is a race with
	// no useful outcome.
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
		var parts []string
		for _, v := range r.Attributes[name] {
			parts = append(parts, v.Raw)
		}
		// An absent attribute becomes an empty cell rather than a missing one:
		// the table has fixed columns, and a short row shifts everything after.
		cells = append(cells, strings.Join(parts, "; "))
	}

	return SearchRow{DN: r.DN, Icon: iconFor(classes), Cells: cells}
}

// columnsFor makes sure the distinguished name is always the last column and
// appears exactly once. Without it a result is not identifiable.
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

// emit sends an event, tolerating a nil context so the package stays testable
// without a window.
func (a *App) emit(name string, data any) {
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, name, data)
}
```

Add the cancel registry to `app/app.go`:

```go
// searches holds the cancel function of the one search running on each
// connection.
searches map[string]context.CancelFunc
```

initialised in `New` and served by:

```go
func (a *App) setSearchCancel(id string, cancel context.CancelFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.searches[id] = cancel
}

func (a *App) stopSearch(id string) {
	a.mu.Lock()
	cancel := a.searches[id]
	delete(a.searches, id)
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (a *App) clearSearchCancel(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.searches, id)
}
```

`Shutdown` and `dropLive` must both call `stopSearch`, or a search outlives the connection it is reading from.

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -race ./app/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/search.go app/search_test.go app/app.go
git commit -m "Stream search results to the interface as events"
```

---

## Task 2: The filter library API

**Files:**
- Create: `app/filterlib.go`
- Test: `app/filterlib_test.go`

- [ ] **Step 1: Write the failing test**

```go
package app

import "testing"

func TestValidateFilterShowsWhatItExpandsTo(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	got := a.ValidateFilter("nobody", "(whenCreated<={{now-90d:generalized}})")
	if !got.Valid {
		t.Fatalf("Valid = false: %s", got.Error)
	}
	if got.Expanded == "" || got.Expanded == "(whenCreated<={{now-90d:generalized}})" {
		t.Errorf("Expanded = %q, want the substitution resolved", got.Expanded)
	}
}

func TestValidateFilterReportsWhatIsWrong(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	got := a.ValidateFilter("nobody", "(objectClass=user")
	if got.Valid {
		t.Error("Valid = true for an unbalanced filter")
	}
	if got.Error == "" {
		t.Error("Error is empty")
	}
}

func TestSaveFilterRejectsAnInvalidOne(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	msg := a.SaveFilter(FilterInput{ID: "mine", Name: "Mine", Filter: "(objectClass=user"})
	if msg == "" {
		t.Error("SaveFilter() stored a filter that cannot be compiled")
	}
}

func TestEditingABuiltinAndResettingIt(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if msg := a.SaveFilter(FilterInput{
		ID: "ad-disabled-accounts", Name: "Switched off",
		Filter: "(objectClass=user)", Scope: "subtree", Dialect: "ad",
	}); msg != "" {
		t.Fatalf("SaveFilter() = %q", msg)
	}

	var found bool
	for _, f := range a.ListFilters("nobody") {
		if f.ID == "ad-disabled-accounts" {
			found = true
			if !f.Modified {
				t.Error("the edited built-in is not marked modified")
			}
			if f.Name != "Switched off" {
				t.Errorf("Name = %q, want the edit", f.Name)
			}
		}
	}
	if !found {
		t.Fatal("the edited built-in disappeared from the library")
	}

	if msg := a.ResetFilter("ad-disabled-accounts"); msg != "" {
		t.Fatalf("ResetFilter() = %q", msg)
	}
	for _, f := range a.ListFilters("nobody") {
		if f.ID == "ad-disabled-accounts" && f.Modified {
			t.Error("the filter is still modified after Reset")
		}
	}
}

func TestRestoreDefaultsKeepsCustomFilters(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if msg := a.SaveFilter(FilterInput{ID: "mine", Name: "Mine", Filter: "(objectClass=*)"}); msg != "" {
		t.Fatalf("SaveFilter() = %q", msg)
	}
	if msg := a.DeleteFilter("posix-accounts"); msg != "" {
		t.Fatalf("DeleteFilter() = %q", msg)
	}
	if msg := a.RestoreDefaultFilters(); msg != "" {
		t.Fatalf("RestoreDefaultFilters() = %q", msg)
	}

	var sawMine, sawPosix bool
	for _, f := range a.ListFilters("nobody") {
		switch f.ID {
		case "mine":
			sawMine = true
		case "posix-accounts":
			sawPosix = true
		}
	}
	if !sawMine {
		t.Error("RestoreDefaultFilters() removed a custom filter, which it must never do")
	}
	if !sawPosix {
		t.Error("RestoreDefaultFilters() did not bring back the deleted built-in")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./app/ -run Filter -v`
Expected: FAIL — `undefined: FilterInput`.

- [ ] **Step 3: Implement it**

```go
package app

import (
	"time"

	"github.com/skensell201/ldapper/internal/filters"
)

// FilterInput is what the filter editor sends back.
type FilterInput struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Filter      string   `json:"filter"`
	Scope       string   `json:"scope"`
	Base        string   `json:"base"`
	Columns     []string `json:"columns"`
	Dialect     string   `json:"dialect"`
}

// FilterCheck is what the editor shows under the filter box as you type.
type FilterCheck struct {
	Valid    bool   `json:"valid"`
	Error    string `json:"error,omitempty"`
	Expanded string `json:"expanded,omitempty"`
}

// ValidateFilter compiles a filter and shows what its substitutions resolve
// to. Doing this locally matters: the server's answer to a malformed filter is
// a protocol error that says nothing about which part was wrong.
func (a *App) ValidateFilter(profileID, filter string) FilterCheck {
	bindDN := ""
	if c, ok := a.live(profileID); ok {
		bindDN = c.conn.BindDN
	}
	e := filters.Expander{Now: time.Now(), BindDN: bindDN}

	if err := filters.Validate(filter, e); err != nil {
		return FilterCheck{Error: err.Error()}
	}
	expanded, err := e.Expand(filter)
	if err != nil {
		return FilterCheck{Error: err.Error()}
	}
	return FilterCheck{Valid: true, Expanded: expanded}
}

// SaveFilter stores a filter after checking that it compiles.
func (a *App) SaveFilter(in FilterInput) string {
	if check := a.ValidateFilter("", in.Filter); !check.Valid {
		return check.Error
	}

	err := a.filters.Save(filters.Filter{
		ID:          in.ID,
		Name:        in.Name,
		Description: in.Description,
		Filter:      in.Filter,
		Scope:       filters.Scope(in.Scope),
		Base:        in.Base,
		Columns:     in.Columns,
		Dialect:     filters.Dialect(in.Dialect),
	})
	if err != nil {
		return err.Error()
	}
	return ""
}

// ResetFilter returns a built-in filter to the version Ldapper ships.
func (a *App) ResetFilter(id string) string {
	if err := a.filters.Reset(id); err != nil {
		return err.Error()
	}
	return ""
}

// DeleteFilter removes a filter. A built-in stays gone across restarts.
func (a *App) DeleteFilter(id string) string {
	if err := a.filters.Delete(id); err != nil {
		return err.Error()
	}
	return ""
}

// RestoreDefaultFilters undoes every edit and deletion of a built-in, leaving
// the user's own filters alone.
func (a *App) RestoreDefaultFilters() string {
	if err := a.filters.RestoreDefaults(); err != nil {
		return err.Error()
	}
	return ""
}
```

Note that `ValidateFilter("", …)` is called with no profile, so `{{me}}` in a
saved filter is rejected at save time. That is deliberate: a filter that only
compiles while connected is a filter that fails at the worst moment.

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -race ./app/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/filterlib.go app/filterlib_test.go
git commit -m "Expose the filter library's editing operations"
```

---

## Task 3: Export

**Files:**
- Create: `app/export.go`
- Test: `app/export_test.go`

- [ ] **Step 1: Write the failing test**

```go
package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriterForPicksTheFormatFromTheExtension(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"out.ldif", "out.LDIF"} {
		path := filepath.Join(dir, name)
		f, w, err := writerFor(path, []string{"cn"})
		if err != nil {
			t.Fatalf("writerFor(%q) returned error: %v", name, err)
		}
		if err := w.Write("cn=x,dc=example,dc=com", map[string][]string{"cn": {"x"}}); err != nil {
			t.Fatalf("Write() returned error: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close() returned error: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("file Close() returned error: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(data), "version: 1") {
			t.Errorf("%s is not LDIF:\n%s", name, data)
		}
	}

	path := filepath.Join(dir, "out.csv")
	f, w, err := writerFor(path, []string{"cn"})
	if err != nil {
		t.Fatalf("writerFor() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "dn,cn") {
		t.Errorf("out.csv is not CSV:\n%s", data)
	}
}

func TestWriterForRejectsAnUnknownExtension(t *testing.T) {
	if _, _, err := writerFor(filepath.Join(t.TempDir(), "out.txt"), []string{"cn"}); err == nil {
		t.Error("writerFor() accepted an extension it cannot write")
	}
}

func TestExportWithoutAConnection(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if msg := a.ExportSearch(ExportInput{ProfileID: "nobody", Path: "/tmp/x.ldif"}); msg == "" {
		t.Error("ExportSearch() succeeded with no connection open")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./app/ -run Export -v`
Expected: FAIL — `undefined: writerFor`.

- [ ] **Step 3: Implement it**

```go
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skensell201/ldapper/internal/export"
	"github.com/skensell201/ldapper/internal/filters"
	"github.com/skensell201/ldapper/internal/ldaperr"
	"github.com/skensell201/ldapper/internal/search"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportInput is what the export button sends.
type ExportInput struct {
	ProfileID string   `json:"profileId"`
	Path      string   `json:"path"`
	Filter    string   `json:"filter"`
	Scope     string   `json:"scope"`
	Base      string   `json:"base"`
	Columns   []string `json:"columns"`
}

// ChooseExportPath opens the system's save dialog and returns the chosen path,
// or an empty string if the user cancelled.
func (a *App) ChooseExportPath(suggested string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app: no window to open a dialog from")
	}
	return wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		DefaultFilename: suggested,
		Filters: []wruntime.FileFilter{
			{DisplayName: "LDIF (*.ldif)", Pattern: "*.ldif"},
			{DisplayName: "CSV (*.csv)", Pattern: "*.csv"},
		},
	})
}

// ExportSearch runs a search and writes every result to path. It streams
// straight to the file: carrying a subtree through JavaScript and back is
// exactly the case that runs out of memory.
func (a *App) ExportSearch(in ExportInput) string {
	c, ok := a.live(in.ProfileID)
	if !ok {
		return "That connection is not open. Connect first."
	}

	e := filters.Expander{Now: time.Now(), BindDN: c.conn.BindDN}
	if err := filters.Validate(in.Filter, e); err != nil {
		return err.Error()
	}
	expanded, err := e.Expand(in.Filter)
	if err != nil {
		return err.Error()
	}

	base := in.Base
	if base == "" {
		base = c.info.RootDN()
	}
	columns := columnsFor(in.Columns)

	f, w, err := writerFor(in.Path, columns)
	if err != nil {
		return err.Error()
	}
	defer f.Close()

	_, err = search.Stream(context.Background(), c.conn.Conn, search.Request{
		Base:       base,
		Filter:     expanded,
		Scope:      in.Scope,
		Attributes: columns,
	}, func(batch []search.Result) error {
		for _, r := range batch {
			if err := w.Write(r.DN, flatten(r)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ldaperr.Explain(err)
	}
	if err := w.Close(); err != nil {
		return err.Error()
	}
	return ""
}

// writerFor opens path and picks the format from its extension.
func writerFor(path string, columns []string) (*os.File, export.Writer, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".ldif" && ext != ".csv" {
		return nil, nil, fmt.Errorf("export: %q is neither .ldif nor .csv", filepath.Base(path))
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("export: cannot write %s: %w", path, err)
	}

	if ext == ".ldif" {
		return f, export.NewLDIF(f), nil
	}
	w, err := export.NewCSV(f, columns)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, w, nil
}

// flatten drops the decoded renderings: an export carries what the server
// stores, not our reading of it.
func flatten(r search.Result) map[string][]string {
	out := make(map[string][]string, len(r.Attributes))
	for name, values := range r.Attributes {
		for _, v := range values {
			out[name] = append(out[name], v.Raw)
		}
	}
	return out
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -race ./app/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/export.go app/export_test.go
git commit -m "Export search results straight to a chosen file"
```

---

## Task 4: Search and library state in the store

**Files:**
- Modify: `frontend/src/store.ts`

- [ ] **Step 1: Add the state and the event subscriptions**

Results arrive as events, so the store subscribes once at start-up rather than
awaiting a call that would never return in time.

```ts
// Added to the State interface
  mode: "browse" | "search" | "library";
  filterText: string;
  scope: string;
  searching: boolean;
  results: app.SearchRow[];
  columns: string[];
  matched: number;
  searchNote: string;
  library: app.FilterSummary[];
  editing: app.FilterSummary | null;
  check: app.FilterCheck | null;

  setMode: (m: "browse" | "search" | "library") => void;
  setFilterText: (s: string) => void;
  runSearch: () => Promise<void>;
  stopSearch: () => Promise<void>;
  loadLibrary: () => Promise<void>;
  editFilter: (f: app.FilterSummary | null) => void;
  checkFilter: (s: string) => Promise<void>;
  saveFilter: (f: app.FilterInput) => Promise<void>;
  resetFilter: (id: string) => Promise<void>;
  deleteFilter: (id: string) => Promise<void>;
  restoreFilters: () => Promise<void>;
  useFilter: (f: app.FilterSummary) => void;
  exportResults: () => Promise<void>;
```

with the implementations, and at the bottom of the file:

```ts
// Results arrive as events. Subscribing once at module load is what keeps a
// forty-thousand-row search from being one frozen call.
EventsOn("search:batch", (batch: app.SearchBatch) => {
  useStore.setState((s) => ({
    results: [...s.results, ...(batch.rows ?? [])],
    matched: batch.matched,
  }));
});

EventsOn("search:done", (done: app.SearchDone) => {
  useStore.setState({
    searching: false,
    matched: done.matched,
    columns: done.columns ?? [],
    searchNote: done.error || done.reason || (done.cancelled ? "stopped" : `${done.matched} in ${done.elapsed} ms`),
  });
});
```

- [ ] **Step 2: Build and commit**

```bash
cd frontend && npm run build
git add frontend/src/store.ts && git commit -m "Hold search and filter library state, fed by events"
```

---

## Task 5: The toolbar, the results table and the library

**Files:**
- Create: `frontend/src/Toolbar.tsx`, `frontend/src/panes/Results.tsx`, `frontend/src/panes/Library.tsx` and their stylesheets
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Write them**

The toolbar carries the filter box, the scope selector and run/stop/export.
The results table is virtualised like the tree, for the same reason. The
library is a list beside an editor, with the dialect marked on every row and a
reason on the ones this server cannot answer.

Full component code follows the mockup's screens 02 and 03; see the mockup for
the exact chrome.

- [ ] **Step 2: Verify against a real directory**

```bash
docker compose -f test/integration/docker-compose.yml up -d --wait
wails build && open build/bin/Ldapper.app
```

- [ ] Searching `(objectClass=inetOrgPerson)` fills the table as it runs and ends at 122
- [ ] Stop halts a search partway and says so
- [ ] The library lists 18 filters with POSIX ones available and Active Directory ones greyed
- [ ] Editing a built-in marks it, and Reset clears the mark
- [ ] Export writes a file the shell can read back

- [ ] **Step 3: Commit**

---

## Definition of done

- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` reports nothing
- [ ] `npm run build` has no TypeScript errors
- [ ] The manual checks in Task 5 pass
- [ ] CI green, and v0.2.0 published with both platforms

---

## Self-review against the spec

| Spec section | Covered by |
|---|---|
| 3.3 search, streamed | Task 1 |
| 3.6 filters, edited and reset | Task 2 |
| 3.8 export, LDIF and CSV | Task 3 |
| 4, screen 2 (search) | Tasks 4, 5 |
| 4, screen 3 (filter library) | Tasks 4, 5 |
| 3.1 reconnect after a drop | **Still deferred.** The event bridge this plan builds is what it needs, but it also needs a failure to reconnect *from*, which no test currently produces. Worth its own task once there is a way to drop a connection on purpose. |
| 3.4 attribute syntaxes | **Still deferred.** Presentation only, and nothing reads it yet. |

With this, the v1 boundaries in spec section 7 are met: read, search, filter,
export. Writing, snapshots and Kerberos remain out of scope by design.
