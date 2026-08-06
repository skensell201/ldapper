// Package schema reads what a server says about itself, so the rest of
// Ldapper can stop guessing which kind of directory it is talking to.
package schema

import (
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/skensell201/ldapper/internal/filters"
)

// Control OIDs worth naming.
const (
	// OIDPagedResults is the simple paged results control. Without it, a
	// server will only ever return its first page.
	OIDPagedResults = "1.2.840.113556.1.4.319"
	// OIDActiveDirectory is advertised by Active Directory and by nothing
	// else in common use, which makes it a reliable way to recognise it.
	//
	// It is a *capability*, not a control: Active Directory publishes it in
	// supportedCapabilities and never in supportedControl. Looking for it in
	// the wrong attribute meant every Active Directory server was reported as
	// plain LDAP, which greyed out all twelve of its filters.
	OIDActiveDirectory = "1.2.840.113556.1.4.800"
)

// Info is what the RootDSE told us.
type Info struct {
	NamingContexts        []string
	DefaultNamingContext  string
	SupportedControls     []string
	SupportedCapabilities []string
	SupportedSASL         []string
	SubschemaSubentry     string
	VendorName            string
	// ObjectClasses is filled in lazily from the subschema; it stays empty
	// until something needs it.
	ObjectClasses []string
}

// IsActiveDirectory reports whether this is an Active Directory server.
//
// The capability list is where the marker actually lives. The control list is
// checked as well, because a server behind a proxy sometimes republishes the
// two together, and being generous here costs nothing.
func (i Info) IsActiveDirectory() bool {
	for _, oid := range append(append([]string{}, i.SupportedCapabilities...), i.SupportedControls...) {
		if oid == OIDActiveDirectory {
			return true
		}
	}
	return false
}

// SupportsPaging reports whether large result sets can be walked a page at a
// time. Without it, browsing a large organizational unit is not possible.
func (i Info) SupportsPaging() bool {
	for _, oid := range i.SupportedControls {
		if oid == OIDPagedResults {
			return true
		}
	}
	return false
}

// RootDN is the branch to open the tree at.
func (i Info) RootDN() string {
	if i.DefaultNamingContext != "" {
		return i.DefaultNamingContext
	}
	if len(i.NamingContexts) > 0 {
		return i.NamingContexts[0]
	}
	return ""
}

// Dialects lists the filter dialects this server can answer.
func (i Info) Dialects() []filters.Dialect {
	out := []filters.Dialect{filters.DialectGeneric}
	if i.IsActiveDirectory() {
		out = append(out, filters.DialectAD)
	}
	for _, class := range i.ObjectClasses {
		if strings.EqualFold(class, "posixAccount") {
			out = append(out, filters.DialectPOSIX)
			break
		}
	}
	return out
}

// Read fetches the RootDSE. Every attribute is optional: servers differ in
// what they publish, and a missing one is a fact, not a failure.
func Read(conn *ldap.Conn) (Info, error) {
	req := ldap.NewSearchRequest(
		"",
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{
			"namingContexts", "defaultNamingContext", "supportedControl",
			"supportedCapabilities", "supportedSASLMechanisms",
			"subschemaSubentry", "vendorName",
		},
		nil,
	)

	res, err := conn.Search(req)
	if err != nil {
		return Info{}, err
	}
	if len(res.Entries) == 0 {
		return Info{}, nil
	}

	e := res.Entries[0]
	return Info{
		NamingContexts:        e.GetAttributeValues("namingContexts"),
		DefaultNamingContext:  e.GetAttributeValue("defaultNamingContext"),
		SupportedControls:     e.GetAttributeValues("supportedControl"),
		SupportedCapabilities: e.GetAttributeValues("supportedCapabilities"),
		SupportedSASL:         e.GetAttributeValues("supportedSASLMechanisms"),
		SubschemaSubentry:     e.GetAttributeValue("subschemaSubentry"),
		VendorName:            e.GetAttributeValue("vendorName"),
	}, nil
}

// ReadObjectClasses fills in Info.ObjectClasses from the subschema entry. It is
// a separate call because the subschema is large and only needed to decide
// whether POSIX filters apply.
func ReadObjectClasses(conn *ldap.Conn, info Info) (Info, error) {
	if info.SubschemaSubentry == "" {
		return info, nil
	}

	req := ldap.NewSearchRequest(
		info.SubschemaSubentry,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=subschema)",
		[]string{"objectClasses"},
		nil,
	)

	res, err := conn.Search(req)
	if err != nil {
		return info, err
	}
	if len(res.Entries) == 0 {
		return info, nil
	}

	for _, def := range res.Entries[0].GetAttributeValues("objectClasses") {
		if name := objectClassName(def); name != "" {
			info.ObjectClasses = append(info.ObjectClasses, name)
		}
	}
	return info, nil
}

// objectClassName pulls the NAME out of a schema definition such as
// ( 1.3.6.1.1.1.2.0 NAME 'posixAccount' SUP top AUXILIARY … ).
func objectClassName(def string) string {
	i := strings.Index(def, "NAME '")
	if i < 0 {
		return ""
	}
	rest := def[i+len("NAME '"):]
	j := strings.IndexByte(rest, '\'')
	if j < 0 {
		return ""
	}
	return rest[:j]
}
