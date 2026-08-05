package profiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSecrets stands in for the operating system's keychain. Tests must never
// reach the real one: on macOS it raises an access prompt, and on a CI runner
// there is no secret service at all.
type fakeSecrets struct {
	items     map[string]string
	available bool
}

func newFakeSecrets() *fakeSecrets {
	return &fakeSecrets{items: map[string]string{}, available: true}
}

func (f *fakeSecrets) Set(service, user, password string) error {
	if !f.available {
		return errors.New("no secret service")
	}
	f.items[service+"/"+user] = password
	return nil
}

func (f *fakeSecrets) Get(service, user string) (string, error) {
	if !f.available {
		return "", errors.New("no secret service")
	}
	pw, ok := f.items[service+"/"+user]
	if !ok {
		return "", errors.New("not found")
	}
	return pw, nil
}

func (f *fakeSecrets) Delete(service, user string) error {
	if !f.available {
		return errors.New("no secret service")
	}
	delete(f.items, service+"/"+user)
	return nil
}

func newTestStore(t *testing.T) (*Store, *fakeSecrets) {
	t.Helper()
	return newTestStoreAt(t, filepath.Join(t.TempDir(), "connections.json"))
}

func newTestStoreAt(t *testing.T, path string) (*Store, *fakeSecrets) {
	t.Helper()
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	secrets := newFakeSecrets()
	s.secrets = secrets
	return s, secrets
}

func sample() Profile {
	return Profile{
		ID:         "dc01",
		Name:       "CORP production",
		Host:       "dc01.corp.example.com",
		Port:       636,
		Encryption: EncryptionLDAPS,
		BindMethod: BindNTLM,
		Domain:     "CORP",
		Username:   "a.kensel",
	}
}

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connections.json")

	s, _ := newTestStoreAt(t, path)
	if err := s.Save(sample()); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	reopened, _ := newTestStoreAt(t, path)
	got, ok := reopened.Get("dc01")
	if !ok {
		t.Fatal("Get() could not find the saved profile")
	}
	if got.Host != "dc01.corp.example.com" || got.Port != 636 {
		t.Errorf("Get() = %+v, want the saved host and port", got)
	}
	if got.BindMethod != BindNTLM || got.Domain != "CORP" {
		t.Errorf("Get() = %+v, want the bind settings preserved", got)
	}
}

func TestStoreNeverWritesAPasswordToDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connections.json")
	s, secrets := newTestStoreAt(t, path)

	p := sample()
	p.RememberPassword = true
	p = p.WithPassword("hunter2")
	if err := s.Save(p); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the profile file: %v", err)
	}
	if strings.Contains(string(data), "hunter2") {
		t.Fatalf("the password was written to connections.json:\n%s", data)
	}
	if got, _ := secrets.Get(keyringService, "dc01"); got != "hunter2" {
		t.Errorf("the password did not reach the keychain, got %q", got)
	}
}

func TestLoadPassword(t *testing.T) {
	s, _ := newTestStore(t)

	p := sample()
	p.RememberPassword = true
	if err := s.Save(p.WithPassword("hunter2")); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	stored, _ := s.Get("dc01")
	if stored.Password() != "" {
		t.Error("the in-memory profile list is holding a password")
	}

	loaded, err := s.LoadPassword(stored)
	if err != nil {
		t.Fatalf("LoadPassword() returned error: %v", err)
	}
	if loaded.Password() != "hunter2" {
		t.Errorf("Password() = %q, want the remembered password", loaded.Password())
	}
}

func TestLoadPasswordIsANoOpWhenNotRemembered(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Save(sample()); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	stored, _ := s.Get("dc01")
	got, err := s.LoadPassword(stored)
	if err != nil {
		t.Fatalf("LoadPassword() returned error: %v", err)
	}
	if got.Password() != "" {
		t.Errorf("Password() = %q, want empty", got.Password())
	}
}

