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
		t.Fatalf(`decodeCookie("") returned error: %v`, err)
	}
	if len(got) != 0 {
		t.Errorf(`decodeCookie("") = %v, want no bytes`, got)
	}
}

func TestDecodeCookieRejectsGarbage(t *testing.T) {
	if _, err := decodeCookie("not base64!"); err == nil {
		t.Error("decodeCookie() accepted a value it could not decode")
	}
}

func TestFirstRDN(t *testing.T) {
	tests := []struct {
		dn   string
		want string
	}{
		{"CN=Anna Volkova,OU=Users,DC=example,DC=com", "CN=Anna Volkova"},
		{"DC=corp,DC=example,DC=com", "DC=corp"},
		{`CN=Volkova\, Anna,OU=Users,DC=example,DC=com`, `CN=Volkova\, Anna`},
		{"dc=example", "dc=example"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.dn, func(t *testing.T) {
			if got := firstRDN(tt.dn); got != tt.want {
				t.Errorf("firstRDN(%q) = %q, want %q", tt.dn, got, tt.want)
			}
		})
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
