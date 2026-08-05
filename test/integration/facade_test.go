//go:build integration

package integration

import (
	"strconv"
	"testing"

	"github.com/skensell201/ldapper/app"
)

// facade builds an App against a throwaway config directory and connects it to
// the compose-managed server. It exercises exactly the path the window takes:
// save a profile, connect, browse, read an entry.
func facade(t *testing.T) (*app.App, string) {
	t.Helper()
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())

	a, err := app.New()
	if err != nil {
		t.Fatalf("app.New() returned error: %v", err)
	}
	t.Cleanup(func() { a.Shutdown(nil) })

	const id = "test"
	if msg := a.SaveProfile(app.ProfileInput{
		ID:         id,
		Name:       "Test directory",
		Host:       "localhost",
		Port:       port(t),
		Encryption: "none",
		BindMethod: "simple",
		Username:   adminDN,
	}); msg != "" {
		t.Fatalf("SaveProfile() = %q", msg)
	}

	result := a.Connect(id, adminPW)
	if result.Error != "" {
		t.Fatalf("Connect() = %q — is the compose file up? Run `make integration`.", result.Error)
	}
	if result.Certificate != nil {
		t.Fatalf("Connect() asked about a certificate on an unencrypted connection")
	}
	return a, id
}

func TestFacadeConnectReportsTheServer(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())

	a, err := app.New()
	if err != nil {
		t.Fatalf("app.New() returned error: %v", err)
	}
	defer a.Shutdown(nil)

	if msg := a.SaveProfile(app.ProfileInput{
		ID: "test", Name: "Test", Host: "localhost", Port: port(t),
		Encryption: "none", BindMethod: "simple", Username: adminDN,
	}); msg != "" {
		t.Fatalf("SaveProfile() = %q", msg)
	}

	got := a.Connect("test", adminPW)
	if got.Error != "" {
		t.Fatalf("Connect() = %q", got.Error)
	}
	if !got.State.Connected {
		t.Error("State.Connected is false after a successful connect")
	}
	if got.State.RootDN != rootDN {
		t.Errorf("RootDN = %q, want %q", got.State.RootDN, rootDN)
	}
	if got.State.IsActiveDirectory {
		t.Error("IsActiveDirectory = true for OpenLDAP")
	}
	if !got.State.SupportsPaging {
		t.Error("SupportsPaging = false; OpenLDAP does support the control")
	}
	if got.State.BoundAs != adminDN {
		t.Errorf("BoundAs = %q, want %q", got.State.BoundAs, adminDN)
	}

	// The connection now shows up as open in the profile list, which is what
	// the utility bar reads.
	list := a.ListProfiles()
	if len(list) != 1 || !list[0].Connected {
		t.Errorf("ListProfiles() = %+v, want one open connection", list)
	}
}

func TestFacadeConnectRejectsABadPassword(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := app.New()
	defer a.Shutdown(nil)

	if msg := a.SaveProfile(app.ProfileInput{
		ID: "test", Name: "Test", Host: "localhost", Port: port(t),
		Encryption: "none", BindMethod: "simple", Username: adminDN,
	}); msg != "" {
		t.Fatalf("SaveProfile() = %q", msg)
	}

	got := a.Connect("test", "wrong password")
	if got.Error == "" {
		t.Fatal("Connect() reported no error for a wrong password")
	}
	if got.State.Connected {
		t.Error("State.Connected is true after a failed bind")
	}
	// The message has to be the readable one, not a raw result code.
	if got.Error == "LDAP Result Code 49" {
		t.Errorf("Error = %q, want a sentence", got.Error)
	}
}

func TestFacadeBrowsesTheRoot(t *testing.T) {
	a, id := facade(t)

	got := a.Children(id, rootDN, 0, "")
	if got.Error != "" {
		t.Fatalf("Children() = %q", got.Error)
	}
	if len(got.Nodes) != 2 {
		t.Fatalf("got %d children of the root, want ou=people and ou=groups", len(got.Nodes))
	}

	for _, n := range got.Nodes {
		if n.Icon != "ou" {
			t.Errorf("%s drew icon %q, want ou", n.DN, n.Icon)
		}
		if n.Label == n.RDN {
			t.Errorf("%s has label %q, want the attribute name stripped", n.DN, n.Label)
		}
		if n.ChildCount != -1 {
			t.Errorf("%s reported %d children; OpenLDAP publishes no count", n.DN, n.ChildCount)
		}
	}
}

