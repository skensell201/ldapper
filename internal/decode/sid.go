// Package decode turns the raw attribute values a directory returns into
// something a person can read. Every function here is pure: no network, no
// clock, no filesystem.
package decode

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// SID formats a binary objectSid in the standard S-R-I-S-S… notation.
func SID(b []byte) (string, error) {
	if len(b) < 8 {
		return "", fmt.Errorf("sid: need at least 8 bytes, got %d", len(b))
	}

	revision := b[0]
	subCount := int(b[1])

	if want := 8 + subCount*4; len(b) != want {
		return "", fmt.Errorf("sid: %d sub-authorities need %d bytes, got %d", subCount, want, len(b))
	}

	// The identifier authority is a 48-bit big-endian value in bytes 2..7.
	var authority uint64
	for _, x := range b[2:8] {
		authority = authority<<8 | uint64(x)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "S-%d-%d", revision, authority)
	for i := 0; i < subCount; i++ {
		off := 8 + i*4
		fmt.Fprintf(&sb, "-%d", binary.LittleEndian.Uint32(b[off:off+4]))
	}
	return sb.String(), nil
}
