package schema

import (
	"testing"

	"github.com/skensell201/ldapper/internal/filters"
)

func TestDetectActiveDirectory(t *testing.T) {
	info := Info{SupportedControls: []string{
		"1.2.840.113556.1.4.319", // paged results
		"1.2.840.113556.1.4.800", // the marker that says Active Directory
	}}
	if !info.IsActiveDirectory() {
		t.Error("IsActiveDirectory() = false for a server advertising 1.2.840.113556.1.4.800")
	}
}

func TestDetectPlainLDAP(t *testing.T) {
	info := Info{SupportedControls: []string{"1.2.840.113556.1.4.319"}}
	if info.IsActiveDirectory() {
		t.Error("IsActiveDirectory() = true for a server that only supports paging")
	}
}

func TestSupportsPaging(t *testing.T) {
	with := Info{SupportedControls: []string{"1.2.840.113556.1.4.319"}}
	if !with.SupportsPaging() {
		t.Error("SupportsPaging() = false for a server advertising the paging control")
	}
	if (Info{}).SupportsPaging() {
		t.Error("SupportsPaging() = true for a server advertising nothing")
	}
}

func TestDialectsOnActiveDirectory(t *testing.T) {
	info := Info{SupportedControls: []string{"1.2.840.113556.1.4.800"}}
	got := info.Dialects()

	if !contains(got, filters.DialectAD) {
		t.Error("Dialects() omits the AD dialect on an AD server")
	}
	if !contains(got, filters.DialectGeneric) {
		t.Error("Dialects() omits the generic dialect, which every server supports")
	}
	if contains(got, filters.DialectPOSIX) {
		t.Error("Dialects() claims POSIX support on stock Active Directory")
	}
}

func TestDialectsOnOpenLDAP(t *testing.T) {
	info := Info{ObjectClasses: []string{"posixAccount", "inetOrgPerson"}}
	got := info.Dialects()

	if contains(got, filters.DialectAD) {
		t.Error("Dialects() claims AD support on a server that is not AD")
	}
	if !contains(got, filters.DialectPOSIX) {
		t.Error("Dialects() omits POSIX on a server carrying posixAccount")
	}
}

func TestDialectsIsCaseInsensitiveAboutObjectClasses(t *testing.T) {
	info := Info{ObjectClasses: []string{"POSIXACCOUNT"}}
	if !contains(info.Dialects(), filters.DialectPOSIX) {
		t.Error("Dialects() missed posixAccount spelled in a different case")
	}
}

func TestDefaultNamingContext(t *testing.T) {
	info := Info{
		DefaultNamingContext: "DC=corp,DC=example,DC=com",
		NamingContexts:       []string{"DC=corp,DC=example,DC=com", "CN=Configuration,DC=corp,DC=example,DC=com"},
	}
	if got := info.RootDN(); got != "DC=corp,DC=example,DC=com" {
		t.Errorf("RootDN() = %q, want the default naming context", got)
	}
}

func TestRootDNFallsBackToTheFirstNamingContext(t *testing.T) {
	// OpenLDAP does not publish defaultNamingContext.
	info := Info{NamingContexts: []string{"dc=example,dc=com"}}
	if got := info.RootDN(); got != "dc=example,dc=com" {
		t.Errorf("RootDN() = %q, want the first naming context", got)
	}
}

func TestRootDNOfASilentServer(t *testing.T) {
	if got := (Info{}).RootDN(); got != "" {
		t.Errorf("RootDN() = %q, want empty when the server published nothing", got)
	}
}

func TestObjectClassName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"a real definition",
			"( 1.3.6.1.1.1.2.0 NAME 'posixAccount' SUP top AUXILIARY MUST ( cn $ uid ) )",
			"posixAccount",
		},
		{"no name clause", "( 1.3.6.1.1.1.2.0 SUP top AUXILIARY )", ""},
		{"unterminated name", "( 1.2.3 NAME 'broken", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := objectClassName(tt.in); got != tt.want {
				t.Errorf("objectClassName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func contains(list []filters.Dialect, want filters.Dialect) bool {
	for _, d := range list {
		if d == want {
			return true
		}
	}
	return false
}
