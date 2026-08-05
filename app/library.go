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
		// Trust decisions belong to the server, not to the form. Re-saving a
		// profile must not silently revoke a certificate the user approved.
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
