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

// Re-saving a profile from the form must not revoke a certificate the user
// already approved: the form has no field for it and would send nothing.
func TestSaveProfileKeepsTrustedFingerprints(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	in := ProfileInput{ID: "dc01", Name: "CORP", Host: "dc01.example.com", Port: 636}
	if msg := a.SaveProfile(in); msg != "" {
		t.Fatalf("SaveProfile() = %q", msg)
	}
	if msg := a.TrustCertificate("dc01", "4B:9C:1E:07"); msg != "" {
		t.Fatalf("TrustCertificate() = %q", msg)
	}

	in.Port = 389
	if msg := a.SaveProfile(in); msg != "" {
		t.Fatalf("SaveProfile() = %q", msg)
	}

	p, ok := a.profileStore().Get("dc01")
	if !ok {
		t.Fatal("the profile disappeared")
	}
	if !p.Trusts("4B:9C:1E:07") {
		t.Error("re-saving the profile discarded the approved fingerprint")
	}
}

func TestDeleteProfile(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if msg := a.SaveProfile(ProfileInput{ID: "dc01", Name: "CORP", Host: "h", Port: 636}); msg != "" {
		t.Fatalf("SaveProfile() = %q", msg)
	}
	if msg := a.DeleteProfile("dc01"); msg != "" {
		t.Fatalf("DeleteProfile() = %q", msg)
	}
	if len(a.ListProfiles()) != 0 {
		t.Error("the profile survived deletion")
	}
}

func TestListFiltersMarksWhatThisServerCannotRun(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	// With no connection open, nothing is known about the server, so no
	// filter is claimed to be supported — but every one is still listed.
	got := a.ListFilters("nobody")
	if len(got) != 18 {
		t.Fatalf("ListFilters() returned %d filters, want all 18 regardless of support", len(got))
	}

	for _, f := range got {
		if f.Supported {
			t.Errorf("filter %q is marked supported with no connection open", f.ID)
		}
		if f.Reason == "" {
			t.Errorf("filter %q is unsupported with no reason given", f.ID)
		}
	}
}

func TestListFiltersFollowsTheConnectedServer(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	// Stand in for a connection to a plain LDAP server: generic filters run,
	// the Active Directory ones do not.
	a.setLive("dc01", &liveConn{})

	var ad, generic int
	for _, f := range a.ListFilters("dc01") {
		switch {
		case f.Dialect == "ad" && f.Supported:
			ad++
		case f.Dialect == "generic" && f.Supported:
			generic++
		}
	}
	if ad != 0 {
		t.Errorf("%d Active Directory filters are marked supported on a plain LDAP server", ad)
	}
	if generic == 0 {
		t.Error("no generic filter is marked supported, though every server can answer them")
	}
}
