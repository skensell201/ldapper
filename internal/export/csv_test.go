package export

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

func TestCSVWritesAHeaderAndRows(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewCSV(&buf, []string{"cn", "mail"})
	if err != nil {
		t.Fatalf("NewCSV() returned error: %v", err)
	}

	if err := w.Write("CN=Anna,DC=example,DC=com", map[string][]string{
		"cn":   {"Anna Volkova"},
		"mail": {"a.volkova@example.com"},
	}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\r\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want a header and one row:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "dn,cn,mail") {
		t.Errorf("header = %q, want dn first, then the requested columns", lines[0])
	}
	if !strings.Contains(lines[1], "Anna Volkova") {
		t.Errorf("row = %q, want the cn value", lines[1])
	}
}

func TestCSVJoinsMultipleValues(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewCSV(&buf, []string{"objectClass"})
	if err != nil {
		t.Fatalf("NewCSV() returned error: %v", err)
	}
	if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{
		"objectClass": {"top", "person", "user"},
	}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "\"top\nperson\nuser\"") {
		t.Errorf("multi-valued attribute not joined by newlines:\n%q", buf.String())
	}
}

// A single value containing the separator must not read like several values.
// This is the case that made semicolons unusable.
func TestCSVKeepsASemicolonInsideAValue(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewCSV(&buf, []string{"description"})
	if err != nil {
		t.Fatalf("NewCSV() returned error: %v", err)
	}
	if err := w.Write("cn=x,dc=example,dc=com", map[string][]string{
		"description": {"A value with; several; semicolons in it."},
	}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	// Two newlines: one ending the header, one ending the row. A third would
	// mean the value had been split.
	got := buf.String()
	if strings.Count(got, "\n") != 2 {
		t.Errorf("the single value was split across lines:\n%q", got)
	}
	if !strings.Contains(got, "A value with; several; semicolons in it.") {
		t.Errorf("the value did not survive:\n%q", got)
	}
}

// Reading the file back with a CSV reader has to produce exactly what went in.
func TestCSVRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewCSV(&buf, []string{"mail", "description"})
	if err != nil {
		t.Fatalf("NewCSV() returned error: %v", err)
	}
	if err := w.Write("cn=x,dc=example,dc=com", map[string][]string{
		"mail":        {"one@example.com", "two@example.com"},
		"description": {"has; a semicolon"},
	}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("the file cannot be read back: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want a header and one row", len(records))
	}
	if records[1][1] != "one@example.com\ntwo@example.com" {
		t.Errorf("mail = %q, want both values separated by a newline", records[1][1])
	}
	if records[1][2] != "has; a semicolon" {
		t.Errorf("description = %q, want the semicolon left alone", records[1][2])
	}
}

func TestCSVLeavesMissingAttributesEmpty(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewCSV(&buf, []string{"cn", "telephoneNumber"})
	if err != nil {
		t.Fatalf("NewCSV() returned error: %v", err)
	}
	if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{"cn": {"x"}}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if !strings.HasSuffix(strings.TrimRight(buf.String(), "\r\n"), ",x,") {
		t.Errorf("a missing attribute must produce an empty field:\n%s", buf.String())
	}
}

func TestCSVQuotesFieldsContainingSeparators(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewCSV(&buf, []string{"cn"})
	if err != nil {
		t.Fatalf("NewCSV() returned error: %v", err)
	}
	if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{
		"cn": {`Volkova, Anna`},
	}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if !strings.Contains(buf.String(), `"Volkova, Anna"`) {
		t.Errorf("a value containing a comma must be quoted:\n%s", buf.String())
	}
}

func TestCSVRejectsAnEmptyColumnList(t *testing.T) {
	var buf bytes.Buffer
	if _, err := NewCSV(&buf, nil); err == nil {
		t.Error("NewCSV() with no columns succeeded, want an error")
	}
}

// Both writers satisfy the same interface, so a caller can pick a format
// without changing anything else.
func TestBothWritersShareOneInterface(t *testing.T) {
	var buf bytes.Buffer
	csv, err := NewCSV(&buf, []string{"cn"})
	if err != nil {
		t.Fatalf("NewCSV() returned error: %v", err)
	}
	writers := []Writer{NewLDIF(&buf), csv}
	for _, w := range writers {
		if w == nil {
			t.Error("a writer is nil")
		}
	}
}
