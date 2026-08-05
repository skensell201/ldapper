package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

type csvWriter struct {
	w       *csv.Writer
	columns []string
}

// NewCSV returns a Writer emitting one row per entry. The distinguished name
// is always the first column; columns names the attributes after it.
func NewCSV(w io.Writer, columns []string) (Writer, error) {
	if len(columns) == 0 {
		return nil, fmt.Errorf("export: a CSV export needs at least one column")
	}

	cw := csv.NewWriter(w)
	if err := cw.Write(append([]string{"dn"}, columns...)); err != nil {
		return nil, err
	}

	return &csvWriter{w: cw, columns: columns}, nil
}

func (c *csvWriter) Write(dn string, attrs map[string][]string) error {
	row := make([]string, 0, len(c.columns)+1)
	row = append(row, dn)
	for _, name := range c.columns {
		// A multi-valued attribute becomes one field. Semicolon-space keeps
		// it readable in a spreadsheet without colliding with the separator.
		row = append(row, strings.Join(attrs[name], "; "))
	}
	return c.w.Write(row)
}

func (c *csvWriter) Close() error {
	c.w.Flush()
	return c.w.Error()
}
