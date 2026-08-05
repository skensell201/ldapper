package session

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

// selfSigned builds a real certificate so the verifier is exercised against
// DER a server could actually present, not a stand-in byte slice.
func selfSigned(t *testing.T, commonName string) (der []byte, cert *x509.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("cannot generate a key: %v", err)
	}
	// The issuer comes from the parent's subject, not from template.Issuer,
	// so the two templates have to be separate for the test to see a
	// realistic internal-CA certificate.
	parent := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "CORP-ROOT-CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(48 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err = x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("cannot create a certificate: %v", err)
	}
	cert, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("cannot parse the certificate just created: %v", err)
	}
	return der, cert
}

func TestAddressBuildsTheRightURL(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{"plain", Config{Host: "dc01.example.com", Port: 389, Encryption: EncryptionNone}, "ldap://dc01.example.com:389"},
		{"ldaps", Config{Host: "dc01.example.com", Port: 636, Encryption: EncryptionLDAPS}, "ldaps://dc01.example.com:636"},
		{"starttls dials plain first", Config{Host: "dc01.example.com", Port: 389, Encryption: EncryptionStartTLS}, "ldap://dc01.example.com:389"},
		{"ipv6 is bracketed", Config{Host: "2001:db8::1", Port: 636, Encryption: EncryptionLDAPS}, "ldaps://[2001:db8::1]:636"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.address(); got != tt.want {
				t.Errorf("address() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFingerprintFormat(t *testing.T) {
	_, cert := selfSigned(t, "dc01.corp.example.com")
	got := Fingerprint(cert)

	if len(got) != 95 { // 32 bytes → 64 hex characters + 31 colons
		t.Errorf("Fingerprint() = %q (%d chars), want 95", got, len(got))
	}
	if strings.ToUpper(got) != got {
		t.Errorf("Fingerprint() = %q, want upper case", got)
	}
	if strings.Count(got, ":") != 31 {
		t.Errorf("Fingerprint() = %q, want 31 separators", got)
	}
}

func TestVerifierAcceptsAVerifiedChain(t *testing.T) {
	der, _ := selfSigned(t, "dc01.corp.example.com")
	verify := Config{}.verifier()

	// A non-empty verified chain means the system already trusts it.
	if err := verify([][]byte{der}, [][]*x509.Certificate{{}}); err != nil {
		t.Errorf("verifier() rejected a chain the system verified: %v", err)
	}
}

func TestVerifierAcceptsAPreviouslyTrustedFingerprint(t *testing.T) {
	der, cert := selfSigned(t, "dc01.corp.example.com")
	cfg := Config{TrustedFingerprints: []string{Fingerprint(cert)}}

	if err := cfg.verifier()([][]byte{der}, nil); err != nil {
		t.Errorf("verifier() rejected a trusted fingerprint: %v", err)
	}
}

func TestVerifierIgnoresFingerprintFormatting(t *testing.T) {
	der, cert := selfSigned(t, "dc01.corp.example.com")
	stored := strings.ToLower(strings.ReplaceAll(Fingerprint(cert), ":", ""))
	cfg := Config{TrustedFingerprints: []string{stored}}

	if err := cfg.verifier()([][]byte{der}, nil); err != nil {
		t.Errorf("verifier() rejected a fingerprint stored without separators: %v", err)
	}
}

func TestVerifierReportsAnUntrustedCertificate(t *testing.T) {
	der, cert := selfSigned(t, "dc01.corp.example.com")

	err := Config{}.verifier()([][]byte{der}, nil)
	if err == nil {
		t.Fatal("verifier() accepted an untrusted certificate")
	}

	var certErr *CertError
	if !errors.As(err, &certErr) {
		t.Fatalf("verifier() returned %T, want a *CertError", err)
	}
	if certErr.Fingerprint != Fingerprint(cert) {
		t.Errorf("Fingerprint = %q, want %q", certErr.Fingerprint, Fingerprint(cert))
	}
	if !strings.Contains(certErr.Subject, "dc01.corp.example.com") {
		t.Errorf("Subject = %q, want the server name", certErr.Subject)
	}
	if !strings.Contains(certErr.Issuer, "CORP-ROOT-CA") {
		t.Errorf("Issuer = %q, want the issuing authority", certErr.Issuer)
	}
	if certErr.NotAfter.IsZero() {
		t.Error("NotAfter is zero; the expiry is part of what the user is being asked to judge")
	}
}

// A certificate that will not parse still has a fingerprint, and the user
// still has to be told rather than shown a blank dialog.
func TestVerifierReportsUnparsableCertificates(t *testing.T) {
	err := Config{}.verifier()([][]byte{{0xFF, 0xFE}}, nil)
	if err == nil {
		t.Fatal("verifier() accepted a certificate it could not parse")
	}

	var certErr *CertError
	if !errors.As(err, &certErr) {
		t.Fatalf("verifier() returned %T, want a *CertError", err)
	}
	if certErr.Fingerprint == "" {
		t.Error("Fingerprint is empty; it is computed from the bytes and does not need parsing")
	}
}

func TestVerifierRejectsAnEmptyChain(t *testing.T) {
	if err := (Config{}).verifier()(nil, nil); err == nil {
		t.Error("verifier() accepted a handshake with no certificate at all")
	}
}

func TestCertErrorReadsAsASentence(t *testing.T) {
	err := &CertError{
		Fingerprint: "4B:9C:1E:07",
		Subject:     "CN=dc01.corp.example.com",
		Issuer:      "CN=CORP-ROOT-CA",
	}
	msg := err.Error()
	for _, want := range []string{"4B:9C:1E:07", "dc01.corp.example.com", "CORP-ROOT-CA"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to include %q", msg, want)
		}
	}
}

func TestTimeoutHasADefault(t *testing.T) {
	if got := (Config{}).timeout(); got <= 0 {
		t.Errorf("timeout() = %v, want a positive default", got)
	}
	if got := (Config{Timeout: 5 * time.Second}).timeout(); got != 5*time.Second {
		t.Errorf("timeout() = %v, want the configured value", got)
	}
}

func TestDialRefusesAnUnreachableHost(t *testing.T) {
	cfg := Config{Host: "127.0.0.1", Port: 1, Encryption: EncryptionNone, Timeout: 2 * time.Second}
	if _, err := Dial(context.Background(), cfg); err == nil {
		t.Error("Dial() to a closed port succeeded, want an error")
	}
}
