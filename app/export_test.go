package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriterForPicksLDIFFromTheExtension(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"out.ldif", "OUT.LDIF"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			f, w, err := writerFor(path, []string{"cn"})
			if err != nil {
				t.Fatalf("writerFor(%q) returned error: %v", name, err)
			}
			if err := w.Write("cn=x,dc=example,dc=com", map[string][]string{"cn": {"x"}}); err != nil {
				t.Fatalf("Write() returned error: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("Close() returned error: %v", err)
			}
			if err := f.Close(); err != nil {
				t.Fatalf("file Close() returned error: %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(data), "version: 1") {
				t.Errorf("%s is not LDIF:\n%s", name, data)
			}
			if !strings.Contains(string(data), "cn: x") {
				t.Errorf("%s is missing the attribute:\n%s", name, data)
			}
		})
	}
}

func TestWriterForPicksCSVFromTheExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.csv")

	f, w, err := writerFor(path, []string{"cn"})
	if err != nil {
		t.Fatalf("writerFor() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "dn,cn") {
		t.Errorf("out.csv is not CSV:\n%s", data)
	}
}

func TestWriterForRejectsAnUnknownExtension(t *testing.T) {
	for _, name := range []string{"out.txt", "out", "out.json"} {
		if _, _, err := writerFor(filepath.Join(t.TempDir(), name), []string{"cn"}); err == nil {
			t.Errorf("writerFor(%q) accepted an extension it cannot write", name)
		}
	}
}

// A path that cannot be created has to say so rather than leave a half-open
// file behind.
func TestWriterForReportsAnUnwritablePath(t *testing.T) {
	if _, _, err := writerFor("/no/such/directory/out.ldif", []string{"cn"}); err == nil {
		t.Error("writerFor() succeeded on a path that cannot be created")
	}
}

func TestExportWithoutAConnection(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if msg := a.ExportSearch(ExportInput{
		ProfileID: "nobody",
		Path:      filepath.Join(t.TempDir(), "x.ldif"),
	}); msg == "" {
		t.Error("ExportSearch() succeeded with no connection open")
	}
	if msg := a.ExportEntry("nobody", "dc=example,dc=com", filepath.Join(t.TempDir(), "x.ldif")); msg == "" {
		t.Error("ExportEntry() succeeded with no connection open")
	}
}

func TestChooseExportPathWithoutAWindow(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	if _, err := a.ChooseExportPath("results.ldif"); err == nil {
		t.Error("ChooseExportPath() succeeded with no window to open a dialog from")
	}
}