// A machine with no secret service is a normal condition, not a crash: the
// user can still type the password each time.
func TestMissingKeychainIsReported(t *testing.T) {
	s, secrets := newTestStore(t)
	secrets.available = false

	p := sample()
	p.RememberPassword = true
	err := s.Save(p.WithPassword("hunter2"))
	if !errors.Is(err, ErrNoKeychain) {
		t.Errorf("Save() = %v, want it to wrap ErrNoKeychain", err)
	}
}

func TestTrustFingerprintIsPerProfile(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Save(sample()); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	const fp = "4B:9C:1E:07:AA:63:F2:80"
	if err := s.Trust("dc01", fp); err != nil {
		t.Fatalf("Trust() returned error: %v", err)
	}

	got, _ := s.Get("dc01")
	if !got.Trusts(fp) {
		t.Error("Trusts() = false for a fingerprint that was just trusted")
	}
	if got.Trusts("00:11:22:33") {
		t.Error("Trusts() = true for a fingerprint nobody approved")
	}
}

// The same fingerprint gets written several ways. Comparison must not care.
func TestTrustsIgnoresFormatting(t *testing.T) {
	p := Profile{TrustedFingerprints: []string{"4B:9C:1E:07"}}
	for _, in := range []string{"4b:9c:1e:07", "4B9C1E07", "4b 9c 1e 07", "4b-9c-1e-07"} {
		if !p.Trusts(in) {
			t.Errorf("Trusts(%q) = false, want the same fingerprint recognised", in)
		}
	}
}

func TestTrustDoesNotLeakToOtherProfiles(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Save(sample()); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	second := sample()
	second.ID = "dc02"
	second.Host = "dc02.corp.example.com"
	if err := s.Save(second); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	const fp = "4B:9C:1E:07:AA:63:F2:80"
	if err := s.Trust("dc01", fp); err != nil {
		t.Fatalf("Trust() returned error: %v", err)
	}

	other, _ := s.Get("dc02")
	if other.Trusts(fp) {
		t.Error("trusting a certificate for one profile trusted it for another")
	}
}

func TestTrustIsIdempotent(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Save(sample()); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := s.Trust("dc01", "4B:9C"); err != nil {
			t.Fatalf("Trust() returned error: %v", err)
		}
	}
	got, _ := s.Get("dc01")
	if len(got.TrustedFingerprints) != 1 {
		t.Errorf("TrustedFingerprints = %v, want one entry", got.TrustedFingerprints)
	}
}

func TestTrustRejectsAnUnknownProfile(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Trust("nobody", "4B:9C"); err == nil {
		t.Error("Trust() on an unknown profile succeeded, want an error")
	}
}

func TestDeleteRemovesTheProfileAndItsPassword(t *testing.T) {
	s, secrets := newTestStore(t)

	p := sample()
	p.RememberPassword = true
	if err := s.Save(p.WithPassword("hunter2")); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if err := s.Delete("dc01"); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}
	if _, ok := s.Get("dc01"); ok {
		t.Error("the profile is still present after Delete()")
	}
	if _, err := secrets.Get(keyringService, "dc01"); err == nil {
		t.Error("the password outlived the profile it belonged to")
	}
}

func TestSaveRejectsAnIncompleteProfile(t *testing.T) {
	s, _ := newTestStore(t)
	tests := []struct {
		name string
		mut  func(*Profile)
	}{
		{"no id", func(p *Profile) { p.ID = "" }},
		{"no host", func(p *Profile) { p.Host = "" }},
		{"port zero", func(p *Profile) { p.Port = 0 }},
		{"port too high", func(p *Profile) { p.Port = 70000 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := sample()
			tt.mut(&p)
			if err := s.Save(p); err == nil {
				t.Error("Save() accepted an incomplete profile, want an error")
			}
		})
	}
}

func TestNewStoreRejectsAMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connections.json")
	if err := os.WriteFile(path, []byte("[not json"), 0o600); err != nil {
		t.Fatalf("cannot write the fixture: %v", err)
	}
	if _, err := NewStore(path); err == nil {
		t.Error("NewStore() accepted a malformed file, want an error")
	}
}
