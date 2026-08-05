package filters

import (
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

// Validate checks that a filter template expands and then parses as RFC 4515.
// Catching this locally matters: an invalid filter sent to a server comes back
// as a protocol error with no hint about which part was wrong.
func Validate(filter string, e Expander) error {
	if filter == "" {
		return fmt.Errorf("filters: the filter is empty")
	}

	expanded, err := e.Expand(filter)
	if err != nil {
		return err
	}

	if _, err := ldap.CompileFilter(expanded); err != nil {
		return fmt.Errorf("filters: %q is not a valid LDAP filter: %w", filter, err)
	}
	return nil
}
