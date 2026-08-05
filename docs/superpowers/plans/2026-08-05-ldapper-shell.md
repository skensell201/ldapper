# Ldapper Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the engine from plan 1 into an application that opens: connect to a directory, judge its certificate, walk its tree, and read an object's attributes.

**Architecture:** A Wails v2 desktop shell. The Go side adds one thin package — `app` — that holds live connections and translates domain types into DTOs; it contains no directory logic, because all of that already exists in `internal/` and is already tested. The React side is a three-pane window styled from the Superlist tokens, with both the tree and the attribute table virtualised from the first commit.

**Tech Stack:** Wails v2.13.0, Go 1.26, React 19 + TypeScript + Vite, Zustand for state, `@tanstack/react-virtual` for the lists, hand-written CSS with custom properties.

**Scope:** Plan 2 of 3. Search, the filter library screen, export and packaging are plan 3. This plan ends with a window that browses a real directory.

**Depends on:** the `core-engine` branch (PR #1). Branch from it, not from `main`.

**Spec:** `docs/superpowers/specs/2026-08-05-ldapper-design.md`
**Mockup:** `docs/design/mockup-v1-superlist.html`

---

## Decisions taken before writing this plan

Three were checked against reality rather than assumed:

| Decision | Why |
|---|---|
| **Wails v2.13.0, not v3** | v3 reached its first beta on 2026-08-03, after eighteen months of alpha; its API still moves week to week. v2.13.0 is stable, was released 2026-07-06, and is what a tool aimed at administrators should be built on. Revisit for v2 of Ldapper, not now. |
| **Plain CSS with custom properties, no Tailwind** | The mockup is already written this way, and the Superlist system is a token set, not a utility vocabulary. Porting it verbatim keeps the design review meaningful. |
| **Zustand, not Context** | The tree cache is keyed by DN and updated from streamed events. Context re-renders every consumer on each update; a store with selectors does not, and with fifty thousand nodes that difference is the whole frame budget. |

---

## File Structure

| File | Responsibility |
|---|---|
| `wails.json`, `main.go` | Wails project definition and entry point |
| `internal/config/paths.go` | Per-OS locations for `filters.json` and `connections.json` |
| `app/app.go` | The bound struct: lifecycle, stores, the live-connection registry |
| `app/dto.go` | The types crossing into TypeScript, and the mapping onto them |
| `app/icon.go` | objectClass values → the icon name the tree draws |
| `app/connect.go` | Connect, Disconnect, TrustCertificate |
| `app/browse.go` | Children of a node, and one entry's attributes |
| `app/library.go` | Profiles and filters passed through to the interface |
| `frontend/src/tokens.css` | The Superlist palette, radii and shadows |
| `frontend/src/App.tsx` | Window shell: utility bar, panes, status bar |
| `frontend/src/store.ts` | Zustand store: connection, tree cache, selection |
| `frontend/src/panes/Tree.tsx` | Virtualised directory tree with paged loading |
| `frontend/src/panes/Detail.tsx` | Virtualised attribute table |
| `frontend/src/dialogs/Connect.tsx` | Connection form |
| `frontend/src/dialogs/Certificate.tsx` | Fingerprint prompt |

---

## Task 1: The Wails skeleton

**Files:**
- Create: `wails.json`, `main.go`, `frontend/` (generated)

- [ ] **Step 1: Confirm the toolchain**

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
wails doctor
```

Expected: "Your system is ready for Wails development!". NSIS may be reported missing — that is the Windows installer, which plan 3 needs and this plan does not.

- [ ] **Step 2: Scaffold the frontend in place**

Wails' own `wails init` wants an empty directory, and this repository is not one. Create the frontend by hand instead:

```bash
mkdir -p frontend
cd frontend && npm create vite@latest . -- --template react-ts && npm install
npm install zustand @tanstack/react-virtual
```

- [ ] **Step 3: Write wails.json**

```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "Ldapper",
  "outputfilename": "Ldapper",
  "frontend:install": "npm install",
  "frontend:build": "npm run build",
  "frontend:dev:watcher": "npm run dev",
  "frontend:dev:serverUrl": "auto",
  "author": {
    "name": "Ldapper"
  },
  "info": {
    "productName": "Ldapper",
    "comments": "A desktop explorer for LDAP and Active Directory"
  }
}
```

- [ ] **Step 4: Write main.go**

```go
// Command Ldapper is a desktop explorer for LDAP and Active Directory
// directories.
package main

import (
	"embed"
	"log"

	"github.com/skensell201/ldapper/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("Ldapper cannot start: %v", err)
	}

	err = wails.Run(&options.App{
		Title:  "Ldapper",
		Width:  1280,
		Height: 800,
		MinWidth:  960,
		MinHeight: 600,
		AssetServer: &assetserver.Options{Assets: assets},
		// The window chrome is part of the design, so it matches the canvas
		// rather than sitting on the system's own grey.
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 36, A: 1},
		OnStartup:        a.Startup,
		OnShutdown:       a.Shutdown,
		Bind:             []any{a},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		log.Fatalf("Ldapper stopped: %v", err)
	}
}
```

- [ ] **Step 5: Verify it builds and commit**

The `app` package does not exist yet, so this will not compile until Task 3. Commit the scaffolding on its own:

```bash
git add wails.json frontend/ .gitignore
git commit -m "Scaffold the Wails project and its React frontend"
```

Add `frontend/node_modules/`, `frontend/dist/` and `build/bin/` to `.gitignore` first if they are not already covered.

---

## Task 2: Where configuration lives

Every store needs a path, and each operating system puts it somewhere different. Getting this wrong means a user's saved filters vanish on upgrade.

**Files:**
- Create: `internal/config/paths.go`
- Test: `internal/config/paths_test.go`

- [ ] **Step 1: Write the failing test**

```go
package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDirIsUnderTheUserConfigDirectory(t *testing.T) {
	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir() returned error: %v", err)
	}
	if !strings.HasSuffix(got, appName) && !strings.HasSuffix(got, strings.ToLower(appName)) {
		t.Errorf("Dir() = %q, want it to end in the application name", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("Dir() = %q, want an absolute path", got)
	}
}

func TestFilePathsSitSideBySide(t *testing.T) {
	filters, err := FiltersPath()
	if err != nil {
		t.Fatalf("FiltersPath() returned error: %v", err)
	}
	connections, err := ConnectionsPath()
	if err != nil {
		t.Fatalf("ConnectionsPath() returned error: %v", err)
	}

	if filepath.Dir(filters) != filepath.Dir(connections) {
		t.Errorf("the two files are in different directories: %q and %q", filters, connections)
	}
	if filepath.Base(filters) != "filters.json" {
		t.Errorf("FiltersPath() = %q, want it to end in filters.json", filters)
	}
	if filepath.Base(connections) != "connections.json" {
		t.Errorf("ConnectionsPath() = %q, want it to end in connections.json", connections)
	}
}

// An override keeps tests and portable installs off the real configuration.
func TestOverrideWins(t *testing.T) {
	t.Setenv(dirEnv, t.TempDir())

	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir() returned error: %v", err)
	}
	if got != strings.TrimSuffix(got, appName) {
		// The override is used as-is: no application name appended.
		t.Errorf("Dir() = %q, want the override used verbatim", got)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/config/ -v`
Expected: FAIL — `undefined: Dir`.

- [ ] **Step 3: Implement it**

```go
// Package config decides where Ldapper keeps its files. Each operating system
// has its own answer, and getting it wrong means a user's saved filters
// disappear on upgrade.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// appName is the directory Ldapper creates inside the platform's own
// configuration location.
const appName = "Ldapper"

// dirEnv overrides the location entirely. Tests use it, and so does anyone
// running Ldapper from a memory stick.
const dirEnv = "LDAPPER_CONFIG_DIR"

// Dir is where Ldapper's files live:
//
//	Windows  %AppData%\Ldapper
//	macOS    ~/Library/Application Support/Ldapper
//	Linux    $XDG_CONFIG_HOME/Ldapper, or ~/.config/Ldapper
func Dir() (string, error) {
	if override := os.Getenv(dirEnv); override != "" {
		return override, nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: cannot find this system's configuration directory: %w", err)
	}
	return filepath.Join(base, appName), nil
}

// FiltersPath is where the filter overlay is stored.
func FiltersPath() (string, error) { return pathTo("filters.json") }

// ConnectionsPath is where saved connections are stored.
func ConnectionsPath() (string, error) { return pathTo("connections.json") }

func pathTo(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "Decide where Ldapper keeps its configuration on each platform"
```

---

## Task 3: Icons from object classes

Which icon a tree row draws is decided from its `objectClass` values. It is a pure function, so it gets a real test rather than being buried in a component.

**Files:**
- Create: `app/icon.go`
- Test: `app/icon_test.go`

- [ ] **Step 1: Write the failing test**

```go
package app

import "testing"

func TestIconFor(t *testing.T) {
	tests := []struct {
		name    string
		classes []string
		want    string
	}{
		{"a person", []string{"top", "person", "organizationalPerson", "user"}, "user"},
		{"an inetOrgPerson", []string{"top", "inetOrgPerson"}, "user"},
		{"an AD group", []string{"top", "group"}, "group"},
		{"a groupOfNames", []string{"top", "groupOfNames"}, "group"},
		{"a posixGroup", []string{"posixGroup"}, "group"},
		{"an organizational unit", []string{"top", "organizationalUnit"}, "ou"},
		{"a computer", []string{"top", "person", "computer"}, "computer"},
		{"a domain", []string{"top", "domain", "domainDNS"}, "domain"},
		{"a dcObject", []string{"top", "dcObject", "organization"}, "domain"},
		{"a container", []string{"top", "container"}, "container"},
		{"anything else", []string{"top", "printQueue"}, "object"},
		{"nothing at all", nil, "object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := iconFor(tt.classes); got != tt.want {
				t.Errorf("iconFor(%v) = %q, want %q", tt.classes, got, tt.want)
			}
		})
	}
}

