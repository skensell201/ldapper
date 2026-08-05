package session

import (
	"fmt"
	"strings"
)

// SplitAccount separates a down-level logon name such as CORP\a.kensel into
// its domain and account. A user principal name is returned untouched: the
// part after the @ is a realm, not a NetBIOS domain.
func SplitAccount(s string) (domain, user string) {
	s = strings.TrimSpace(s)
	for _, sep := range []string{`\`, "/"} {
		if i := strings.Index(s, sep); i >= 0 {
			return s[:i], s[i+1:]
		}
	}
	return "", s
}

// BindSimple authenticates with a DN or user principal name and a password.
func (c *Conn) BindSimple(username, password string) error {
	if username == "" {
		return fmt.Errorf("session: a simple bind needs a username")
	}
	if password == "" {
		return fmt.Errorf("session: a simple bind needs a password — an empty one signs in anonymously and shows an empty directory")
	}
	if err := c.Conn.Bind(username, password); err != nil {
		return err
	}
	c.BindDN = username
	return nil
}

// BindNTLM authenticates the way an administrator thinks about their own
// account: domain, account name and password, with no distinguished name to
// look up first.
func (c *Conn) BindNTLM(domain, username, password string) error {
	if username == "" {
		return fmt.Errorf("session: an NTLM bind needs a username")
	}
	if password == "" {
		return fmt.Errorf("session: an NTLM bind needs a password")
	}
	// Accept CORP\a.kensel in the username field as well as in its own box.
	if d, u := SplitAccount(username); d != "" {
		domain, username = d, u
	}
	if domain == "" {
		return fmt.Errorf(`session: an NTLM bind needs a domain, either on its own or as DOMAIN\user`)
	}
	if err := c.Conn.NTLMBind(domain, username, password); err != nil {
		return err
	}
	c.BindDN = domain + `\` + username
	return nil
}

// BindAnonymous connects without credentials. Some directories allow it for
// reading public entries.
func (c *Conn) BindAnonymous() error {
	if err := c.Conn.UnauthenticatedBind(""); err != nil {
		return err
	}
	c.BindDN = ""
	return nil
}
