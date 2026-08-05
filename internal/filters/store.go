package filters

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// overlay is the on-disk shape: only what the user changed, never a copy of the
// shipped set. Keeping it this way is what lets an Ldapper update improve a
// built-in filter without discarding anyone's edits.
type overlay struct {
	// Overrides holds edited built-ins, keyed by filter ID.
	Overrides map[string]Filter `json:"overrides"`
	// Custom holds filters the user created.
	Custom []Filter `json:"custom"`
	// Removed lists built-in IDs the user deleted, so they stay gone.
	Removed []string `json:"removed"`
}

// Store is the filter library: the shipped set with the user's overlay applied.
// It is safe for concurrent use.
type Store struct {
	path string

	mu   sync.RWMutex
	over overlay
}

// NewStore loads the overlay at path. A missing file is not an error — it means
// nothing has been customised yet, and no file is written until something is.
func NewStore(path string) (*Store, error) {
	s := &Store{
		path: path,
		over: overlay{Overrides: map[string]Filter{}},
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("filters: cannot read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &s.over); err != nil {
		return nil, fmt.Errorf("filters: %s is not valid JSON: %w", path, err)
	}
	if s.over.Overrides == nil {
		s.over.Overrides = map[string]Filter{}
	}
	return s, nil
}

// All returns every filter the user should see, built-ins first in their
// shipped order, then custom filters by name.
func (s *Store) All() []Filter {
	s.mu.RLock()
	defer s.mu.RUnlock()

	removed := map[string]bool{}
	for _, id := range s.over.Removed {
		removed[id] = true
	}

	var out []Filter
	for _, f := range builtins {
		if removed[f.ID] {
			continue
		}
		if edited, ok := s.over.Overrides[f.ID]; ok {
			edited.BuiltIn = true
			edited.Modified = true
			out = append(out, edited)
			continue
		}
		out = append(out, f)
	}

	custom := make([]Filter, len(s.over.Custom))
	copy(custom, s.over.Custom)
	sort.Slice(custom, func(i, j int) bool { return custom[i].Name < custom[j].Name })
	return append(out, custom...)
}

// Get returns one filter by ID.
func (s *Store) Get(id string) (Filter, bool) {
	for _, f := range s.All() {
		if f.ID == id {
			return f, true
		}
	}
	return Filter{}, false
}

// Save writes f. Saving over a built-in ID records an override; any other ID
// creates or replaces a custom filter.
func (s *Store) Save(f Filter) error {
	if f.ID == "" {
		return fmt.Errorf("filters: a filter needs an ID")
	}
	if f.Name == "" {
		return fmt.Errorf("filters: filter %q needs a name", f.ID)
	}
	if f.Filter == "" {
		return fmt.Errorf("filters: filter %q needs a filter expression", f.ID)
	}
	if f.Scope == "" {
		f.Scope = ScopeSubtree
	}
	if f.Dialect == "" {
		f.Dialect = DialectGeneric
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// These two are derived on read, never stored.
	f.BuiltIn, f.Modified = false, false

	if isBuiltin(f.ID) {
		s.over.Overrides[f.ID] = f
		s.over.Removed = without(s.over.Removed, f.ID)
	} else {
		replaced := false
		for i := range s.over.Custom {
			if s.over.Custom[i].ID == f.ID {
				s.over.Custom[i] = f
				replaced = true
				break
			}
		}
		if !replaced {
			s.over.Custom = append(s.over.Custom, f)
		}
	}
	return s.flush()
}

// Reset drops the user's edit to a built-in filter. It is a no-op for custom
// filters, which have no shipped version to return to.
func (s *Store) Reset(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.over.Overrides[id]; !ok {
		return nil
	}
	delete(s.over.Overrides, id)
	return s.flush()
}

// Delete removes a filter. A built-in is remembered as removed so that it does
// not reappear the next time Ldapper starts.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if isBuiltin(id) {
		delete(s.over.Overrides, id)
		for _, existing := range s.over.Removed {
			if existing == id {
				return nil
			}
		}
		s.over.Removed = append(s.over.Removed, id)
		return s.flush()
	}

	kept := s.over.Custom[:0]
	for _, f := range s.over.Custom {
		if f.ID != id {
			kept = append(kept, f)
		}
	}
	s.over.Custom = kept
	return s.flush()
}

// RestoreDefaults undoes every edit and deletion of a built-in filter. Custom
// filters are left alone — they were never part of the shipped set.
func (s *Store) RestoreDefaults() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.over.Overrides = map[string]Filter{}
	s.over.Removed = nil
	return s.flush()
}

// flush writes the overlay to disk. The caller must hold the write lock.
func (s *Store) flush() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("filters: cannot create %s: %w", filepath.Dir(s.path), err)
	}

	data, err := json.MarshalIndent(s.over, "", "  ")
	if err != nil {
		return fmt.Errorf("filters: cannot encode the overlay: %w", err)
	}

	// Write to a neighbouring file and rename, so a crash mid-write cannot
	// leave the library truncated.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("filters: cannot write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("filters: cannot replace %s: %w", s.path, err)
	}
	return nil
}

func isBuiltin(id string) bool {
	for _, f := range builtins {
		if f.ID == id {
			return true
		}
	}
	return false
}

func without(list []string, id string) []string {
	out := list[:0]
	for _, existing := range list {
		if existing != id {
			out = append(out, existing)
		}
	}
	return out
}
