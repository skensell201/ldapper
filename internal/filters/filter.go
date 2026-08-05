// Package filters holds the library of saved LDAP searches: the set shipped
// with Ldapper, the user's edits to it, and their own additions.
package filters

// Dialect says which servers a filter can run against.
type Dialect string

const (
	// DialectAD marks filters using Active Directory extensions, such as the
	// bitwise matching rule 1.2.840.113556.1.4.803.
	DialectAD Dialect = "ad"
	// DialectGeneric marks filters that work on any LDAP server.
	DialectGeneric Dialect = "generic"
	// DialectPOSIX marks filters needing the POSIX schema, which Active
	// Directory does not carry by default.
	DialectPOSIX Dialect = "posix"
)

// Scope is the LDAP search scope.
type Scope string

const (
	ScopeBase    Scope = "base"
	ScopeOne     Scope = "one"
	ScopeSubtree Scope = "subtree"
)

// Filter is one saved search.
type Filter struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Filter is an RFC 4515 filter that may contain {{…}} substitutions.
	Filter string `json:"filter"`
	Scope  Scope  `json:"scope"`
	// Base is the search base. Empty means the server's default naming context.
	Base    string   `json:"base,omitempty"`
	Columns []string `json:"columns,omitempty"`
	Dialect Dialect  `json:"dialect"`

	// BuiltIn is true for filters that ship with Ldapper. Not persisted:
	// it is derived from where the filter was loaded from.
	BuiltIn bool `json:"-"`
	// Modified is true for a built-in filter the user has edited.
	Modified bool `json:"-"`
}
