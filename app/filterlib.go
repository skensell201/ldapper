package app

import (
	"time"

	"github.com/skensell201/ldapper/internal/filters"
)

// FilterInput is what the filter editor sends back.
type FilterInput struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Filter      string   `json:"filter"`
	Scope       string   `json:"scope"`
	Base        string   `json:"base"`
	Columns     []string `json:"columns"`
	Dialect     string   `json:"dialect"`
}

// FilterCheck is what the editor shows under the filter box as you type.
type FilterCheck struct {
	Valid    bool   `json:"valid"`
	Error    string `json:"error,omitempty"`
	Expanded string `json:"expanded,omitempty"`
}

// ValidateFilter compiles a filter and shows what its substitutions resolve
// to. Doing this locally matters: the server's answer to a malformed filter is
// a protocol error that says nothing about which part was wrong.
func (a *App) ValidateFilter(profileID, filter string) FilterCheck {
	e := filters.Expander{Now: time.Now(), BindDN: a.bindDNOf(profileID)}

	if err := filters.Validate(filter, e); err != nil {
		return FilterCheck{Error: err.Error()}
	}
	expanded, err := e.Expand(filter)
	if err != nil {
		return FilterCheck{Error: err.Error()}
	}
	return FilterCheck{Valid: true, Expanded: expanded}
}

// SaveFilter stores a filter after checking that it compiles.
//
// The check runs with no bound identity, so a filter using {{me}} is refused
// at save time rather than at the moment somebody reaches for it. A filter
// that only compiles while connected fails at the worst possible moment.
func (a *App) SaveFilter(in FilterInput) string {
	if check := a.ValidateFilter("", in.Filter); !check.Valid {
		return check.Error
	}

	err := a.filters.Save(filters.Filter{
		ID:          in.ID,
		Name:        in.Name,
		Description: in.Description,
		Filter:      in.Filter,
		Scope:       filters.Scope(in.Scope),
		Base:        in.Base,
		Columns:     in.Columns,
		Dialect:     filters.Dialect(in.Dialect),
	})
	if err != nil {
		return err.Error()
	}
	return ""
}

// ResetFilter returns a built-in filter to the version Ldapper ships.
func (a *App) ResetFilter(id string) string {
	if err := a.filters.Reset(id); err != nil {
		return err.Error()
	}
	return ""
}

// DeleteFilter removes a filter. A built-in stays gone across restarts.
func (a *App) DeleteFilter(id string) string {
	if err := a.filters.Delete(id); err != nil {
		return err.Error()
	}
	return ""
}

// RestoreDefaultFilters undoes every edit and deletion of a built-in, leaving
// the user's own filters alone.
func (a *App) RestoreDefaultFilters() string {
	if err := a.filters.RestoreDefaults(); err != nil {
		return err.Error()
	}
	return ""
}

// bindDNOf returns the identity a connection is bound as, or an empty string
// when it is not open.
func (a *App) bindDNOf(profileID string) string {
	c, ok := a.live(profileID)
	if !ok || c.conn == nil {
		return ""
	}
	return c.conn.BindDN
}
