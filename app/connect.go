package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/skensell201/ldapper/internal/ldaperr"
	"github.com/skensell201/ldapper/internal/profiles"
	"github.com/skensell201/ldapper/internal/schema"
	"github.com/skensell201/ldapper/internal/session"
)

// ConnectResult is what the connect dialog acts on.
type ConnectResult struct {
	// Trusted is false when the server's certificate needs a decision. The
	// dialog shows Certificate and, if the user accepts, calls Connect again.
	Trusted bool `json:"trusted"`
	// Certificate is set only when Trusted is false.
	Certificate *CertPrompt `json:"certificate,omitempty"`
	// Error is a sentence, already translated out of LDAP result codes.
	Error string          `json:"error,omitempty"`
	State ConnectionState `json:"state"`
}

// Connect opens a connection for a saved profile and reads what the server
// says about itself.
func (a *App) Connect(profileID, password string) ConnectResult {
	p, ok := a.profiles.Get(profileID)
	if !ok {
		return ConnectResult{Trusted: true, Error: fmt.Sprintf("There is no saved connection called %q.", profileID)}
	}

	if password == "" {
		// A missing keychain is not fatal here: the dialog asks for the
		// password instead, and the bind below reports an empty one.
		if loaded, err := a.profiles.LoadPassword(p); err == nil {
			p = loaded
		}
	} else {
		p = p.WithPassword(password)
	}

	// The connection outlives this call, so its context is cancelled by
	// Disconnect or Shutdown rather than by returning from here.
	ctx, cancel := context.WithCancel(context.Background())

	conn, err := session.Dial(ctx, session.Config{
		Host:                p.Host,
		Port:                p.Port,
		Encryption:          encryptionOf(string(p.Encryption)),
		TrustedFingerprints: p.TrustedFingerprints,
	})
	if err != nil {
		cancel()
		return connectResultFor(err)
	}

	switch p.BindMethod {
	case profiles.BindAnonymous:
		err = conn.BindAnonymous()
	case profiles.BindNTLM:
		domain, account := session.SplitAccount(p.Username)
		if domain == "" {
			domain = p.Domain
		}
		err = conn.BindNTLM(domain, account, p.Password())
	default:
		err = conn.BindSimple(p.Username, p.Password())
	}
	if err != nil {
		cancel()
		return connectResultFor(err)
	}

	info, err := schema.Read(conn.Conn)
	if err != nil {
		cancel()
		return connectResultFor(err)
	}
	// The object classes are only needed to decide whether POSIX filters
	// apply; a server that will not hand over its subschema is not a failure.
	if full, err := schema.ReadObjectClasses(conn.Conn, info); err == nil {
		info = full
	}

	dialects := []string{}
	for _, d := range info.Dialects() {
		dialects = append(dialects, string(d))
	}

	state := ConnectionState{
		ProfileID:         p.ID,
		Connected:         true,
		Host:              p.Host,
		BoundAs:           conn.BindDN,
		RootDN:            info.RootDN(),
		IsActiveDirectory: info.IsActiveDirectory(),
		SupportsPaging:    info.SupportsPaging(),
		Dialects:          dialects,
		Encryption:        string(p.Encryption),
	}

	a.setLive(p.ID, &liveConn{conn: conn, info: info, state: state, cancel: cancel})

	result := connectResultFor(nil)
	result.State = state
	return result
}

// TrustCertificate records a fingerprint against one profile. The dialog calls
// it and then calls Connect again.
func (a *App) TrustCertificate(profileID, fingerprint string) string {
	if err := a.profiles.Trust(profileID, fingerprint); err != nil {
		return err.Error()
	}
	return ""
}

// Disconnect closes one connection.
func (a *App) Disconnect(profileID string) { a.dropLive(profileID) }

// connectResultFor turns whatever went wrong into something the interface can
// act on: a certificate decision, a sentence, or nothing at all.
func connectResultFor(err error) ConnectResult {
	if err == nil {
		return ConnectResult{Trusted: true}
	}

	var certErr *session.CertError
	if errors.As(err, &certErr) {
		return ConnectResult{
			Certificate: &CertPrompt{
				Fingerprint: certErr.Fingerprint,
				Subject:     certErr.Subject,
				Issuer:      certErr.Issuer,
				NotAfter:    certErr.NotAfter.Format("2006-01-02"),
			},
		}
	}

	return ConnectResult{Trusted: true, Error: ldaperr.Explain(err)}
}

func encryptionOf(s string) session.Encryption {
	switch s {
	case string(session.EncryptionLDAPS):
		return session.EncryptionLDAPS
	case string(session.EncryptionStartTLS):
		return session.EncryptionStartTLS
	default:
		return session.EncryptionNone
	}
}
