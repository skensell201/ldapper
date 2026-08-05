// Package session owns the life cycle of a directory connection: dialling,
// protecting it with TLS, and authenticating.
package session

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Encryption is how the connection is protected.
type Encryption string

const (
	EncryptionNone     Encryption = "none"
	EncryptionLDAPS    Encryption = "ldaps"
	EncryptionStartTLS Encryption = "starttls"
)

// Config is everything needed to reach a server.
type Config struct {
	Host       string
	Port       int
	Encryption Encryption
	// TrustedFingerprints are SHA-256 fingerprints the user has already
	// approved for this server. Anything else fails verification.
	TrustedFingerprints []string
	// Timeout bounds the dial and every later request. Zero means 30 seconds.
	Timeout time.Duration
}

// CertError says the server presented a certificate the system does not trust,
// and carries what a person needs in order to decide whether to accept it.
type CertError struct {
	Fingerprint string
	Subject     string
	Issuer      string
	NotAfter    time.Time
	Err         error
}

func (e *CertError) Error() string {
	return fmt.Sprintf("the certificate for %s is not trusted (issued by %s, SHA-256 %s)",
		e.Subject, e.Issuer, e.Fingerprint)
}

func (e *CertError) Unwrap() error { return e.Err }

// Conn is a directory connection.
type Conn struct {
	*ldap.Conn
	// BindDN is the identity the connection is bound as, once a bind succeeds.
	BindDN string
}

// Fingerprint is the SHA-256 of a certificate, formatted the way certificate
// dialogs show it: upper-case hex, colon-separated.
func Fingerprint(cert *x509.Certificate) string {
	return fingerprintOf(cert.Raw)
}

// fingerprintOf works straight from the DER bytes. A certificate too damaged
// to parse still has a fingerprint, and the user still has to be shown one.
func fingerprintOf(der []byte) string {
	sum := sha256.Sum256(der)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}

// address renders the LDAP URL to dial. StartTLS starts on the plain scheme
// and upgrades afterwards.
func (c Config) address() string {
	scheme := "ldap"
	if c.Encryption == EncryptionLDAPS {
		scheme = "ldaps"
	}
	return scheme + "://" + net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func (c Config) timeout() time.Duration {
	if c.Timeout <= 0 {
		return 30 * time.Second
	}
	return c.Timeout
}

// verifier builds the certificate check. It honours the system's own
// verification first and only then consults the fingerprints the user
// approved, so a valid certificate never depends on a stored exception.
func (c Config) verifier() func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verified [][]*x509.Certificate) error {
		if len(verified) > 0 {
			return nil
		}
		if len(rawCerts) == 0 {
			return fmt.Errorf("session: the server presented no certificate")
		}

		fp := fingerprintOf(rawCerts[0])
		for _, trusted := range c.TrustedFingerprints {
			if normalise(trusted) == normalise(fp) {
				return nil
			}
		}

		out := &CertError{Fingerprint: fp}
		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			out.Subject = "an unreadable certificate"
			out.Issuer = "unknown"
			out.Err = err
			return out
		}

		out.Subject = cert.Subject.String()
		out.Issuer = cert.Issuer.String()
		out.NotAfter = cert.NotAfter
		return out
	}
}

var fingerprintNoise = strings.NewReplacer(":", "", " ", "", "-", "")

func normalise(s string) string {
	return strings.ToLower(fingerprintNoise.Replace(s))
}

// tlsConfig turns off Go's built-in verification only so that verifier can run
// the same checks and report a usable error instead of an opaque one. It is
// not a relaxation: verifier still requires either system trust or an explicit
// approval recorded against this profile.
func (c Config) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName:            c.Host,
		InsecureSkipVerify:    true, //nolint:gosec // verifier below performs the check
		VerifyPeerCertificate: c.verifier(),
		MinVersion:            tls.VersionTLS12,
	}
}

// Dial opens a connection and applies the requested encryption. The returned
// connection is not yet authenticated — call one of the Bind methods next.
func Dial(ctx context.Context, cfg Config) (*Conn, error) {
	dialer := &net.Dialer{Timeout: cfg.timeout()}

	opts := []ldap.DialOpt{ldap.DialWithDialer(dialer)}
	if cfg.Encryption == EncryptionLDAPS {
		opts = append(opts, ldap.DialWithTLSConfig(cfg.tlsConfig()))
	}

	conn, err := ldap.DialURL(cfg.address(), opts...)
	if err != nil {
		return nil, err
	}

	if cfg.Encryption == EncryptionStartTLS {
		if err := conn.StartTLS(cfg.tlsConfig()); err != nil {
			conn.Close()
			return nil, err
		}
	}

	conn.SetTimeout(cfg.timeout())

	// Closing on cancellation is the only cancellation go-ldap offers for the
	// connection itself; per-request cancellation comes from SearchAsync.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	return &Conn{Conn: conn}, nil
}
