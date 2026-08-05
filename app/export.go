package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skensell201/ldapper/internal/export"
	"github.com/skensell201/ldapper/internal/filters"
	"github.com/skensell201/ldapper/internal/ldaperr"
	"github.com/skensell201/ldapper/internal/search"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportInput is what the export button sends.
type ExportInput struct {
	ProfileID string   `json:"profileId"`
	Path      string   `json:"path"`
	Filter    string   `json:"filter"`
	Scope     string   `json:"scope"`
	Base      string   `json:"base"`
	Columns   []string `json:"columns"`
}

// ChooseExportPath opens the system's save dialog and returns the chosen path,
// or an empty string if the user cancelled.
func (a *App) ChooseExportPath(suggested string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app: there is no window to open a dialog from")
	}
	return wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		DefaultFilename: suggested,
		Filters: []wruntime.FileFilter{
			{DisplayName: "LDIF (*.ldif)", Pattern: "*.ldif"},
			{DisplayName: "CSV (*.csv)", Pattern: "*.csv"},
		},
	})
}

// ExportSearch runs a search and writes every result to path. It streams
// straight to the file: carrying a subtree through JavaScript and back is
// exactly the case that runs out of memory.
func (a *App) ExportSearch(in ExportInput) string {
	c, ok := a.live(in.ProfileID)
	if !ok {
		return "That connection is not open. Connect first."
	}

	e := filters.Expander{Now: time.Now(), BindDN: a.bindDNOf(in.ProfileID)}
	if err := filters.Validate(in.Filter, e); err != nil {
		return err.Error()
	}
	expanded, err := e.Expand(in.Filter)
	if err != nil {
		return err.Error()
	}

	base := in.Base
	if base == "" {
		base = c.info.RootDN()
	}
	columns := columnsFor(in.Columns)

	f, w, err := writerFor(in.Path, columns)
	if err != nil {
		return err.Error()
	}
	defer func() { _ = f.Close() }()

	_, err = search.Stream(context.Background(), c.conn.Conn, search.Request{
		Base:       base,
		Filter:     expanded,
		Scope:      in.Scope,
		Attributes: columns,
	}, func(batch []search.Result) error {
		for _, r := range batch {
			if err := w.Write(r.DN, flatten(r)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ldaperr.Explain(err)
	}
	if err := w.Close(); err != nil {
		return err.Error()
	}
	return ""
}

// ExportEntry writes one object to a file.
func (a *App) ExportEntry(profileID, dn, path string) string {
	got := a.Entry(profileID, dn)
	if got.Error != "" {
		return got.Error
	}

	columns := make([]string, 0, len(got.Detail.Rows))
	attrs := make(map[string][]string, len(got.Detail.Rows))
	for _, row := range got.Detail.Rows {
		columns = append(columns, row.Name)
		for _, v := range row.Values {
			attrs[row.Name] = append(attrs[row.Name], v.Raw)
		}
	}

	f, w, err := writerFor(path, columns)
	if err != nil {
		return err.Error()
	}
	defer func() { _ = f.Close() }()

	if err := w.Write(got.Detail.DN, attrs); err != nil {
		return err.Error()
	}
	if err := w.Close(); err != nil {
		return err.Error()
	}
	return ""
}

// writerFor opens path and picks the format from its extension.
func writerFor(path string, columns []string) (*os.File, export.Writer, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".ldif" && ext != ".csv" {
		return nil, nil, fmt.Errorf("export: %q is neither .ldif nor .csv", filepath.Base(path))
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("export: cannot write %s: %w", path, err)
	}

	if ext == ".ldif" {
		return f, export.NewLDIF(f), nil
	}

	w, err := export.NewCSV(f, columns)
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	return f, w, nil
}

// flatten drops the decoded renderings: an export carries what the server
// stores, not our reading of it.
func flatten(r search.Result) map[string][]string {
	out := make(map[string][]string, len(r.Attributes))
	for name, values := range r.Attributes {
		for _, v := range values {
			out[name] = append(out[name], v.Raw)
		}
	}
	return out
}