// A computer is also a person in Active Directory's schema. The more specific
// class has to win, or every domain controller draws as a user.
func TestIconPrefersTheMoreSpecificClass(t *testing.T) {
	if got := iconFor([]string{"user", "computer", "person"}); got != "computer" {
		t.Errorf("iconFor() = %q, want computer to beat user", got)
	}
}

func TestIconIsCaseInsensitive(t *testing.T) {
	if got := iconFor([]string{"ORGANIZATIONALUNIT"}); got != "ou" {
		t.Errorf("iconFor() = %q, want ou", got)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./app/ -run TestIcon -v`
Expected: FAIL — `undefined: iconFor`.

- [ ] **Step 3: Implement it**

```go
package app

import "strings"

// iconRule maps an objectClass to an icon name. Order matters: the first rule
// that matches wins, so the most specific classes are listed first. In Active
// Directory a computer is also a user and a person, and without that ordering
// every domain controller would draw as somebody's account.
var iconRules = []struct {
	class string
	icon  string
}{
	{"computer", "computer"},
	{"organizationalunit", "ou"},
	{"group", "group"},
	{"groupofnames", "group"},
	{"groupofuniquenames", "group"},
	{"posixgroup", "group"},
	{"domaindns", "domain"},
	{"domain", "domain"},
	{"dcobject", "domain"},
	{"container", "container"},
	{"builtindomain", "container"},
	{"user", "user"},
	{"inetorgperson", "user"},
	{"posixaccount", "user"},
	{"person", "user"},
}

// iconFor picks the icon a tree row should draw.
func iconFor(classes []string) string {
	present := make(map[string]bool, len(classes))
	for _, c := range classes {
		present[strings.ToLower(c)] = true
	}

	for _, rule := range iconRules {
		if present[rule.class] {
			return rule.icon
		}
	}
	return "object"
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./app/ -run TestIcon -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/icon.go app/icon_test.go
git commit -m "Choose a tree icon from an entry's object classes"
```

---

## Task 4: The types that cross into TypeScript

Wails generates TypeScript from these structs, so their shape is the contract between the two halves of the program. Anything awkward here shows up in every component.

**Files:**
- Create: `app/dto.go`
- Test: `app/dto_test.go`

- [ ] **Step 1: Write the failing test**

```go
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

func TestNodeLabelKeepsAnEscapedComma(t *testing.T) {
	got := nodeFrom(browse.Entry{RDN: `CN=Volkova\, Anna`})
	if got.Label != `Volkova\, Anna` {
		t.Errorf("Label = %q, want everything after the first equals sign", got.Label)
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
	if rows[1].Name == "objectClass" && len(rows[1].Values) != 2 {
		t.Errorf("objectClass has %d values, want 2", len(rows[1].Values))
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

// Every DTO has to survive a round trip through JSON, because that is how it
// reaches the interface.
func TestDTOsMarshal(t *testing.T) {
	for _, v := range []any{
		Node{},
		AttributeRow{},
		EntryDetail{},
		NodePage{},
		ConnectionState{},
		CertPrompt{},
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
	if !strings.Contains(string(data), `"dn"`) {
		t.Errorf("Node marshals to %s, want a lower-case dn field", data)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./app/ -run 'Node|Attribute|DTO|JSON' -v`
Expected: FAIL — `undefined: nodeFrom`.

- [ ] **Step 3: Implement it**

```go
// Package app is the surface Ldapper's interface calls. It holds the live
// connections and translates domain types into the shapes TypeScript sees.
// It deliberately contains no directory logic: that lives in internal/, where
// it is tested without a window in the way.
package app

import (
	"sort"
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
	// Page is which page this is, counting from one, so the tree can say
	// "loading page 2 of 4" instead of showing a spinner.
	Page int `json:"page"`
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

// labelOf strips the attribute name from an RDN: CN=Anna Volkova becomes
// Anna Volkova. An RDN with no equals sign is left alone.
func labelOf(rdn string) string {
	if i := strings.IndexByte(rdn, '='); i >= 0 {
		return rdn[i+1:]
	}
	return rdn
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
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./app/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/dto.go app/dto_test.go
git commit -m "Define the types that cross from Go into TypeScript"
```

---

## Task 5: The application object and its connection registry

**Files:**
- Create: `app/app.go`
- Test: `app/app_test.go`

- [ ] **Step 1: Write the failing test**

```go
package app

import (
	"testing"
)

func TestNewLoadsBothStores(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())

	a, err := New()
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if a.Filters() == nil {
		t.Error("Filters() is nil")
	}
	if a.Profiles() == nil {
		t.Error("Profiles() is nil")
	}
}

func TestNewOnAFreshMachineHasTheBuiltinFilters(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())

	a, err := New()
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if got := len(a.Filters().All()); got != 18 {
		t.Errorf("a fresh install has %d filters, want the 18 built-ins", got)
	}
}

func TestLiveConnectionRegistry(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if _, ok := a.live("nobody"); ok {
		t.Error("live() found a connection that was never opened")
	}

	a.setLive("dc01", &liveConn{state: ConnectionState{ProfileID: "dc01", Connected: true}})
	got, ok := a.live("dc01")
	if !ok {
		t.Fatal("live() lost the connection just registered")
	}
	if !got.state.Connected {
		t.Error("the registered connection is not marked connected")
	}

	a.dropLive("dc01")
	if _, ok := a.live("dc01"); ok {
		t.Error("dropLive() left the connection behind")
	}
}

// Shutdown must close every connection, not just the current one.
func TestShutdownClosesEverything(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	var closed int
	for _, id := range []string{"dc01", "dc02"} {
		a.setLive(id, &liveConn{
			state:  ConnectionState{ProfileID: id},
			cancel: func() { closed++ },
		})
	}

	a.Shutdown(nil)

	if closed != 2 {
		t.Errorf("%d connections were closed, want 2", closed)
	}
	if _, ok := a.live("dc01"); ok {
		t.Error("Shutdown() left a connection in the registry")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./app/ -run 'New|Live|Shutdown' -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Implement it**

```go
package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/skensell201/ldapper/internal/config"
	"github.com/skensell201/ldapper/internal/filters"
	"github.com/skensell201/ldapper/internal/profiles"
	"github.com/skensell201/ldapper/internal/schema"
	"github.com/skensell201/ldapper/internal/session"
)

// liveConn is one open connection and what the interface knows about it.
type liveConn struct {
	conn   *session.Conn
	info   schema.Info
	state  ConnectionState
	cancel context.CancelFunc
}

// App is the object Wails binds. Every exported method on it becomes a
// function the interface can call.
type App struct {
	ctx      context.Context
	filters  *filters.Store
	profiles *profiles.Store

	mu    sync.RWMutex
	conns map[string]*liveConn
}

// New loads the stores. It is called before the window exists, so it must not
// depend on anything Wails provides.
func New() (*App, error) {
	filtersPath, err := config.FiltersPath()
	if err != nil {
		return nil, err
	}
	connectionsPath, err := config.ConnectionsPath()
	if err != nil {
		return nil, err
	}

	f, err := filters.NewStore(filtersPath)
	if err != nil {
		return nil, fmt.Errorf("app: cannot load the filter library: %w", err)
	}
	p, err := profiles.NewStore(connectionsPath)
	if err != nil {
		return nil, fmt.Errorf("app: cannot load the saved connections: %w", err)
	}

	return &App{filters: f, profiles: p, conns: map[string]*liveConn{}}, nil
}

// Startup receives the Wails context, which is what event emission needs.
func (a *App) Startup(ctx context.Context) { a.ctx = ctx }

// Shutdown closes every open connection. Leaving one behind would hold a
// socket open past the window closing.
func (a *App) Shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for id, c := range a.conns {
		if c.cancel != nil {
			c.cancel()
		}
		delete(a.conns, id)
	}
}

// Filters exposes the filter library to the rest of the package.
func (a *App) Filters() *filters.Store { return a.filters }

// Profiles exposes the connection store to the rest of the package.
func (a *App) Profiles() *profiles.Store { return a.profiles }

func (a *App) live(id string) (*liveConn, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	c, ok := a.conns[id]
	return c, ok
}

func (a *App) setLive(id string, c *liveConn) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.conns[id] = c
}

func (a *App) dropLive(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if c, ok := a.conns[id]; ok && c.cancel != nil {
		c.cancel()
	}
	delete(a.conns, id)
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./app/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/app.go app/app_test.go
git commit -m "Add the bound application object and its connection registry"
```

---

## Task 6: Connect, trust and disconnect

**Files:**
- Create: `app/connect.go`
- Test: `app/connect_test.go`

- [ ] **Step 1: Write the failing test**

The dialling itself is covered by the session package and the integration tests. What belongs here is the shape of the result — in particular that an untrusted certificate arrives as something the interface can render, not as an error string.

```go
package app

import (
	"errors"
	"testing"
	"time"

	"github.com/skensell201/ldapper/internal/session"
)

func TestConnectResultCarriesACertificatePrompt(t *testing.T) {
	certErr := &session.CertError{
		Fingerprint: "4B:9C:1E:07",
		Subject:     "CN=dc01.corp.example.com",
		Issuer:      "CN=CORP-ROOT-CA",
		NotAfter:    time.Date(2027, 2, 11, 0, 0, 0, 0, time.UTC),
	}

	got := connectResultFor(certErr)

	if got.Trusted {
		t.Error("Trusted = true for a certificate nobody has approved")
	}
	if got.Certificate == nil {
		t.Fatal("Certificate is nil; the dialog has nothing to show")
	}
	if got.Certificate.Fingerprint != "4B:9C:1E:07" {
		t.Errorf("Fingerprint = %q", got.Certificate.Fingerprint)
	}
	if got.Certificate.NotAfter != "2027-02-11" {
		t.Errorf("NotAfter = %q, want a plain date", got.Certificate.NotAfter)
	}
	if got.Error != "" {
		t.Errorf("Error = %q, want the certificate reported through Certificate instead", got.Error)
	}
}

func TestConnectResultExplainsOtherFailures(t *testing.T) {
	got := connectResultFor(errors.New("dial tcp 10.0.0.1:636: connect: connection refused"))

	if got.Certificate != nil {
		t.Error("Certificate is set for a failure that has nothing to do with certificates")
	}
	if got.Error == "" {
		t.Error("Error is empty; the user would see nothing at all")
	}
}

func TestConnectResultForSuccess(t *testing.T) {
	got := connectResultFor(nil)
	if !got.Trusted || got.Error != "" || got.Certificate != nil {
		t.Errorf("connectResultFor(nil) = %+v, want a clean success", got)
	}
}

func TestEncryptionMapping(t *testing.T) {
	tests := []struct {
		in   string
		want session.Encryption
	}{
		{"ldaps", session.EncryptionLDAPS},
		{"starttls", session.EncryptionStartTLS},
		{"none", session.EncryptionNone},
		{"", session.EncryptionNone},
		{"nonsense", session.EncryptionNone},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := encryptionOf(tt.in); got != tt.want {
				t.Errorf("encryptionOf(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./app/ -run 'Connect|Encryption' -v`
Expected: FAIL — `undefined: connectResultFor`.

- [ ] **Step 3: Implement it**

```go
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/skensell201/ldapper/internal/ldaperr"
	"github.com/skensell201/ldapper/internal/profiles"
	"github.com/skensell201/ldapper/internal/schema"
	"github.com/skensell201/ldapper/internal/session"
)

// ConnectResult is what the connect dialog acts on.
type ConnectResult struct {
	// Trusted is false when the server's certificate needs a decision. The
	// dialog shows Certificate and, if the user accepts, calls Connect again.
	Trusted bool `json:"trusted"`
	// Certificate is set only when Trusted is false.
	Certificate *CertPrompt `json:"certificate,omitempty"`
	// Error is a sentence, already translated out of LDAP result codes.
	Error string          `json:"error,omitempty"`
	State ConnectionState `json:"state"`
}

// Connect opens a connection for a saved profile and reads what the server
// says about itself.
func (a *App) Connect(profileID, password string) ConnectResult {
	p, ok := a.profiles.Get(profileID)
	if !ok {
		return ConnectResult{Error: fmt.Sprintf("There is no saved connection called %q.", profileID)}
	}

	if password == "" {
		loaded, err := a.profiles.LoadPassword(p)
		if err == nil {
			p = loaded
		}
		// A missing keychain is not fatal: the dialog asks for the password.
	} else {
		p = p.WithPassword(password)
	}

	// The connection outlives this call, so its context is cancelled by
	// Disconnect or Shutdown rather than by returning from here.
	ctx, cancel := context.WithCancel(context.Background())

	conn, err := session.Dial(ctx, session.Config{
		Host:                p.Host,
		Port:                p.Port,
		Encryption:          encryptionOf(string(p.Encryption)),
		TrustedFingerprints: p.TrustedFingerprints,
	})
	if err != nil {
		cancel()
		return connectResultFor(err)
	}

	switch p.BindMethod {
	case profiles.BindNTLM:
		domain, account := session.SplitAccount(p.Username)
		if domain == "" {
			domain = p.Domain
		}
		err = conn.BindNTLM(domain, account, p.Password())
	default:
		err = conn.BindSimple(p.Username, p.Password())
	}
	if err != nil {
		cancel()
		return connectResultFor(err)
	}

	info, err := schema.Read(conn.Conn)
	if err != nil {
		cancel()
		return connectResultFor(err)
	}
	info, _ = schema.ReadObjectClasses(conn.Conn, info)

	var dialects []string
	for _, d := range info.Dialects() {
		dialects = append(dialects, string(d))
	}

	state := ConnectionState{
		ProfileID:         p.ID,
		Connected:         true,
		Host:              p.Host,
		BoundAs:           conn.BindDN,
		RootDN:            info.RootDN(),
		IsActiveDirectory: info.IsActiveDirectory(),
		SupportsPaging:    info.SupportsPaging(),
		Dialects:          dialects,
		Encryption:        string(p.Encryption),
	}

	a.setLive(p.ID, &liveConn{conn: conn, info: info, state: state, cancel: cancel})

	result := connectResultFor(nil)
	result.State = state
	return result
}

// TrustCertificate records a fingerprint against one profile. The dialog calls
// it and then calls Connect again.
func (a *App) TrustCertificate(profileID, fingerprint string) string {
	if err := a.profiles.Trust(profileID, fingerprint); err != nil {
		return err.Error()
	}
	return ""
}

// Disconnect closes one connection.
func (a *App) Disconnect(profileID string) {
	a.dropLive(profileID)
}

// connectResultFor turns whatever went wrong into something the interface can
// act on: a certificate decision, a sentence, or nothing at all.
func connectResultFor(err error) ConnectResult {
	if err == nil {
		return ConnectResult{Trusted: true}
	}

	var certErr *session.CertError
	if errors.As(err, &certErr) {
		return ConnectResult{
			Certificate: &CertPrompt{
				Fingerprint: certErr.Fingerprint,
				Subject:     certErr.Subject,
				Issuer:      certErr.Issuer,
				NotAfter:    certErr.NotAfter.Format("2006-01-02"),
			},
		}
	}

	return ConnectResult{Trusted: true, Error: ldaperr.Explain(err)}
}

func encryptionOf(s string) session.Encryption {
	switch s {
	case string(session.EncryptionLDAPS):
		return session.EncryptionLDAPS
	case string(session.EncryptionStartTLS):
		return session.EncryptionStartTLS
	default:
		return session.EncryptionNone
	}
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./app/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/connect.go app/connect_test.go
git commit -m "Connect through a saved profile, surfacing certificates as a decision"
```

---

## Task 7: Browsing and entry detail across the bridge

**Files:**
- Create: `app/browse.go`
- Test: `app/browse_test.go`

- [ ] **Step 1: Write the failing test**

```go
package app

import (
	"encoding/base64"
	"testing"
)

func TestCookieSurvivesJSON(t *testing.T) {
	// Paging cookies are opaque bytes, and some servers put values in them
	// that are not valid UTF-8. Base64 is what gets them through JSON intact.
	raw := []byte{0x00, 0xFF, 0x10, 0x80}

	encoded := encodeCookie(raw)
	if encoded != base64.StdEncoding.EncodeToString(raw) {
		t.Errorf("encodeCookie() = %q", encoded)
	}

	decoded, err := decodeCookie(encoded)
	if err != nil {
		t.Fatalf("decodeCookie() returned error: %v", err)
	}
	if string(decoded) != string(raw) {
		t.Errorf("decodeCookie() = %v, want the original bytes", decoded)
	}
}

func TestEmptyCookieRoundTrips(t *testing.T) {
	if got := encodeCookie(nil); got != "" {
		t.Errorf("encodeCookie(nil) = %q, want empty", got)
	}
	got, err := decodeCookie("")
	if err != nil {
		t.Fatalf("decodeCookie(\"\") returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("decodeCookie(\"\") = %v, want no bytes", got)
	}
}

func TestDecodeCookieRejectsGarbage(t *testing.T) {
	if _, err := decodeCookie("not base64!"); err == nil {
		t.Error("decodeCookie() accepted a value it could not decode")
	}
}

func TestChildrenWithoutAConnection(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	got := a.Children("nobody", "dc=example,dc=com", 0, "")
	if got.Error == "" {
		t.Error("Error is empty; asking an unopened connection for children must say so")
	}
}

func TestEntryWithoutAConnection(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	got := a.Entry("nobody", "dc=example,dc=com")
	if got.Error == "" {
		t.Error("Error is empty; asking an unopened connection for an entry must say so")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./app/ -run 'Cookie|Children|Entry' -v`
Expected: FAIL — `undefined: encodeCookie`.

- [ ] **Step 3: Implement it**

```go
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
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./app/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/browse.go app/browse_test.go
git commit -m "Expose paged browsing and entry detail to the interface"
```

---

## Task 8: Profiles and filters passed through

**Files:**
- Create: `app/library.go`
- Test: `app/library_test.go`

- [ ] **Step 1: Write the failing test**

```go
package app

import "testing"

func TestSaveAndListProfiles(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if got := a.ListProfiles(); len(got) != 0 {
		t.Errorf("a fresh install has %d profiles, want none", len(got))
	}

	if msg := a.SaveProfile(ProfileInput{
		ID: "dc01", Name: "CORP", Host: "dc01.example.com", Port: 636,
		Encryption: "ldaps", BindMethod: "ntlm", Username: `CORP\a.kensel`,
	}); msg != "" {
		t.Fatalf("SaveProfile() = %q, want no error", msg)
	}

	got := a.ListProfiles()
	if len(got) != 1 || got[0].ID != "dc01" {
		t.Fatalf("ListProfiles() = %+v, want the profile just saved", got)
	}
	if got[0].Connected {
		t.Error("a saved profile that was never opened is marked connected")
	}
}

func TestSaveProfileReportsWhatIsWrong(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if msg := a.SaveProfile(ProfileInput{ID: "dc01", Name: "CORP", Port: 636}); msg == "" {
		t.Error("SaveProfile() accepted a profile with no host")
	}
}

func TestListFiltersMarksWhatThisServerCannotRun(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	// With no connection open, nothing is known about the server, so no
	// filter is claimed to be supported.
	got := a.ListFilters("nobody")
	if len(got) != 18 {
		t.Fatalf("ListFilters() returned %d filters, want all 18 regardless of support", len(got))
	}

	var supported int
	for _, f := range got {
		if f.Supported {
			supported++
		}
	}
	if supported != 0 {
		t.Errorf("%d filters are marked supported with no connection open", supported)
	}
	for _, f := range got {
		if f.Reason == "" && f.Dialect != "generic" {
			t.Errorf("filter %q is unsupported with no reason given", f.ID)
		}
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./app/ -run 'Profile|Filters' -v`
Expected: FAIL — `undefined: ProfileInput`.

- [ ] **Step 3: Implement it**

```go
package app

import (
	"github.com/skensell201/ldapper/internal/filters"
	"github.com/skensell201/ldapper/internal/profiles"
)

// ProfileInput is what the connection form sends back.
type ProfileInput struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Encryption       string `json:"encryption"`
	BindMethod       string `json:"bindMethod"`
	Domain           string `json:"domain"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	RememberPassword bool   `json:"rememberPassword"`
}

// ProfileSummary is one row of the connection list.
type ProfileSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Encryption string `json:"encryption"`
	BindMethod string `json:"bindMethod"`
	Domain     string `json:"domain"`
	Username   string `json:"username"`
	Connected  bool   `json:"connected"`
}

// FilterSummary is one row of the filter list, with whether the connected
// server can actually answer it.
type FilterSummary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Filter      string   `json:"filter"`
	Scope       string   `json:"scope"`
	Base        string   `json:"base"`
	Columns     []string `json:"columns"`
	Dialect     string   `json:"dialect"`
	BuiltIn     bool     `json:"builtIn"`
	Modified    bool     `json:"modified"`
	// Supported is false when this server cannot answer the filter correctly.
	Supported bool `json:"supported"`
	// Reason explains why, for the tooltip on a greyed-out row.
	Reason string `json:"reason,omitempty"`
}

// ListProfiles returns every saved connection.
func (a *App) ListProfiles() []ProfileSummary {
	out := []ProfileSummary{}
	for _, p := range a.profiles.All() {
		_, connected := a.live(p.ID)
		out = append(out, ProfileSummary{
			ID:         p.ID,
			Name:       p.Name,
			Host:       p.Host,
			Port:       p.Port,
			Encryption: string(p.Encryption),
			BindMethod: string(p.BindMethod),
			Domain:     p.Domain,
			Username:   p.Username,
			Connected:  connected,
		})
	}
	return out
}

// SaveProfile stores a connection. It returns an empty string on success and a
// sentence to show otherwise.
func (a *App) SaveProfile(in ProfileInput) string {
	p := profiles.Profile{
		ID:               in.ID,
		Name:             in.Name,
		Host:             in.Host,
		Port:             in.Port,
		Encryption:       profiles.Encryption(in.Encryption),
		BindMethod:       profiles.BindMethod(in.BindMethod),
		Domain:           in.Domain,
		Username:         in.Username,
		RememberPassword: in.RememberPassword,
	}
	if existing, ok := a.profiles.Get(in.ID); ok {
		// Trust decisions belong to the server, not to the form.
		p.TrustedFingerprints = existing.TrustedFingerprints
	}

	if err := a.profiles.Save(p.WithPassword(in.Password)); err != nil {
		return err.Error()
	}
	return ""
}

// DeleteProfile removes a connection and forgets its password.
func (a *App) DeleteProfile(id string) string {
	a.dropLive(id)
	if err := a.profiles.Delete(id); err != nil {
		return err.Error()
	}
	return ""
}

// ListFilters returns the whole library, each entry marked with whether the
// connected server can answer it. Filters are never hidden — a greyed row with
// a reason tells the user something; a missing row tells them nothing.
func (a *App) ListFilters(profileID string) []FilterSummary {
	var supported []filters.Dialect
	if c, ok := a.live(profileID); ok {
		supported = c.info.Dialects()
	}

	out := []FilterSummary{}
	for _, f := range a.filters.All() {
		s := FilterSummary{
			ID:          f.ID,
			Name:        f.Name,
			Description: f.Description,
			Filter:      f.Filter,
			Scope:       string(f.Scope),
			Base:        f.Base,
			Columns:     f.Columns,
			Dialect:     string(f.Dialect),
			BuiltIn:     f.BuiltIn,
			Modified:    f.Modified,
			Supported:   filters.Compatible(f, supported),
		}
		if !s.Supported {
			s.Reason = filters.IncompatibleReason(f)
			if s.Reason == "" {
				s.Reason = "Connect to a directory to run this filter."
			}
		}
		out = append(out, s)
	}
	return out
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./app/ -v`
Expected: PASS.

- [ ] **Step 5: Generate the TypeScript bindings and check them**

```bash
wails generate module
```

Expected: `frontend/wailsjs/go/app/App.d.ts` appears, declaring `Connect`, `Children`, `Entry`, `ListProfiles`, `SaveProfile` and `ListFilters` with the DTO types.

- [ ] **Step 6: Commit**

```bash
git add app/library.go app/library_test.go
git commit -m "Pass profiles and the filter library through to the interface"
```

---

## Task 9: Design tokens and the window shell

**Files:**
- Create: `frontend/src/tokens.css`, `frontend/src/App.tsx`, `frontend/src/App.css`
- Modify: `frontend/src/main.tsx`, `frontend/index.html`

- [ ] **Step 1: Write the tokens**

These are lifted from `docs/design/mockup-v1-superlist.html` unchanged, so a design review of the mockup still means something.

```css
/* frontend/src/tokens.css */
:root {
  color-scheme: dark;

  --canvas: #181824;
  --plum:   #26253b;
  --well:   #1f1e30;
  --deep:   #131320;
  --raise:  #2f2e47;

  --coral: #ff4a36;
  --iris:  #535676;

  --white: #ffffff;
  --mist:  #a3a3a7;
  --ash:   #8e8da0;
  --fog:   #696f81;
  --hairline: rgba(247, 247, 255, 0.09);
  --hairline-soft: rgba(247, 247, 255, 0.05);

  --sans: "Inter Tight", Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
  --mono: ui-monospace, "SF Mono", "Cascadia Mono", Menlo, Consolas, monospace;

  --r-card: 20px;
  --r-pane: 16px;
  --r-pill: 100px;
  --r-input: 8px;

  --sh-md: rgba(0, 0, 0, 0.34) 0 18px 42px -12px, rgba(0, 0, 0, 0.22) 0 6px 14px -6px;
  --sh-sm: rgba(0, 0, 0, 0.24) 0 4px 12px -4px;
  --sh-modal: rgba(0, 0, 0, 0.5) 0 28px 70px -18px;

  /* The tree and the table are virtualised, so every row is exactly this
     tall. Changing it here changes both. */
  --row-height: 28px;
}

* { box-sizing: border-box; }

html, body, #root { height: 100%; margin: 0; }

body {
  background: var(--canvas);
  color: var(--white);
  font-family: var(--sans);
  font-size: 13px;
  line-height: 1.4;
  letter-spacing: -0.26px;
  -webkit-font-smoothing: antialiased;
  overflow: hidden;
}

/* Wails' frameless window needs an explicit drag region. */
.drag { --wails-draggable: drag; }
.no-drag { --wails-draggable: no-drag; }

button { font: inherit; color: inherit; background: none; border: 0; cursor: pointer; }
button:focus-visible, [tabindex]:focus-visible { outline: 2px solid var(--coral); outline-offset: 2px; }

@media (prefers-reduced-motion: reduce) {
  * { animation-duration: 0.01ms !important; transition-duration: 0.01ms !important; }
}
```

- [ ] **Step 2: Write the window shell**

`App.tsx` renders the utility bar, the two panes and the status bar, and nothing else — the panes and dialogs are their own files.

```tsx
// frontend/src/App.tsx
import { useEffect } from "react";
import { Tree } from "./panes/Tree";
import { Detail } from "./panes/Detail";
import { Connect } from "./dialogs/Connect";
import { Certificate } from "./dialogs/Certificate";
import { useStore } from "./store";
import "./App.css";

export function App() {
  const state = useStore((s) => s.connection);
  const dialog = useStore((s) => s.dialog);
  const openConnect = useStore((s) => s.openConnect);
  const loadProfiles = useStore((s) => s.loadProfiles);

  useEffect(() => {
    void loadProfiles();
  }, [loadProfiles]);

  return (
    <div className="window">
      <header className="utility drag">
        <span className="wordmark">Ldapper</span>
        <button className="conn no-drag" onClick={openConnect}>
          <i className={state?.connected ? "led live" : "led"} />
          {state?.connected ? `${state.host} · ${state.boundAs || "anonymous"}` : "not connected"}
          <span className="chev">▾</span>
        </button>
        <span className="spacer" />
        <button className="util no-drag" onClick={openConnect}>Connections</button>
      </header>

      <main className="workspace">
        <Tree />
        <Detail />
      </main>

      <footer className="statusbar">
        {state?.connected ? (
          <>
            <span className="live">Connected</span>
            <span>{state.encryption === "none" ? "unencrypted" : state.encryption.toUpperCase()}</span>
            <span>{state.isActiveDirectory ? "Active Directory" : "LDAP"}</span>
            <span>{state.supportsPaging ? "paged results" : "no paging control"}</span>
            <span className="spacer" />
            <span>{state.rootDN}</span>
          </>
        ) : (
          <span>idle</span>
        )}
      </footer>

      {dialog === "connect" && <Connect />}
      {dialog === "certificate" && <Certificate />}
    </div>
  );
}
```

- [ ] **Step 3: Write App.css**

```css
/* frontend/src/App.css */
.window { display: grid; grid-template-rows: 44px 1fr 34px; height: 100%; background: var(--plum); }

.utility { display: flex; align-items: center; gap: 14px; padding: 0 18px 0 82px; }
.wordmark { font-size: 14px; font-weight: 600; letter-spacing: -0.02em; }
.conn {
  display: flex; align-items: center; gap: 9px;
  height: 28px; padding: 0 14px;
  border-radius: var(--r-pill); background: var(--well);
  font-family: var(--mono); font-size: 11px; letter-spacing: 0; color: var(--ash);
}
.led { width: 6px; height: 6px; border-radius: 50%; background: var(--iris); }
.led.live { background: var(--white); }
.chev { color: var(--iris); font-size: 9px; }
.util { font-size: 13px; color: var(--fog); }
.util:hover { color: var(--white); }
.spacer { margin-left: auto; }

.workspace { display: grid; grid-template-columns: 320px 1fr; gap: 10px; padding: 0 10px 10px; min-height: 0; }
.pane { background: var(--well); border-radius: var(--r-pane); overflow: hidden; min-width: 0; display: flex; flex-direction: column; }
.pane-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 18px 10px;
  font-size: 11px; letter-spacing: 0.1em; text-transform: uppercase; color: var(--iris);
  flex: none;
}
.pane-head .n { font-family: var(--mono); text-transform: none; letter-spacing: 0; color: var(--fog); }

.statusbar {
  display: flex; align-items: center; gap: 20px; padding: 0 22px;
  background: var(--deep);
  font-family: var(--mono); font-size: 10.5px; letter-spacing: 0; color: var(--iris);
}
.statusbar .live { color: var(--mist); }

.empty { display: grid; place-items: center; height: 100%; color: var(--iris); font-family: var(--mono); font-size: 11px; }
.error { margin: 10px 18px; padding: 10px 14px; border-radius: var(--r-input); background: rgba(255, 74, 54, 0.12); color: var(--coral); font-size: 12px; }
```

- [ ] **Step 4: Point main.tsx at it**

```tsx
// frontend/src/main.tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./App";
import "./tokens.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

Delete the Vite template's `index.css` and `assets/react.svg`, and set the page title in `index.html` to `Ldapper`.

- [ ] **Step 5: Commit**

```bash
git add frontend/src frontend/index.html
git commit -m "Port the Superlist tokens and lay out the window shell"
```

---

## Task 10: The store

**Files:**
- Create: `frontend/src/store.ts`

- [ ] **Step 1: Write it**

```ts
// frontend/src/store.ts
import { create } from "zustand";
import * as api from "../wailsjs/go/app/App";
import type { app } from "../wailsjs/go/models";

/** A node plus what the tree knows about it: whether it is open, how deep it
 *  sits, and how much of it has been loaded. */
export interface TreeNode {
  node: app.Node;
  depth: number;
  expanded: boolean;
  /** loading is set while a page is in flight, so the row can say so. */
  loading: boolean;
  /** cookie is the cursor for the next page; empty once fully loaded. */
  cookie: string;
  /** page counts the pages fetched so far. */
  page: number;
  children: string[] | null;
}

interface State {
  connection: app.ConnectionState | null;
  profiles: app.ProfileSummary[];
  dialog: "connect" | "certificate" | null;
  certificate: app.CertPrompt | null;
  pendingProfileID: string | null;
  error: string;

  /** nodes is keyed by DN. A flat map is what keeps expanding a node from
   *  re-rendering its siblings. */
  nodes: Record<string, TreeNode>;
  roots: string[];
  selected: string | null;
  detail: app.EntryDetail | null;
  detailError: string;

  loadProfiles: () => Promise<void>;
  openConnect: () => void;
  closeDialog: () => void;
  connect: (profileID: string, password: string) => Promise<void>;
  trustAndRetry: () => Promise<void>;
  toggle: (dn: string) => Promise<void>;
  loadMore: (dn: string) => Promise<void>;
  select: (dn: string) => Promise<void>;
}

const PAGE_SIZE = 200;

export const useStore = create<State>((set, get) => ({
  connection: null,
  profiles: [],
  dialog: null,
  certificate: null,
  pendingProfileID: null,
  error: "",
  nodes: {},
  roots: [],
  selected: null,
  detail: null,
  detailError: "",

  loadProfiles: async () => {
    set({ profiles: await api.ListProfiles() });
  },

  openConnect: () => set({ dialog: "connect", error: "" }),
  closeDialog: () => set({ dialog: null, certificate: null }),

  connect: async (profileID, password) => {
    const result = await api.Connect(profileID, password);

    if (result.certificate) {
      set({ dialog: "certificate", certificate: result.certificate, pendingProfileID: profileID });
      return;
    }
    if (result.error) {
      set({ error: result.error });
      return;
    }

    set({
      connection: result.state,
      dialog: null,
      certificate: null,
      error: "",
      nodes: {},
      roots: [],
      selected: null,
      detail: null,
    });
    await get().loadProfiles();

    // Open the root branch so the window is never empty after connecting.
    const rootDN = result.state.rootDN;
    if (rootDN) {
      set((s) => ({
        nodes: {
          ...s.nodes,
          [rootDN]: {
            node: { dn: rootDN, label: rootDN, rdn: rootDN, icon: "domain", childCount: -1, hasChildren: true },
            depth: 0,
            expanded: false,
            loading: false,
            cookie: "",
            page: 0,
            children: null,
          },
        },
        roots: [rootDN],
      }));
      await get().toggle(rootDN);
    }
  },

  trustAndRetry: async () => {
    const { certificate, pendingProfileID } = get();
    if (!certificate || !pendingProfileID) return;

    const err = await api.TrustCertificate(pendingProfileID, certificate.fingerprint);
    if (err) {
      set({ error: err, dialog: "connect" });
      return;
    }
    set({ certificate: null, dialog: null });
    await get().connect(pendingProfileID, "");
  },

  toggle: async (dn) => {
    const entry = get().nodes[dn];
    if (!entry) return;

    if (entry.expanded) {
      set((s) => ({ nodes: { ...s.nodes, [dn]: { ...s.nodes[dn], expanded: false } } }));
      return;
    }
    if (entry.children !== null) {
      set((s) => ({ nodes: { ...s.nodes, [dn]: { ...s.nodes[dn], expanded: true } } }));
      return;
    }
    await get().loadMore(dn);
  },

  loadMore: async (dn) => {
    const { connection, nodes } = get();
    const entry = nodes[dn];
    if (!connection || !entry || entry.loading) return;

    set((s) => ({ nodes: { ...s.nodes, [dn]: { ...s.nodes[dn], loading: true, expanded: true } } }));

    const result = await api.Children(connection.profileId, dn, PAGE_SIZE, entry.cookie);

    if (result.error) {
      set((s) => ({
        error: result.error,
        nodes: { ...s.nodes, [dn]: { ...s.nodes[dn], loading: false } },
      }));
      return;
    }

    set((s) => {
      const parent = s.nodes[dn];
      const added: Record<string, TreeNode> = {};
      for (const node of result.nodes ?? []) {
        added[node.dn] = {
          node,
          depth: parent.depth + 1,
          expanded: false,
          loading: false,
          cookie: "",
          page: 0,
          children: null,
        };
      }
      return {
        nodes: {
          ...s.nodes,
          ...added,
          [dn]: {
            ...parent,
            loading: false,
            expanded: true,
            cookie: result.cookie,
            page: parent.page + 1,
            children: [...(parent.children ?? []), ...(result.nodes ?? []).map((n) => n.dn)],
          },
        },
      };
    });
  },

  select: async (dn) => {
    const { connection } = get();
    set({ selected: dn, detail: null, detailError: "" });
    if (!connection) return;

    const result = await api.Entry(connection.profileId, dn);
    // Ignore a result that arrived after the user moved on.
    if (get().selected !== dn) return;

    if (result.error) {
      set({ detailError: result.error });
      return;
    }
    set({ detail: result.detail });
  },
}));

/** visibleRows flattens the tree into exactly the rows on screen. The
 *  virtualiser needs a list, and computing it here keeps the component from
 *  walking the whole directory on every render. */
export function visibleRows(state: State): TreeNode[] {
  const out: TreeNode[] = [];

  const walk = (dn: string) => {
    const entry = state.nodes[dn];
    if (!entry) return;
    out.push(entry);
    if (!entry.expanded || !entry.children) return;
    for (const child of entry.children) walk(child);
  };

  for (const root of state.roots) walk(root);
  return out;
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/store.ts
git commit -m "Add the tree cache and connection state store"
```

---

## Task 11: The tree pane

**Files:**
- Create: `frontend/src/panes/Tree.tsx`, `frontend/src/panes/Tree.css`, `frontend/src/Icon.tsx`

- [ ] **Step 1: Write the icon component**

One inline SVG sprite, matching the names `iconFor` returns in Task 3.

```tsx
// frontend/src/Icon.tsx
const paths: Record<string, JSX.Element> = {
  domain: (
    <>
      <circle cx="8" cy="8" r="5.8" />
      <path d="M2.2 8h11.6M8 2.2c2.4 2.7 2.4 9.1 0 11.6M8 2.2C5.6 4.9 5.6 11.3 8 13.8" />
    </>
  ),
  ou: <path d="M2.2 12.8V4.2h3.9l1.3 1.7h6.4v6.9z" />,
  container: (
    <>
      <rect x="2.4" y="3.6" width="11.2" height="8.8" rx="1.6" />
      <path d="M2.4 6.6h11.2" />
    </>
  ),
  user: (
    <>
      <circle cx="8" cy="5.4" r="2.6" />
      <path d="M2.8 13.6c0-2.9 2.3-4.4 5.2-4.4s5.2 1.5 5.2 4.4" />
    </>
  ),
  group: (
    <>
      <circle cx="6.1" cy="5.6" r="2.3" />
      <path d="M1.6 13.4c0-2.6 2-4 4.5-4s4.5 1.4 4.5 4" />
      <path d="M10.6 3.6a2.3 2.3 0 0 1 0 4.4M11.6 9.7c1.7.4 2.8 1.6 2.8 3.7" />
    </>
  ),
  computer: (
    <>
      <rect x="2.2" y="3.4" width="11.6" height="7.4" rx="1.4" />
      <path d="M5.6 13.2h4.8" />
    </>
  ),
  object: <circle cx="8" cy="8" r="4.4" />,
};

export function Icon({ name, className }: { name: string; className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.2"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name] ?? paths.object}
    </svg>
  );
}
```

- [ ] **Step 2: Write the tree**

```tsx
// frontend/src/panes/Tree.tsx
import { useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useStore, visibleRows } from "../store";
import { Icon } from "../Icon";
import "./Tree.css";

export function Tree() {
  const rows = useStore(visibleRows);
  const selected = useStore((s) => s.selected);
  const connected = useStore((s) => s.connection?.connected ?? false);
  const toggle = useStore((s) => s.toggle);
  const loadMore = useStore((s) => s.loadMore);
  const select = useStore((s) => s.select);

  const scrollRef = useRef<HTMLDivElement>(null);

  // Virtualising from the first commit is not premature: an organizational
  // unit holding fifty thousand accounts is ordinary, and rendering that many
  // rows is not survivable.
  const virtual = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 28,
    overscan: 12,
  });

  if (!connected) {
    return (
      <aside className="pane">
        <div className="pane-head"><span>Directory</span></div>
        <div className="empty">not connected</div>
      </aside>
    );
  }

  return (
    <aside className="pane">
      <div className="pane-head">
        <span>Directory</span>
        <span className="n">{rows.length}</span>
      </div>

      <div className="tree-scroll" ref={scrollRef}>
        <div className="tree-inner" style={{ height: virtual.getTotalSize() }}>
          {virtual.getVirtualItems().map((item) => {
            const row = rows[item.index];
            const { node } = row;
            const isSelected = selected === node.dn;

            return (
              <div
                key={node.dn}
                className={isSelected ? "row selected" : "row"}
                style={{
                  transform: `translateY(${item.start}px)`,
                  paddingLeft: 10 + row.depth * 15,
                }}
                onClick={() => void select(node.dn)}
                onDoubleClick={() => void toggle(node.dn)}
                role="treeitem"
                aria-selected={isSelected}
                aria-expanded={node.hasChildren ? row.expanded : undefined}
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") { e.preventDefault(); void select(node.dn); }
                  if (e.key === "ArrowRight" && !row.expanded) void toggle(node.dn);
                  if (e.key === "ArrowLeft" && row.expanded) void toggle(node.dn);
                }}
                title={node.dn}
              >
                <button
                  className="chev"
                  onClick={(e) => { e.stopPropagation(); void toggle(node.dn); }}
                  tabIndex={-1}
                  aria-hidden={!node.hasChildren}
                >
                  {node.hasChildren ? (row.expanded ? "▾" : "▸") : ""}
                </button>

                <Icon name={node.icon} className="row-icon" />
                <span className="label">{node.label}</span>

                {row.loading && <span className="count">loading…</span>}
                {!row.loading && row.cookie && (
                  <button
                    className="more"
                    onClick={(e) => { e.stopPropagation(); void loadMore(node.dn); }}
                  >
                    page {row.page + 1}
                  </button>
                )}
                {!row.loading && !row.cookie && node.childCount >= 0 && (
                  <span className="count">{node.childCount}</span>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </aside>
  );
}
```

- [ ] **Step 3: Write Tree.css**

```css
/* frontend/src/panes/Tree.css */
.tree-scroll { flex: 1; overflow: auto; padding: 0 8px 10px; min-height: 0; }
.tree-inner { position: relative; width: 100%; }

.row {
  position: absolute; top: 0; left: 0; right: 0;
  display: flex; align-items: center; gap: 8px;
  height: var(--row-height);
  padding-right: 10px;
  border-radius: var(--r-input);
  color: var(--ash);
  white-space: nowrap;
  cursor: default;
}
.row:hover { background: var(--raise); }
.row.selected { background: var(--coral); color: var(--white); font-weight: 500; }
.row.selected .row-icon, .row.selected .chev, .row.selected .count { color: rgba(255, 255, 255, 0.8); }

.chev { width: 10px; flex: none; color: var(--iris); font-size: 8px; text-align: center; padding: 0; }
.row-icon { width: 14px; height: 14px; flex: none; color: var(--iris); }
.label { overflow: hidden; text-overflow: ellipsis; }
.count { margin-left: auto; font-family: var(--mono); font-size: 10.5px; letter-spacing: 0; color: var(--iris); }

.more {
  margin-left: auto;
  font-family: var(--mono); font-size: 10.5px; letter-spacing: 0;
  color: var(--mist);
  padding: 1px 8px; border-radius: var(--r-pill); background: var(--raise);
}
.more:hover { color: var(--white); }
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/panes/Tree.tsx frontend/src/panes/Tree.css frontend/src/Icon.tsx
git commit -m "Add the virtualised directory tree with visible paging"
```

---

## Task 12: The attribute table

**Files:**
- Create: `frontend/src/panes/Detail.tsx`, `frontend/src/panes/Detail.css`

- [ ] **Step 1: Write it**

```tsx
// frontend/src/panes/Detail.tsx
import { useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useStore } from "../store";
import { Icon } from "../Icon";
import "./Detail.css";

/** A row of the table is one value, not one attribute: an attribute with
 *  seven values needs seven rows for the virtualiser to measure. */
interface ValueRow {
  attribute: string;
  first: boolean;
  raw: string;
  decoded: string[];
}

export function Detail() {
  const detail = useStore((s) => s.detail);
  const error = useStore((s) => s.detailError);
  const selected = useStore((s) => s.selected);

  const rows = useMemo<ValueRow[]>(() => {
    if (!detail) return [];
    const out: ValueRow[] = [];
    for (const row of detail.rows ?? []) {
      for (const [i, value] of (row.values ?? []).entries()) {
        out.push({ attribute: row.name, first: i === 0, raw: value.raw, decoded: value.decoded ?? [] });
      }
    }
    return out;
  }, [detail]);

  const scrollRef = useRef<HTMLDivElement>(null);
  const virtual = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    // A decoded value adds a second line, so rows are not all one height.
    estimateSize: (i) => (rows[i]?.decoded.length ? 52 : 32),
    overscan: 10,
  });

  if (error) {
    return (
      <section className="pane">
        <div className="error">{error}</div>
      </section>
    );
  }
  if (!selected) {
    return (
      <section className="pane">
        <div className="empty">select an object</div>
      </section>
    );
  }
  if (!detail) {
    return (
      <section className="pane">
        <div className="empty">loading…</div>
      </section>
    );
  }

  return (
    <section className="pane">
      <header className="detail-head">
        <span className="avatar"><Icon name={detail.icon} /></span>
        <span className="detail-title">
          <span className="cn">{detail.rdn.includes("=") ? detail.rdn.split("=").slice(1).join("=") : detail.rdn}</span>
          <span className="dn" title={detail.dn}>{detail.dn}</span>
        </span>
        <button
          className="ghost"
          onClick={() => void navigator.clipboard.writeText(detail.dn)}
        >
          Copy DN
        </button>
      </header>

      <div className="attr-head">
        <span>Attribute</span>
        <span>Value</span>
      </div>

      <div className="attr-scroll" ref={scrollRef}>
        <div className="attr-inner" style={{ height: virtual.getTotalSize() }}>
          {virtual.getVirtualItems().map((item) => {
            const row = rows[item.index];
            return (
              <div
                className="attr-row"
                key={`${row.attribute}-${item.index}`}
                ref={virtual.measureElement}
                data-index={item.index}
                style={{ transform: `translateY(${item.start}px)` }}
              >
                <span className="attr-name">{row.first ? row.attribute : ""}</span>
                <span className="attr-value">
                  <span className={row.decoded.length ? "raw" : "val"}>{row.raw}</span>
                  {row.decoded.length > 0 && (
                    <span className="decoded-list">
                      {row.decoded.map((d) => (
                        <span className="decoded" key={d}>{d}</span>
                      ))}
                    </span>
                  )}
                </span>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
```

- [ ] **Step 2: Write Detail.css**

```css
/* frontend/src/panes/Detail.css */
.detail-head { display: flex; align-items: flex-start; gap: 14px; padding: 20px 22px 16px; flex: none; }
.avatar {
  width: 38px; height: 38px; flex: none; border-radius: 12px;
  background: var(--raise); display: grid; place-items: center; color: var(--mist);
}
.avatar svg { width: 18px; height: 18px; }
.detail-title { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.detail-title .cn { font-size: 17px; font-weight: 600; letter-spacing: -0.34px; color: var(--white); }
.detail-title .dn {
  font-family: var(--mono); font-size: 11px; letter-spacing: 0; color: var(--fog);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.detail-head .ghost {
  margin-left: auto; flex: none;
  height: 30px; padding: 0 16px; border-radius: var(--r-pill);
  background: var(--raise); color: var(--ash); font-size: 12px;
}
.detail-head .ghost:hover { color: var(--white); }

.attr-head {
  display: grid; grid-template-columns: 220px 1fr; gap: 16px;
  padding: 0 22px 10px;
  font-size: 11px; letter-spacing: 0.1em; text-transform: uppercase; color: var(--iris);
  box-shadow: inset 0 -1px 0 var(--hairline);
  flex: none;
}

.attr-scroll { flex: 1; overflow: auto; min-height: 0; }
.attr-inner { position: relative; width: 100%; }

.attr-row {
  position: absolute; top: 0; left: 0; right: 0;
  display: grid; grid-template-columns: 220px 1fr; gap: 16px;
  padding: 8px 22px;
  box-shadow: inset 0 -1px 0 var(--hairline-soft);
}
.attr-name { font-family: var(--mono); font-size: 12px; letter-spacing: 0; color: var(--mist); }
.attr-value { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
.val { font-family: var(--mono); font-size: 11.5px; letter-spacing: 0; color: var(--white); word-break: break-all; }
.raw { font-family: var(--mono); font-size: 11.5px; letter-spacing: 0; color: var(--fog); word-break: break-all; }

.decoded-list { display: flex; flex-wrap: wrap; gap: 6px; }
.decoded {
  padding: 2px 11px; border-radius: var(--r-pill);
  background: var(--raise); color: var(--white);
  font-family: var(--mono); font-size: 11px; letter-spacing: 0;
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/panes/Detail.tsx frontend/src/panes/Detail.css
git commit -m "Add the attribute table, decoded values under raw ones"
```

---

## Task 13: The connect and certificate dialogs

**Files:**
- Create: `frontend/src/dialogs/Connect.tsx`, `frontend/src/dialogs/Certificate.tsx`, `frontend/src/dialogs/Dialog.css`

- [ ] **Step 1: Write the shared dialog styles**

```css
/* frontend/src/dialogs/Dialog.css */
.scrim { position: fixed; inset: 0; background: rgba(19, 19, 32, 0.72); display: grid; place-items: center; padding: 24px; z-index: 10; }

.modal {
  width: 460px; max-width: 100%; max-height: 100%; overflow: auto;
  background: var(--plum); border-radius: var(--r-card); box-shadow: var(--sh-modal);
  padding: 30px; display: flex; flex-direction: column; gap: 20px;
}
.modal h3 { margin: 0; font-size: 24px; font-weight: 600; letter-spacing: -0.48px; line-height: 1.15; }
.modal .sub { margin: -12px 0 0; font-size: 13px; color: var(--fog); line-height: 1.45; }

.form { display: flex; flex-direction: column; gap: 14px; }
.lbl { font-size: 11px; letter-spacing: 0.1em; text-transform: uppercase; color: var(--iris); margin-bottom: 6px; display: block; }
.cols { display: grid; grid-template-columns: 1fr 96px; gap: 10px; }

.field {
  display: flex; align-items: center; width: 100%;
  height: 36px; padding: 0 14px;
  background: var(--well); border: 0; border-radius: var(--r-input);
  color: var(--white); font-family: var(--mono); font-size: 11.5px; letter-spacing: 0;
}
.field::placeholder { color: var(--iris); }

.segs { display: flex; gap: 4px; padding: 4px; background: var(--well); border-radius: var(--r-pill); }
.seg { flex: 1; text-align: center; padding: 6px 0; border-radius: var(--r-pill); font-size: 12px; color: var(--fog); }
.seg.on { background: var(--raise); color: var(--white); }

.check { display: flex; align-items: center; gap: 10px; font-size: 12.5px; color: var(--ash); }

.acts { display: flex; align-items: center; gap: 10px; }
.acts .spacer { margin-left: auto; }
.btn { height: 36px; padding: 0 20px; border-radius: var(--r-pill); font-size: 12.5px; font-weight: 500; }
.btn.plain { color: var(--fog); }
.btn.plain:hover { color: var(--white); }
.btn.light { background: var(--white); color: var(--canvas); font-weight: 600; }
.btn.cta { background: var(--coral); color: var(--white); font-weight: 600; box-shadow: rgba(255, 74, 54, 0.28) 0 6px 18px -6px; }
.btn:disabled { opacity: 0.5; cursor: default; }

.cert { background: var(--well); border-radius: 14px; padding: 16px 18px; display: flex; flex-direction: column; gap: 12px; }
.cert-head { display: flex; align-items: center; gap: 9px; font-size: 12.5px; font-weight: 500; color: var(--coral); }
.cert dl { margin: 0; display: grid; grid-template-columns: 96px 1fr; gap: 6px 12px; font-size: 11px; }
.cert dt { color: var(--iris); font-family: var(--mono); letter-spacing: 0; }
.cert dd { margin: 0; color: var(--ash); font-family: var(--mono); letter-spacing: 0; word-break: break-all; }
.cert dd.fp { color: var(--white); }
```

- [ ] **Step 2: Write the connect dialog**

```tsx
// frontend/src/dialogs/Connect.tsx
import { useState } from "react";
import { useStore } from "../store";
import * as api from "../../wailsjs/go/app/App";
import "./Dialog.css";

export function Connect() {
  const profiles = useStore((s) => s.profiles);
  const error = useStore((s) => s.error);
  const close = useStore((s) => s.closeDialog);
  const connect = useStore((s) => s.connect);
  const loadProfiles = useStore((s) => s.loadProfiles);

  const existing = profiles[0];
  const [id, setID] = useState(existing?.id ?? "dc01");
  const [host, setHost] = useState(existing?.host ?? "");
  const [port, setPort] = useState(String(existing?.port ?? 636));
  const [encryption, setEncryption] = useState(existing?.encryption ?? "ldaps");
  const [bindMethod, setBindMethod] = useState(existing?.bindMethod ?? "ntlm");
  const [username, setUsername] = useState(existing?.username ?? "");
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(false);
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    setBusy(true);
    try {
      const msg = await api.SaveProfile({
        id, name: id, host, port: Number(port) || 0,
        encryption, bindMethod, domain: "", username, password,
        rememberPassword: remember,
      });
      if (msg) {
        useStore.setState({ error: msg });
        return;
      }
      await loadProfiles();
      await connect(id, password);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="scrim" onClick={close}>
      <div className="modal no-drag" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <h3>Connect to a directory</h3>
        <p className="sub">Saved on this machine. The password goes to the system keychain, never to the settings file.</p>

        {error && <div className="error">{error}</div>}

        <div className="form">
          <div className="cols">
            <div>
              <span className="lbl">Host</span>
              <input className="field" value={host} onChange={(e) => setHost(e.target.value)} placeholder="dc01.corp.example.com" autoFocus />
            </div>
            <div>
              <span className="lbl">Port</span>
              <input className="field" value={port} onChange={(e) => setPort(e.target.value)} inputMode="numeric" />
            </div>
          </div>

          <div>
            <span className="lbl">Encryption</span>
            <div className="segs">
              {["ldaps", "starttls", "none"].map((v) => (
                <button key={v} className={encryption === v ? "seg on" : "seg"} onClick={() => setEncryption(v)}>
                  {v === "ldaps" ? "LDAPS" : v === "starttls" ? "StartTLS" : "None"}
                </button>
              ))}
            </div>
          </div>

          <div>
            <span className="lbl">Authentication</span>
            <div className="segs">
              {["ntlm", "simple"].map((v) => (
                <button key={v} className={bindMethod === v ? "seg on" : "seg"} onClick={() => setBindMethod(v)}>
                  {v === "ntlm" ? "NTLM" : "Simple bind"}
                </button>
              ))}
            </div>
          </div>

          <div>
            <span className="lbl">User</span>
            <input
              className="field"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder={bindMethod === "ntlm" ? "CORP\\a.kensel" : "cn=admin,dc=example,dc=com"}
            />
          </div>

          <div>
            <span className="lbl">Password</span>
            <input className="field" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </div>

          <label className="check">
            <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} />
            Remember the password in this system's keychain
          </label>

          <label className="check">
            <input type="text" className="field" value={id} onChange={(e) => setID(e.target.value)} aria-label="Connection name" />
          </label>
        </div>

        <div className="acts">
          <span className="spacer" />
          <button className="btn plain" onClick={close}>Cancel</button>
          <button className="btn cta" onClick={() => void submit()} disabled={busy || !host}>
            {busy ? "Connecting…" : "Connect"}
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Write the certificate dialog**

The confirming button is deliberately not the accent colour. Continuing past a
warning should not look like the recommended action.

```tsx
// frontend/src/dialogs/Certificate.tsx
import { useStore } from "../store";
import "./Dialog.css";

export function Certificate() {
  const cert = useStore((s) => s.certificate);
  const close = useStore((s) => s.closeDialog);
  const trust = useStore((s) => s.trustAndRetry);

  if (!cert) return null;

  return (
    <div className="scrim">
      <div className="modal no-drag" style={{ width: 500 }} role="dialog" aria-modal="true">
        <h3>This server's certificate isn't trusted</h3>
        <p className="sub">
          {cert.subject} signed its certificate with an authority this machine doesn't know.
          That is normal for an internal CA — and it also looks exactly like an interception.
        </p>

        <div className="cert">
          <div className="cert-head">Verify the fingerprint before you continue</div>
          <dl>
            <dt>SHA-256</dt><dd className="fp">{cert.fingerprint}</dd>
            <dt>Subject</dt><dd>{cert.subject}</dd>
            <dt>Issuer</dt><dd>{cert.issuer}</dd>
            <dt>Valid until</dt><dd>{cert.notAfter}</dd>
          </dl>
        </div>

        <div className="acts">
          <span className="spacer" />
          <button className="btn plain" onClick={close}>Cancel</button>
          <button className="btn light" onClick={() => void trust()}>Continue</button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Build the frontend and commit**

```bash
cd frontend && npm run build
```

Expected: a clean TypeScript build. Fix any type errors the bindings surface —
they are the contract from Task 8 doing its job.

```bash
git add frontend/src/dialogs
git commit -m "Add the connect and certificate dialogs"
```

---

## Task 14: End to end against a real directory

- [ ] **Step 1: Build the application**

```bash
wails build
```

Expected: a binary under `build/bin/`. On macOS that is `build/bin/Ldapper.app`.

- [ ] **Step 2: Start the test directory**

```bash
docker compose -f test/integration/docker-compose.yml up -d --wait
```

- [ ] **Step 3: Run it and connect**

Open the application and connect with:

| Field | Value |
|---|---|
| Host | `localhost` |
| Port | `3389` |
| Encryption | None |
| Authentication | Simple bind |
| User | `cn=admin,dc=example,dc=com` |
| Password | `adminpassword` |

Verify by eye:

- [ ] The status bar reads Connected, unencrypted, LDAP, paged results, and shows `dc=example,dc=com`
- [ ] The tree opens at the root with `ou=people` and `ou=groups` beneath it
- [ ] Expanding `ou=people` loads a page and offers "page 2" rather than freezing
- [ ] Selecting a person fills the attribute table, `mail` among them
- [ ] Scrolling `ou=people` stays smooth — that is the virtualiser earning its place

- [ ] **Step 4: Tear down and commit**

```bash
docker compose -f test/integration/docker-compose.yml down -v
git add -A && git commit -m "Wire the shell end to end"
```

---

## Definition of done

- [ ] `go test -race ./...` passes, including the new `app` and `config` packages
- [ ] `golangci-lint run` reports nothing
- [ ] `cd frontend && npm run build` completes with no TypeScript errors
- [ ] `wails build` produces a binary
- [ ] The manual checks in Task 14 all pass against the Docker directory
- [ ] CI is green

---

## Self-review against the spec

| Spec section | Covered by |
|---|---|
| 4, screen 1 (tree and attributes) | Tasks 11, 12 — both virtualised, paging visible as a page number rather than a spinner |
| 4, screen 4 (connection) | Task 13 |
| 4, screen 5 (untrusted certificate) | Task 13, with the confirming button deliberately not the accent colour |
| 4, tokens | Task 9, lifted from the mockup unchanged |
| 3.1, reconnect after a dropped connection | **Deferred to plan 3.** It needs the status bar to show a reconnect in progress, which needs the event bridge that plan 3 builds for streaming search. |
| 3.4, attribute syntaxes from `attributeTypes` | **Deferred to plan 3.** It only changes how a value is presented, and nothing in this plan reads it. |

Screens 2 and 3 of the mockup — search and the filter library — are plan 3 by
design. `app.ListFilters` is built here anyway, because the connect flow
already needs the dialect information it returns.
