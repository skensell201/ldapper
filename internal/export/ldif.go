// Package export writes directory entries out in the formats other tools read.
package export

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Writer takes entries one at a time so an export of a large subtree never
// has to be assembled in memory first.
type Writer interface {
	Write(dn string, attrs map[string][]string) error
	Close() error
}

type ldifWriter struct {
	w            *bufio.Writer
	wroteVersion bool
}

// NewLDIF returns a Writer producing RFC 2849 LDIF.
func NewLDIF(w io.Writer) Writer {
	return &ldifWriter{w: bufio.NewWriter(w)}
}

func (l *ldifWriter) Write(dn string, attrs map[string][]string) error {
	if !l.wroteVersion {
		if _, err := l.w.WriteString("version: 1\n\n"); err != nil {
			return err
		}
		l.wroteVersion = true
	}

	if err := l.writeLine("dn", dn); err != nil {
		return err
	}

	// Sort the attribute names so two exports of the same entry are byte
	// identical — otherwise Go's random map order makes them impossible to
	// compare.
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		for _, value := range attrs[name] {
			if err := l.writeLine(name, value); err != nil {
				return err
			}
		}
	}

	_, err := l.w.WriteString("\n")
	return err
}

// writeLine emits one attribute, base64-encoding the value when LDIF requires
// it: anything not printable ASCII, or starting with a character that would
// change how the line parses.
func (l *ldifWriter) writeLine(name, value string) error {
	if needsBase64(value) {
		_, err := fmt.Fprintf(l.w, "%s:: %s\n", name, base64.StdEncoding.EncodeToString([]byte(value)))
		return err
	}
	_, err := fmt.Fprintf(l.w, "%s: %s\n", name, value)
	return err
}

func (l *ldifWriter) Close() error { return l.w.Flush() }

func needsBase64(s string) bool {
	if s == "" {
		return false
	}
	switch s[0] {
	case ' ', ':', '<':
		return true
	}
	if strings.HasSuffix(s, " ") {
		return true
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7E {
			return true
		}
	}
	return false
}
