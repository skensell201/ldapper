// Command ldapper-probe exercises the Ldapper engine from a terminal. It is a
// development tool, not a product: it exists so the engine can be pointed at a
// real directory before any interface is built.
//
// There are no tests for this file. It is argument parsing and printing, and
// everything it calls is covered in internal/ — anything here worth testing
// belongs in a package, not in main.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/skensell201/ldapper/internal/browse"
	"github.com/skensell201/ldapper/internal/export"
	"github.com/skensell201/ldapper/internal/filters"
	"github.com/skensell201/ldapper/internal/ldaperr"
	"github.com/skensell201/ldapper/internal/schema"
	"github.com/skensell201/ldapper/internal/search"
	"github.com/skensell201/ldapper/internal/session"
)

type options struct {
	host, encryption      string
	port                  int
	user, password, trust string
	ntlm                  bool
	base, filter, scope   string
	attributes            string
	pageSize              uint
	asLDIF, asCSV         bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", ldaperr.Explain(err))
		os.Exit(1)
	}
}

func run() error {
	var o options
	flag.StringVar(&o.host, "host", "localhost", "directory host")
	flag.IntVar(&o.port, "port", 389, "directory port")
	flag.StringVar(&o.encryption, "encryption", "none", "none, ldaps or starttls")
	flag.StringVar(&o.user, "user", "", `bind DN, UPN or DOMAIN\account`)
	flag.StringVar(&o.password, "password", "", "bind password")
	flag.BoolVar(&o.ntlm, "ntlm", false, "use an NTLM bind")
	flag.StringVar(&o.trust, "trust", "", "certificate fingerprint to accept")
	flag.StringVar(&o.base, "base", "", "search base; defaults to the server's own")
	flag.StringVar(&o.filter, "filter", "(objectClass=*)", "LDAP filter, {{now-90d:filetime}} and friends allowed")
	flag.StringVar(&o.scope, "scope", "subtree", "base, one or subtree")
	flag.StringVar(&o.attributes, "attributes", "", "comma-separated attributes to fetch")
	flag.UintVar(&o.pageSize, "pagesize", 0, "entries per page when browsing; 0 uses the 1000 Active Directory defaults to")
	flag.BoolVar(&o.asLDIF, "ldif", false, "write results as LDIF")
	flag.BoolVar(&o.asCSV, "csv", false, "write results as CSV")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ldapper-probe [flags] <command>")
		fmt.Fprintln(os.Stderr, "\ncommands:")
		fmt.Fprintln(os.Stderr, "  info      what the server says about itself")
		fmt.Fprintln(os.Stderr, "  browse    list the children of -base, one page at a time")
		fmt.Fprintln(os.Stderr, "  search    run -filter and print or export the results")
		fmt.Fprintln(os.Stderr, "  filters   list the built-in filters; needs no connection")
		fmt.Fprintln(os.Stderr, "\nflags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	command := flag.Arg(0)
	if command == "" {
		flag.Usage()
		return errors.New("no command given")
	}

	// filters needs no connection at all.
	if command == "filters" {
		return listFilters()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	conn, err := connect(ctx, o)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	info, err := schema.Read(conn.Conn)
	if err != nil {
		return err
	}
	if o.base == "" {
		o.base = info.RootDN()
	}

	switch command {
	case "info":
		return printInfo(conn, info)
	case "browse":
		return printChildren(conn, o.base, uint32(o.pageSize))
	case "search":
		return runSearch(ctx, conn, o)
	default:
		return fmt.Errorf("unknown command %q; try info, browse, search or filters", command)
	}
}

// connect dials and binds, turning an untrusted certificate into the report a
// person needs rather than an opaque TLS failure.
func connect(ctx context.Context, o options) (*session.Conn, error) {
	cfg := session.Config{
		Host:       o.host,
		Port:       o.port,
		Encryption: session.Encryption(o.encryption),
	}
	if o.trust != "" {
		cfg.TrustedFingerprints = []string{o.trust}
	}

	conn, err := session.Dial(ctx, cfg)
	if err != nil {
		var certErr *session.CertError
		if errors.As(err, &certErr) {
			fmt.Fprintf(os.Stderr,
				"untrusted certificate\n  subject:  %s\n  issuer:   %s\n  SHA-256:  %s\n  expires:  %s\n\nrerun with -trust %s to accept it\n",
				certErr.Subject, certErr.Issuer, certErr.Fingerprint,
				certErr.NotAfter.Format("2006-01-02"), certErr.Fingerprint)
			os.Exit(2)
		}
		return nil, err
	}

	switch {
	case o.user == "":
		err = conn.BindAnonymous()
	case o.ntlm:
		domain, account := session.SplitAccount(o.user)
		err = conn.BindNTLM(domain, account, o.password)
	default:
		err = conn.BindSimple(o.user, o.password)
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func printInfo(conn *session.Conn, info schema.Info) error {
	full, err := schema.ReadObjectClasses(conn.Conn, info)
	if err != nil {
		return err
	}

	var dialects []string
	for _, d := range full.Dialects() {
		dialects = append(dialects, string(d))
	}

	fmt.Println("vendor:           ", orNone(full.VendorName))
	fmt.Println("root DN:          ", orNone(full.RootDN()))
	fmt.Println("naming contexts:  ", orNone(strings.Join(full.NamingContexts, ", ")))
	fmt.Println("active directory: ", full.IsActiveDirectory())
	fmt.Println("paged results:    ", full.SupportsPaging())
	fmt.Println("bound as:         ", orNone(conn.BindDN))
	fmt.Println("filter dialects:  ", strings.Join(dialects, ", "))
	return nil
}

func printChildren(conn *session.Conn, base string, size uint32) error {
	var (
		cookie []byte
		page   int
		total  int
	)

	for {
		page++
		result, err := browse.Children(conn.Conn, base, size, cookie)
		if err != nil {
			return err
		}
		for _, e := range result.Entries {
			children := "?"
			if e.NumSubordinates >= 0 {
				children = fmt.Sprint(e.NumSubordinates)
			}
			fmt.Printf("%-52s  %-28s  children=%s\n", e.RDN, strings.Join(e.Classes, "/"), children)
		}
		total += len(result.Entries)
		fmt.Fprintf(os.Stderr, "-- page %d: %d entries\n", page, len(result.Entries))

		if len(result.Cookie) == 0 {
			fmt.Fprintf(os.Stderr, "-- %d entries across %d pages\n", total, page)
			return nil
		}
		cookie = result.Cookie
	}
}

func runSearch(ctx context.Context, conn *session.Conn, o options) error {
	var attrList []string
	if o.attributes != "" {
		attrList = strings.Split(o.attributes, ",")
	}

	e := filters.Expander{Now: time.Now(), BindDN: conn.BindDN}
	if err := filters.Validate(o.filter, e); err != nil {
		return err
	}
	expanded, err := e.Expand(o.filter)
	if err != nil {
		return err
	}
	if expanded != o.filter {
		fmt.Fprintf(os.Stderr, "-- filter expands to %s\n", expanded)
	}

	writer, err := newWriter(o, attrList)
	if err != nil {
		return err
	}

	stats, err := search.Stream(ctx, conn.Conn, search.Request{
		Base:       o.base,
		Filter:     expanded,
		Scope:      o.scope,
		Attributes: attrList,
	}, func(batch []search.Result) error {
		for _, r := range batch {
			if writer == nil {
				fmt.Println(r.DN)
				continue
			}
			if err := writer.Write(r.DN, flatten(r)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if writer != nil {
		if err := writer.Close(); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "-- matched %d\n", stats.Matched)
	if stats.Truncated {
		fmt.Fprintln(os.Stderr, "--", stats.TruncateReason)
	}
	return nil
}

func newWriter(o options, attrList []string) (export.Writer, error) {
	switch {
	case o.asLDIF:
		return export.NewLDIF(os.Stdout), nil
	case o.asCSV:
		columns := attrList
		if len(columns) == 0 {
			columns = []string{"cn"}
		}
		return export.NewCSV(os.Stdout, columns)
	default:
		return nil, nil
	}
}

func listFilters() error {
	for _, f := range filters.Builtins() {
		fmt.Printf("%-28s  %-8s  %s\n", f.ID, f.Dialect, f.Name)
		fmt.Printf("%30s%s\n", "", f.Filter)
	}
	return nil
}

// flatten drops the decoded renderings: an export carries what the server
// actually stores, not our reading of it.
func flatten(r search.Result) map[string][]string {
	out := make(map[string][]string, len(r.Attributes))
	for name, values := range r.Attributes {
		for _, v := range values {
			out[name] = append(out[name], v.Raw)
		}
	}
	return out
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
