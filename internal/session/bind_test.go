package session

import "testing"

func TestSplitAccount(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantDomain string
		wantUser   string
	}{
		{"down-level name", `CORP\a.kensel`, "CORP", "a.kensel"},
		{"forward slash", "CORP/a.kensel", "CORP", "a.kensel"},
		{"bare account", "a.kensel", "", "a.kensel"},
		{"UPN is left alone", "a.kensel@corp.example.com", "", "a.kensel@corp.example.com"},
		{"surrounding space", `  CORP\a.kensel  `, "CORP", "a.kensel"},
		{"empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, user := SplitAccount(tt.in)
			if domain != tt.wantDomain || user != tt.wantUser {
				t.Errorf("SplitAccount(%q) = (%q, %q), want (%q, %q)",
					tt.in, domain, user, tt.wantDomain, tt.wantUser)
			}
		})
	}
}

// An empty password makes most servers perform an unauthenticated bind that
// reports success, after which the directory looks empty and the user goes
// hunting for a permissions problem. Refuse it before it leaves the process.
func TestBindSimpleRejectsAnEmptyPassword(t *testing.T) {
	c := &Conn{}
	if err := c.BindSimple("CN=admin,DC=example,DC=com", ""); err == nil {
		t.Error("BindSimple() with an empty password succeeded, want an error")
	}
}

func TestBindSimpleRejectsAnEmptyUsername(t *testing.T) {
	c := &Conn{}
	if err := c.BindSimple("", "hunter2"); err == nil {
		t.Error("BindSimple() with no username succeeded, want an error")
	}
}

func TestBindNTLMRequiresADomain(t *testing.T) {
	c := &Conn{}
	if err := c.BindNTLM("", "a.kensel", "hunter2"); err == nil {
		t.Error("BindNTLM() with no domain succeeded, want an error")
	}
}

func TestBindNTLMRejectsAnEmptyPassword(t *testing.T) {
	c := &Conn{}
	if err := c.BindNTLM("CORP", "a.kensel", ""); err == nil {
		t.Error("BindNTLM() with an empty password succeeded, want an error")
	}
}
