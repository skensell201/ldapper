package export

import (
	"bytes"
	"strings"
	"testing"
)

func TestLDIFWritesAnEntry(t *testing.T) {
	var buf bytes.Buffer
	w := NewLDIF(&buf)

	err := w.Write("CN=Anna Volkova,OU=Users,DC=example,DC=com", map[string][]string{
		"cn":          {"Anna Volkova"},
		"objectClass": {"top", "person", "user"},
	})
	if err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "dn: CN=Anna Volkova,OU=Users,DC=example,DC=com\n") {
		t.Errorf("output has no dn line:\n%s", got)
	}
	for _, want := range []string{"cn: Anna Volkova\n", "objectClass: top\n", "objectClass: user\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Error("entries must be separated by a blank line")
	}
}

func TestLDIFPutsTheDNFirst(t *testing.T) {
	var buf bytes.Buffer
	w := NewLDIF(&buf)
	if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{"aardvark": {"first alphabetically"}}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// version header, blank, dn, attribute
	if lines[2] != "dn: CN=x,DC=example,DC=com" {
		t.Errorf("line 3 = %q, want the dn before any attribute", lines[2])
	}
}

func TestLDIFBase64EncodesValuesThatNeedIt(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"non-ASCII", "Анна Волкова", "cn:: "},
		{"leading space", " padded", "cn:: "},
		{"leading colon", ":starts with colon", "cn:: "},
		{"leading less-than", "<url-ish", "cn:: "},
		{"trailing space", "padded ", "cn:: "},
		{"embedded newline", "two\nlines", "cn:: "},
		{"plain ASCII stays plain", "Anna Volkova", "cn: "},
		{"empty stays plain", "", "cn: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewLDIF(&buf)
			if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{"cn": {tt.value}}); err != nil {
				t.Fatalf("Write() returned error: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("Close() returned error: %v", err)
			}
			if !strings.Contains(buf.String(), tt.want) {
				t.Errorf("output %q does not contain %q", buf.String(), tt.want)
			}
		})
	}
}

func TestLDIFBase64EncodesADNThatNeedsIt(t *testing.T) {
	var buf bytes.Buffer
	w := NewLDIF(&buf)
	if err := w.Write("CN=Анна,DC=example,DC=com", nil); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "dn:: ") {
		t.Errorf("a non-ASCII DN must be base64-encoded, got:\n%s", buf.String())
	}
}

func TestLDIFIsDeterministic(t *testing.T) {
	// Go randomises map iteration. Two exports of the same entry must still
	// produce identical bytes, or diffing two exports is useless.
	attrs := map[string][]string{"zebra": {"z"}, "alpha": {"a"}, "middle": {"m"}}

	var first, second bytes.Buffer
	for _, buf := range []*bytes.Buffer{&first, &second} {
		w := NewLDIF(buf)
		if err := w.Write("CN=x,DC=example,DC=com", attrs); err != nil {
			t.Fatalf("Write() returned error: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close() returned error: %v", err)
		}
	}
	if first.String() != second.String() {
		t.Errorf("two exports of the same entry differ:\n%s\n---\n%s", first.String(), second.String())
	}
	if !strings.Contains(first.String(), "alpha: a\nmiddle: m\nzebra: z\n") {
		t.Errorf("attributes are not in sorted order:\n%s", first.String())
	}
}

func TestLDIFWritesTheVersionHeaderOnce(t *testing.T) {
	var buf bytes.Buffer
	w := NewLDIF(&buf)
	for i := 0; i < 3; i++ {
		if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{"cn": {"x"}}); err != nil {
			t.Fatalf("Write() returned error: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if n := strings.Count(buf.String(), "version: 1"); n != 1 {
		t.Errorf("found %d version headers, want exactly 1", n)
	}
}
