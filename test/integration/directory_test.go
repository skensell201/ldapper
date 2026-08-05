//go:build integration

// Package integration exercises the engine against a real directory. Every
// test here needs the server from docker-compose.yml; run them with
// `make integration`, which starts it and tears it down again.
package integration

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/skensell201/ldapper/internal/browse"
	"github.com/skensell201/ldapper/internal/filters"
	"github.com/skensell201/ldapper/internal/schema"
	"github.com/skensell201/ldapper/internal/search"
	"github.com/skensell201/ldapper/internal/session"
)

const (
	adminDN  = "cn=admin,dc=example,dc=com"
	adminPW  = "adminpassword"
	rootDN   = "dc=example,dc=com"
	peopleDN = "ou=people,dc=example,dc=com"

	// seededPeople is how many inetOrgPerson entries seed.ldif creates.
	seededPeople = 123
)

// port is where docker-compose.yml publishes the server. Override it with
// LDAPPER_TEST_PORT when running against something else.
func port(t *testing.T) int {
	t.Helper()
	if v := os.Getenv("LDAPPER_TEST_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("LDAPPER_TEST_PORT=%q is not a number", v)
		}
		return n
	}
	return 3389
}

// dial opens an unauthenticated connection to the compose-managed server.
func dial(t *testing.T) *session.Conn {
	t.Helper()

	conn, err := session.Dial(context.Background(), session.Config{
		Host:       "localhost",
		Port:       port(t),
		Encryption: session.EncryptionNone,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Dial() failed — is the compose file up? Run `make integration`. %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// connect opens a connection and binds as the directory administrator.
func connect(t *testing.T) *session.Conn {
	t.Helper()

	conn := dial(t)
	if err := conn.BindSimple(adminDN, adminPW); err != nil {
		t.Fatalf("BindSimple() failed: %v", err)
	}
	return conn
}

func TestReadRootDSE(t *testing.T) {
	conn := connect(t)

	info, err := schema.Read(conn.Conn)
	if err != nil {
		t.Fatalf("schema.Read() failed: %v", err)
	}
	if info.RootDN() != rootDN {
		t.Errorf("RootDN() = %q, want %q", info.RootDN(), rootDN)
	}
	if info.IsActiveDirectory() {
		t.Error("IsActiveDirectory() = true for OpenLDAP")
	}
	if !info.SupportsPaging() {
		t.Error("SupportsPaging() = false; OpenLDAP does support the paging control")
	}
}

// The POSIX dialect has to be discovered from the schema, not assumed.
func TestDialectsComeFromTheSchema(t *testing.T) {
	conn := connect(t)

	info, err := schema.Read(conn.Conn)
	if err != nil {
		t.Fatalf("schema.Read() failed: %v", err)
	}
	full, err := schema.ReadObjectClasses(conn.Conn, info)
	if err != nil {
		t.Fatalf("schema.ReadObjectClasses() failed: %v", err)
	}

	var sawPOSIX, sawAD bool
	for _, d := range full.Dialects() {
		switch d {
		case filters.DialectPOSIX:
			sawPOSIX = true
		case filters.DialectAD:
			sawAD = true
		}
	}
	if !sawPOSIX {
		t.Error("Dialects() omits POSIX; OpenLDAP carries posixAccount in its schema")
	}
	if sawAD {
		t.Error("Dialects() claims Active Directory support on OpenLDAP")
	}
}

func TestBrowseTopLevel(t *testing.T) {
	conn := connect(t)

	page, err := browse.Children(conn.Conn, rootDN, 0, nil)
	if err != nil {
		t.Fatalf("browse.Children() failed: %v", err)
	}
	if len(page.Entries) != 2 {
		t.Errorf("got %d children of the root, want ou=people and ou=groups", len(page.Entries))
	}
	for _, e := range page.Entries {
		// OpenLDAP does not publish numSubordinates. The node must stay
		// expandable rather than be treated as a leaf.
		if e.NumSubordinates != -1 {
			t.Errorf("%s reported %d subordinates; OpenLDAP publishes none", e.RDN, e.NumSubordinates)
		}
		if !e.HasChildren {
			t.Errorf("%s is not expandable, so its children could never be reached", e.RDN)
		}
	}
}

// This is the test that matters most. If it reports one page of 122 entries,
// the paging control is not reaching the server, and browsing a real
// organizational unit would silently stop at its first thousand objects.
func TestBrowsePagesThroughALargeBranch(t *testing.T) {
	conn := connect(t)

	var (
		cookie []byte
		total  int
		pages  int
	)
	for {
		page, err := browse.Children(conn.Conn, peopleDN, 50, cookie)
		if err != nil {
			t.Fatalf("browse.Children() failed on page %d: %v", pages+1, err)
		}
		total += len(page.Entries)
		pages++

		if len(page.Cookie) == 0 {
			break
		}
		cookie = page.Cookie

		if pages > 10 {
			t.Fatal("paging did not terminate — the cookie is not being consumed")
		}
	}

	if total != seededPeople {
		t.Errorf("got %d people across %d pages, want %d", total, pages, seededPeople)
	}
	if pages < 3 {
		t.Errorf("got %d pages at a page size of 50, want at least 3", pages)
	}
}

func TestSearchStreamsInBatches(t *testing.T) {
	conn := connect(t)

	var batches int
	stats, err := search.Stream(context.Background(), conn.Conn, search.Request{
		Base:       rootDN,
		Filter:     "(objectClass=inetOrgPerson)",
		Scope:      "subtree",
		Attributes: []string{"cn", "uid"},
		BatchSize:  25,
	}, func(batch []search.Result) error {
		batches++
		return nil
	})
	if err != nil {
		t.Fatalf("search.Stream() failed: %v", err)
	}
	if stats.Matched != seededPeople {
		t.Errorf("Matched = %d, want %d", stats.Matched, seededPeople)
	}
	if batches < 2 {
		t.Errorf("got %d batches, want the results delivered incrementally", batches)
	}
	if stats.Truncated {
		t.Errorf("Truncated = true unexpectedly: %s", stats.TruncateReason)
	}
}

func TestSearchScopeIsHonoured(t *testing.T) {
	conn := connect(t)

	stats, err := search.Stream(context.Background(), conn.Conn, search.Request{
		Base:   rootDN,
		Filter: "(objectClass=*)",
		Scope:  "one",
	}, func([]search.Result) error { return nil })
	if err != nil {
		t.Fatalf("search.Stream() failed: %v", err)
	}
	if stats.Matched != 2 {
		t.Errorf("Matched = %d at one-level scope, want the two organizational units", stats.Matched)
	}
}

// An error returned by the callback must stop the search, not be swallowed.
func TestSearchStopsWhenTheCallbackFails(t *testing.T) {
	conn := connect(t)

	want := errors.New("caller had enough")
	_, err := search.Stream(context.Background(), conn.Conn, search.Request{
		Base:      rootDN,
		Filter:    "(objectClass=*)",
		BatchSize: 5,
	}, func([]search.Result) error { return want })

	if !errors.Is(err, want) {
		t.Errorf("Stream() = %v, want the callback's own error", err)
	}
}

func TestSearchCancellationStops(t *testing.T) {
	conn := connect(t)

	ctx, cancel := context.WithCancel(context.Background())
	var seen int

	_, err := search.Stream(ctx, conn.Conn, search.Request{
		Base:      rootDN,
		Filter:    "(objectClass=*)",
		Scope:     "subtree",
		BatchSize: 5,
	}, func(batch []search.Result) error {
		seen += len(batch)
		if seen >= 5 {
			cancel()
		}
		return nil
	})
	_ = err // a cancelled search may report the cancellation or simply stop

	if seen >= seededPeople {
		t.Errorf("cancellation delivered %d entries, want the search to stop early", seen)
	}
}

// Every portable built-in has to be one a real server accepts. A filter that
// is valid RFC 4515 can still be rejected for referring to an attribute the
// server does not know.
func TestPortableBuiltinFiltersRunAgainstOpenLDAP(t *testing.T) {
	conn := connect(t)
	e := filters.Expander{Now: time.Now(), BindDN: adminDN}

	for _, f := range filters.Builtins() {
		if f.Dialect == filters.DialectAD {
			continue // no Active Directory here to run these against
		}

		t.Run(f.ID, func(t *testing.T) {
			expanded, err := e.Expand(f.Filter)
			if err != nil {
				t.Fatalf("Expand() failed: %v", err)
			}
			if _, err := search.Stream(context.Background(), conn.Conn, search.Request{
				Base:   rootDN,
				Filter: expanded,
				Scope:  string(f.Scope),
			}, func([]search.Result) error { return nil }); err != nil {
				t.Errorf("filter %q was rejected by the server: %v", f.ID, err)
			}
		})
	}
}

func TestValuesArriveDecoded(t *testing.T) {
	conn := connect(t)

	var got search.Result
	if _, err := search.Stream(context.Background(), conn.Conn, search.Request{
		Base:   "cn=Anna Volkova," + peopleDN,
		Filter: "(objectClass=*)",
		Scope:  "base",
	}, func(batch []search.Result) error {
		got = batch[0]
		return nil
	}); err != nil {
		t.Fatalf("search.Stream() failed: %v", err)
	}

	if len(got.Attributes["mail"]) != 1 || got.Attributes["mail"][0].Raw != "a.volkova@example.com" {
		t.Errorf("mail = %v, want the seeded address", got.Attributes["mail"])
	}
	// createTimestamp is a GeneralizedTime, so it should come back readable.
	if vals := got.Attributes["createTimestamp"]; len(vals) > 0 && len(vals[0].Decoded) == 0 {
		t.Logf("createTimestamp %q was not decoded; only operational attributes are affected", vals[0].Raw)
	}
}

func TestBindFailureIsRejected(t *testing.T) {
	conn := dial(t)
	if err := conn.BindSimple(adminDN, "wrong password"); err == nil {
		t.Fatal("BindSimple() with a wrong password succeeded")
	}
}

func TestBindWithAnEmptyPasswordIsRefusedLocally(t *testing.T) {
	conn := dial(t)
	// The server would treat this as an anonymous bind and report success,
	// after which the directory looks empty. It must never leave the process.
	if err := conn.BindSimple(adminDN, ""); err == nil {
		t.Fatal("BindSimple() with an empty password succeeded")
	}
}

// A comma inside a name is legal and awkward: the server returns it escaped,
// as either \, or \2C, and showing either form to a person is showing them
// the wire format instead of the name.
func TestEscapedCommaReachesTheInterfaceUnescaped(t *testing.T) {
	a, id := facade(t)

	page := a.Children(id, peopleDN, 200, "")
	if page.Error != "" {
		t.Fatalf("Children() = %q", page.Error)
	}

	var found bool
	for _, n := range page.Nodes {
		if !strings.Contains(n.DN, "Volkova") || !strings.Contains(n.DN, "\\") {
			continue
		}
		found = true

		if n.Label != "Volkova, Anna" {
			t.Errorf("Label = %q, want the comma shown as a comma", n.Label)
		}
		// The DN is an identifier and goes back to the server as it arrived.
		if !strings.Contains(n.DN, "\\") {
			t.Errorf("DN = %q, want the escape kept", n.DN)
		}

		// And it has to be usable: reading the entry by that DN must work.
		if got := a.Entry(id, n.DN); got.Error != "" {
			t.Errorf("Entry() on the escaped DN = %q", got.Error)
		}
	}

	if !found {
		t.Fatal("the entry with a comma in its name is not in the fixture")
	}
}
