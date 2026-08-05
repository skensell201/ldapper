package app

import (
	"errors"
	"testing"
	"time"

	"github.com/skensell201/ldapper/internal/session"
)

func TestConnectResultCarriesACertificatePrompt(t *testing.T) {
	certErr := &session.CertError{
		Fingerprint: "4B:9C:1E:07",
		Subject:     "CN=dc01.corp.example.com",
		Issuer:      "CN=CORP-ROOT-CA",
		NotAfter:    time.Date(2027, 2, 11, 0, 0, 0, 0, time.UTC),
	}

	got := connectResultFor(certErr)

	if got.Trusted {
		t.Error("Trusted = true for a certificate nobody has approved")
	}
	if got.Certificate == nil {
		t.Fatal("Certificate is nil; the dialog has nothing to show")
	}
	if got.Certificate.Fingerprint != "4B:9C:1E:07" {
		t.Errorf("Fingerprint = %q", got.Certificate.Fingerprint)
	}
	if got.Certificate.NotAfter != "2027-02-11" {
		t.Errorf("NotAfter = %q, want a plain date", got.Certificate.NotAfter)
	}
	if got.Error != "" {
		t.Errorf("Error = %q, want the certificate reported through Certificate instead", got.Error)
	}
}

func TestConnectResultExplainsOtherFailures(t *testing.T) {
	got := connectResultFor(errors.New("dial tcp 10.0.0.1:636: connect: connection refused"))

	if got.Certificate != nil {
		t.Error("Certificate is set for a failure that has nothing to do with certificates")
	}
	if got.Error == "" {
		t.Error("Error is empty; the user would see nothing at all")
	}
}

func TestConnectResultForSuccess(t *testing.T) {
	got := connectResultFor(nil)
	if !got.Trusted || got.Error != "" || got.Certificate != nil {
		t.Errorf("connectResultFor(nil) = %+v, want a clean success", got)
	}
}

func TestEncryptionMapping(t *testing.T) {
	tests := []struct {
		in   string
		want session.Encryption
	}{
		{"ldaps", session.EncryptionLDAPS},
		{"starttls", session.EncryptionStartTLS},
		{"none", session.EncryptionNone},
		{"", session.EncryptionNone},
		{"nonsense", session.EncryptionNone},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := encryptionOf(tt.in); got != tt.want {
				t.Errorf("encryptionOf(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Connecting to a profile nobody saved must say so rather than dial nothing
// and report a confusing network error.
func TestConnectToAnUnknownProfile(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	got := a.Connect("nobody", "")
	if got.Error == "" {
		t.Error("Error is empty for a profile that was never saved")
	}
	if got.State.Connected {
		t.Error("State says connected after a failed connect")
	}
}

func TestDisconnectIsSafeWhenNothingIsOpen(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()
	a.Disconnect("nobody") // must not panic
}
