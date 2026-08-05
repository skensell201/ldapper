package ldaperr

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

func TestExplainKnownResultCodes(t *testing.T) {
	tests := []struct {
		name string
		code uint16
		want string
	}{
		{"bad credentials", ldap.LDAPResultInvalidCredentials, "username or password"},
		{"no rights", ldap.LDAPResultInsufficientAccessRights, "permission"},
		{"size limit", ldap.LDAPResultSizeLimitExceeded, "stopped early"},
		{"unwilling", ldap.LDAPResultUnwillingToPerform, "refused"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Explain(&ldap.Error{ResultCode: tt.code})
			if !strings.Contains(strings.ToLower(got), tt.want) {
				t.Errorf("Explain() = %q, want it to mention %q", got, tt.want)
			}
		})
	}
}

func TestExplainActiveDirectorySubCodes(t *testing.T) {
	tests := []struct {
		name string
		diag string
		want string
	}{
		{"wrong password", "80090308: LdapErr: DSID-0C09042A, comment: AcceptSecurityContext error, data 52e, v4563", "password"},
		{"account disabled", "80090308: LdapErr: DSID-0C09042A, comment: AcceptSecurityContext error, data 533, v4563", "disabled"},
		{"account locked", "80090308: LdapErr: DSID-0C09042A, comment: AcceptSecurityContext error, data 775, v4563", "locked"},
		{"password expired", "80090308: LdapErr: DSID-0C09042A, comment: AcceptSecurityContext error, data 532, v4563", "expired"},
		{"no such user", "80090308: LdapErr: DSID-0C09042A, comment: AcceptSecurityContext error, data 525, v4563", "does not exist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New(tt.diag))
			got := strings.ToLower(Explain(err))
			if !strings.Contains(got, tt.want) {
				t.Errorf("Explain() = %q, want it to mention %q", got, tt.want)
			}
		})
	}
}

// An unrecognised sub-code must fall back to the generic message rather than
// producing nothing.
func TestExplainUnknownSubCodeFallsBack(t *testing.T) {
	err := ldap.NewError(ldap.LDAPResultInvalidCredentials,
		errors.New("80090308: LdapErr: DSID-0C09042A, comment: AcceptSecurityContext error, data ffff, v4563"))
	if got := Explain(err); !strings.Contains(strings.ToLower(got), "username or password") {
		t.Errorf("Explain() = %q, want the generic invalid-credentials message", got)
	}
}

func TestExplainPassesThroughNonLDAPErrors(t *testing.T) {
	got := Explain(errors.New("dial tcp: connection refused"))
	if !strings.Contains(got, "connection refused") {
		t.Errorf("Explain() = %q, want the original message preserved", got)
	}
}

func TestExplainHandlesNil(t *testing.T) {
	if got := Explain(nil); got != "" {
		t.Errorf("Explain(nil) = %q, want an empty string", got)
	}
}

// A result code with no entry of its own still has to say something useful.
func TestExplainUnmappedResultCode(t *testing.T) {
	got := Explain(&ldap.Error{ResultCode: ldap.LDAPResultLoopDetect})
	if got == "" {
		t.Error("Explain() returned an empty string for an unmapped result code")
	}
}
