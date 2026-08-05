// Package profiles stores connection settings. Passwords are the one thing it
// will not put in a file: those go to the operating system's keychain.
package profiles

import "strings"

// Encryption is how the connection is protected.
type Encryption string

const (
	EncryptionNone     Encryption = "none"
	EncryptionLDAPS    Encryption = "ldaps"
	EncryptionStartTLS Encryption = "starttls"
)

// BindMethod is how the connection authenticates.
type BindMethod string

const (
	BindSimple BindMethod = "simple"
	BindNTLM   BindMethod = "ntlm"
	// BindAnonymous connects without credentials. Plenty of directories
	// allow reading that way, and refusing to offer it means telling somebody
	// to invent an account they do not need.
	BindAnonymous BindMethod = "anonymous"
)

// Profile is one saved connection.
type Profile struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Host       string     `json:"host"`
	Port       int        `json:"port"`
	Encryption Encryption `json:"encryption"`
	BindMethod BindMethod `json:"bindMethod"`
	// Domain is the NetBIOS domain for an NTLM bind. Unused for simple binds.
	Domain string `json:"domain,omitempty"`
	// Username is a DN or UPN for a simple bind, a bare account name for NTLM.
	Username string `json:"username,omitempty"`
	// RememberPassword asks for the password to be kept in the OS keychain.
	RememberPassword bool `json:"rememberPassword"`
	// TrustedFingerprints holds SHA-256 fingerprints the user approved for
	// this server, and this server alone.
	TrustedFingerprints []string `json:"trustedFingerprints,omitempty"`

	// password is never serialised — the lower-case name keeps it out of JSON
	// as well as out of reach of other packages.
	password string
}

// Password returns the password held in memory for this profile.
func (p Profile) Password() string { return p.password }

// WithPassword returns a copy carrying pw. The original is unchanged.
func (p Profile) WithPassword(pw string) Profile {
	p.password = pw
	return p
}

// Trusts reports whether the user has approved this certificate fingerprint
// for this profile. Comparison ignores case and separators, because the same
// fingerprint gets written several ways.
func (p Profile) Trusts(fingerprint string) bool {
	want := normaliseFingerprint(fingerprint)
	if want == "" {
		return false
	}
	for _, fp := range p.TrustedFingerprints {
		if normaliseFingerprint(fp) == want {
			return true
		}
	}
	return false
}

var fingerprintNoise = strings.NewReplacer(":", "", " ", "", "-", "")

func normaliseFingerprint(s string) string {
	return strings.ToLower(fingerprintNoise.Replace(s))
}
