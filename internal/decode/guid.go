package decode

import (
	"encoding/binary"
	"fmt"
)

// GUID formats a 16-byte objectGUID as a canonical UUID string. The first
// three fields are stored little-endian; the remaining eight bytes are not.
func GUID(b []byte) (string, error) {
	if len(b) != 16 {
		return "", fmt.Errorf("guid: need exactly 16 bytes, got %d", len(b))
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.LittleEndian.Uint32(b[0:4]),
		binary.LittleEndian.Uint16(b[4:6]),
		binary.LittleEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16],
	), nil
}
