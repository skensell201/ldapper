// Package ldaperr turns LDAP protocol errors into sentences that say what went
// wrong and what to do about it.
package ldaperr

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/go-ldap/ldap/v3"
)

// adSubCode matches the "data 52e" fragment Active Directory buries inside the
// diagnostic message of an invalid-credentials error. go-ldap keeps that text
// in the wrapped Err rather than a field of its own, so this reads the joined
// message.
var adSubCode = regexp.MustCompile(`data ([0-9a-fA-F]+)`)

var adReasons = map[string]string{
	"525": "That user does not exist in this directory.",
	"52e": "Wrong username or password.",
	"530": "That account is not allowed to sign in at this time of day.",
	"531": "That account is not allowed to sign in from this computer.",
	"532": "The password has expired and must be changed before you can sign in.",
	"533": "That account is disabled.",
	"701": "That account has expired.",
	"773": "The password must be changed before this account can be used.",
	"775": "That account is locked out. It will unlock on its own, or an administrator can unlock it now.",
}

var byResultCode = map[uint16]string{
	ldap.LDAPResultInvalidCredentials:          "Wrong username or password.",
	ldap.LDAPResultInsufficientAccessRights:    "This account does not have permission to read that object. Sign in with an account that does.",
	ldap.LDAPResultSizeLimitExceeded:           "The server stopped early — you are seeing part of the results. Narrow the filter or search from a lower branch.",
	ldap.LDAPResultTimeLimitExceeded:           "The server stopped early — you are seeing part of the results. Narrow the filter or search from a lower branch.",
	ldap.LDAPResultNoSuchObject:                "There is no object at that distinguished name. It may have been moved or deleted.",
	ldap.LDAPResultReferral:                    "That object lives in a different naming context. Connect to the server that holds it.",
	ldap.LDAPResultUnwillingToPerform:          "The server refused the request. Most often it requires an encrypted connection before it will accept a bind.",
	ldap.LDAPResultStrongAuthRequired:          "The server requires a stronger authentication method. Switch on LDAPS or StartTLS.",
	ldap.LDAPResultInappropriateAuthentication: "The server rejected this way of signing in. Try a different bind method.",
	ldap.LDAPResultConstraintViolation:         "The value breaks a rule the server enforces on that attribute.",
	ldap.LDAPResultBusy:                        "The server is too busy to answer right now. Try again in a moment.",
	ldap.LDAPResultUnavailable:                 "The server is not accepting requests right now.",
}

// Explain returns a sentence describing err. Errors that are not LDAP protocol
// errors are returned as they are — a refused TCP connection already says what
// happened.
func Explain(err error) string {
	if err == nil {
		return ""
	}

	var lerr *ldap.Error
	if !errors.As(err, &lerr) {
		return err.Error()
	}

	if lerr.ResultCode == ldap.LDAPResultInvalidCredentials {
		if m := adSubCode.FindStringSubmatch(diagnostic(lerr)); m != nil {
			if reason, ok := adReasons[m[1]]; ok {
				return reason
			}
		}
	}

	if msg, ok := byResultCode[lerr.ResultCode]; ok {
		return msg
	}

	return fallback(lerr)
}

// diagnostic returns the server's own diagnostic text, which go-ldap stores as
// the wrapped error rather than a named field.
func diagnostic(lerr *ldap.Error) string {
	if lerr.Err == nil {
		return ""
	}
	return lerr.Err.Error()
}

// fallback describes a result code that has no message of its own.
//
// It does not call lerr.Error(): go-ldap dereferences the wrapped Err there
// without checking it, so an ldap.Error carrying only a result code panics.
func fallback(lerr *ldap.Error) string {
	name := ldap.LDAPResultCodeMap[lerr.ResultCode]
	if name == "" {
		name = "Unknown"
	}
	if diag := diagnostic(lerr); diag != "" {
		return fmt.Sprintf("The server returned %s (code %d): %s", name, lerr.ResultCode, diag)
	}
	return fmt.Sprintf("The server returned %s (code %d).", name, lerr.ResultCode)
}
