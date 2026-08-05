package decode

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// display is the one timestamp layout the interface uses everywhere.
const display = "2006-01-02 15:04:05 UTC"

// ticksPerSecond is how many 100-nanosecond intervals fit in a second.
const ticksPerSecond = 10_000_000

// epochOffset is the gap in seconds between the FILETIME epoch (1601-01-01)
// and the Unix epoch.
const epochOffset = 11_644_473_600

// FileTime renders a Windows FILETIME string. Zero means the attribute was
// never written; the maximum int64 is Active Directory's way of saying the
// deadline never arrives.
func FileTime(s string) (string, error) {
	ticks, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return "", fmt.Errorf("filetime: %q is not an integer", s)
	}
	switch {
	case ticks < 0:
		return "", fmt.Errorf("filetime: %d is negative", ticks)
	case ticks == 0:
		return "not set", nil
	case ticks == 9223372036854775807:
		return "never", nil
	}
	secs := ticks/ticksPerSecond - epochOffset
	return time.Unix(secs, 0).UTC().Format(display), nil
}

// GeneralizedTime renders an RFC 4517 GeneralizedTime in UTC.
func GeneralizedTime(s string) (string, error) {
	s = strings.TrimSpace(s)
	layouts := []string{
		"20060102150405.0Z", "20060102150405Z",
		"20060102150405.0-0700", "20060102150405-0700",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(display), nil
		}
	}
	return "", fmt.Errorf("generalizedtime: %q matches no known layout", s)
}