// Paging has to survive the trip through the facade, cookie and all. This is
// the one that would catch a cookie mangled by JSON.
func TestFacadePagesThroughABranch(t *testing.T) {
	a, id := facade(t)

	var (
		cookie string
		total  int
		pages  int
	)
	for {
		got := a.Children(id, peopleDN, 50, cookie)
		if got.Error != "" {
			t.Fatalf("Children() on page %d = %q", pages+1, got.Error)
		}
		total += len(got.Nodes)
		pages++

		if got.Cookie == "" {
			break
		}
		cookie = got.Cookie

		if pages > 10 {
			t.Fatal("paging did not terminate — the cookie is not surviving the round trip")
		}
	}

	if total != seededPeople {
		t.Errorf("got %d people across %d pages, want %d", total, pages, seededPeople)
	}
	if pages < 3 {
		t.Errorf("got %d pages at a page size of 50, want at least 3", pages)
	}
}

func TestFacadeReadsAnEntry(t *testing.T) {
	a, id := facade(t)

	got := a.Entry(id, "cn=Anna Volkova,"+peopleDN)
	if got.Error != "" {
		t.Fatalf("Entry() = %q", got.Error)
	}
	if got.Detail.Icon != "user" {
		t.Errorf("Icon = %q, want user", got.Detail.Icon)
	}
	if got.Detail.RDN != "cn=Anna Volkova" {
		t.Errorf("RDN = %q", got.Detail.RDN)
	}

	var names []string
	var mail string
	for _, row := range got.Detail.Rows {
		names = append(names, row.Name)
		if row.Name == "mail" && len(row.Values) > 0 {
			mail = row.Values[0].Raw
		}
	}
	if mail != "a.volkova@example.com" {
		t.Errorf("mail = %q, want the seeded address", mail)
	}
	// Rows arrive sorted, which is what keeps the table from reshuffling
	// every time somebody clicks a different object.
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("rows are not sorted: %q comes before %q", names[i-1], names[i])
		}
	}
}

func TestFacadeEntryThatIsNotThere(t *testing.T) {
	a, id := facade(t)

	got := a.Entry(id, "cn=nobody,"+peopleDN)
	if got.Error == "" {
		t.Error("Entry() reported no error for an object that does not exist")
	}
}

func TestFacadeFiltersFollowTheConnectedServer(t *testing.T) {
	a, id := facade(t)

	var ad, posix int
	for _, f := range a.ListFilters(id) {
		if !f.Supported {
			continue
		}
		switch f.Dialect {
		case "ad":
			ad++
		case "posix":
			posix++
		}
	}
	if ad != 0 {
		t.Errorf("%d Active Directory filters are offered on OpenLDAP", ad)
	}
	if posix == 0 {
		t.Error("no POSIX filter is offered, though the server carries posixAccount")
	}
}

func TestFacadeDisconnect(t *testing.T) {
	a, id := facade(t)

	a.Disconnect(id)

	if got := a.Children(id, rootDN, 0, ""); got.Error == "" {
		t.Error("Children() succeeded after Disconnect()")
	}
	list := a.ListProfiles()
	if len(list) != 1 || list[0].Connected {
		t.Errorf("ListProfiles() = %+v, want the profile kept but no longer open", list)
	}
}

// A sanity check on the fixture, so a wrong port fails with a clear message
// rather than a confusing connection error.
func TestFixturePortIsSane(t *testing.T) {
	if p := port(t); p < 1 || p > 65535 {
		t.Fatalf("port() = %d", p)
	}
	if _, err := strconv.Atoi(strconv.Itoa(port(t))); err != nil {
		t.Fatal(err)
	}
}
