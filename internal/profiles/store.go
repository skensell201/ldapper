package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

// keyringService is the name Ldapper's entries appear under in the OS keychain.
const keyringService = "ldapper"

// ErrNoKeychain means the operating system has no usable secret store. It is a
// condition to report, not a failure to abort on: the user can still type the
// password each time.
var ErrNoKeychain = errors.New("profiles: no usable keychain on this system")

// secrets is the operating system's password store, kept behind an interface
// so tests never touch the real one. On macOS that would raise an access
// prompt; on a CI runner there is no secret service to touch at all.
type secrets interface {
	Set(service, user, password string) error
	Get(service, user string) (string, error)
	Delete(service, user string) error
}

// osKeyring is the real implementation, backed by Credential Manager on
// Windows, Keychain on macOS and the Secret Service on Linux.
type osKeyring struct{}

func (osKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (osKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (osKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

// Store keeps connection profiles in a JSON file. It is safe for concurrent use.
type Store struct {
	path    string
	secrets secrets

	mu   sync.RWMutex
	list []Profile
}

// NewStore loads the profiles at path. A missing file means no profiles yet.
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, secrets: osKeyring{}}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("profiles: cannot read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &s.list); err != nil {
		return nil, fmt.Errorf("profiles: %s is not valid JSON: %w", path, err)
	}
	return s, nil
}

// All returns every saved profile.
func (s *Store) All() []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Profile, len(s.list))
	copy(out, s.list)
	return out
}

// Get returns one profile by ID.
func (s *Store) Get(id string) (Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.list {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// Save writes p, replacing any profile with the same ID. If p asks for its
// password to be remembered, it goes to the keychain and never to the file.
func (s *Store) Save(p Profile) error {
	switch {
	case p.ID == "":
		return fmt.Errorf("profiles: a profile needs an ID")
	case p.Host == "":
		return fmt.Errorf("profiles: profile %q needs a host", p.ID)
	case p.Port < 1 || p.Port > 65535:
		return fmt.Errorf("profiles: profile %q has port %d, want 1-65535", p.ID, p.Port)
	}

	if p.RememberPassword && p.password != "" {
		if err := s.secrets.Set(keyringService, p.ID, p.password); err != nil {
			return fmt.Errorf("%w: %v", ErrNoKeychain, err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	stored := p
	stored.password = ""

	replaced := false
	for i := range s.list {
		if s.list[i].ID == p.ID {
			s.list[i] = stored
			replaced = true
			break
		}
	}
	if !replaced {
		s.list = append(s.list, stored)
	}
	return s.flush()
}

// LoadPassword returns the profile with its remembered password attached.
// A profile that does not remember its password comes back unchanged, and a
// missing keychain returns ErrNoKeychain — in both cases the caller should ask
// the user rather than give up.
func (s *Store) LoadPassword(p Profile) (Profile, error) {
	if !p.RememberPassword {
		return p, nil
	}
	pw, err := s.secrets.Get(keyringService, p.ID)
	if err != nil {
		return p, fmt.Errorf("%w: %v", ErrNoKeychain, err)
	}
	return p.WithPassword(pw), nil
}

// Trust records a certificate fingerprint as approved for one profile only.
func (s *Store) Trust(id, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.list {
		if s.list[i].ID != id {
			continue
		}
		if s.list[i].Trusts(fingerprint) {
			return nil
		}
		s.list[i].TrustedFingerprints = append(s.list[i].TrustedFingerprints, fingerprint)
		return s.flush()
	}
	return fmt.Errorf("profiles: no profile with ID %q", id)
}

// Delete removes a profile and forgets its password.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// A keychain entry that is already gone, or a machine with no keychain at
	// all, is not a reason to refuse to delete the profile.
	_ = s.secrets.Delete(keyringService, id)

	kept := s.list[:0]
	for _, p := range s.list {
		if p.ID != id {
			kept = append(kept, p)
		}
	}
	s.list = kept
	return s.flush()
}

// flush writes the profile list. The caller must hold the write lock.
func (s *Store) flush() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("profiles: cannot create %s: %w", filepath.Dir(s.path), err)
	}

	data, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return fmt.Errorf("profiles: cannot encode the profiles: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("profiles: cannot write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("profiles: cannot replace %s: %w", s.path, err)
	}
	return nil
}
