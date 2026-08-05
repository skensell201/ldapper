package filters

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Expander turns a filter template into a filter a server will accept.
type Expander struct {
	// Now is the instant {{now}} resolves to. Always set it explicitly:
	// a substitution that reads the clock on its own cannot be tested.
	Now time.Time
	// BindDN is the distinguished name {{me}} resolves to. Empty means the
	// connection has no bound identity, and {{me}} becomes an error.
	BindDN string
}

// substitution matches {{now}}, {{now-90d}}, {{now+1d:filetime}} and {{me}}.
var substitution = regexp.MustCompile(`\{\{([^}]*)\}\}`)

// offset splits "now-90d" into its sign, amount and unit.
var offset = regexp.MustCompile(`^now(?:([+-])(\d+)([dhm]))?$`)

const (
	ticksPerSecond = 10_000_000
	epochOffset    = 11_644_473_600
)

// Expand replaces every {{…}} in filter. An unrecognised substitution is an
// error: sending it to the server verbatim would silently return nothing.
func (e Expander) Expand(filter string) (string, error) {
	var firstErr error

	out := substitution.ReplaceAllStringFunc(filter, func(match string) string {
		body := strings.TrimSpace(match[2 : len(match)-2])

		name, format := body, ""
		if i := strings.IndexByte(body, ':'); i >= 0 {
			name, format = body[:i], body[i+1:]
		}

		value, err := e.resolve(name, format)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return value
	})

	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

func (e Expander) resolve(name, format string) (string, error) {
	if name == "me" {
		if format != "" {
			return "", fmt.Errorf("filters: {{me}} takes no format, got %q", format)
		}
		if e.BindDN == "" {
			return "", fmt.Errorf("filters: {{me}} needs a bound connection")
		}
		return e.BindDN, nil
	}

	m := offset.FindStringSubmatch(name)
	if m == nil {
		return "", fmt.Errorf("filters: unknown substitution {{%s}}", name)
	}

	t := e.Now
	if m[1] != "" {
		amount, err := strconv.Atoi(m[2])
		if err != nil {
			return "", fmt.Errorf("filters: %q is not a number", m[2])
		}
		var d time.Duration
		switch m[3] {
		case "d":
			d = time.Duration(amount) * 24 * time.Hour
		case "h":
			d = time.Duration(amount) * time.Hour
		case "m":
			d = time.Duration(amount) * time.Minute
		}
		if m[1] == "-" {
			d = -d
		}
		t = t.Add(d)
	}

	switch format {
	case "", "generalized":
		return t.UTC().Format("20060102150405Z"), nil
	case "filetime":
		return strconv.FormatInt((t.UTC().Unix()+epochOffset)*ticksPerSecond, 10), nil
	default:
		return "", fmt.Errorf("filters: unknown format %q, want filetime or generalized", format)
	}
}
