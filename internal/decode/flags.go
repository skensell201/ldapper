package decode

import (
	"fmt"
	"strconv"
	"strings"
)

// bit pairs a single flag value with its name.
type bit struct {
	value uint32
	name  string
}

// uacBits is ordered by bit value, and splitBits walks it in order. That gives
// a deterministic result without sorting, and it puts the base account type
// ahead of the modifiers that qualify it — NORMAL_ACCOUNT before
// DONT_EXPIRE_PASSWORD, which is the order an administrator reads.
var uacBits = []bit{
	{0x00000001, "SCRIPT"},
	{0x00000002, "ACCOUNTDISABLE"},
	{0x00000008, "HOMEDIR_REQUIRED"},
	{0x00000010, "LOCKOUT"},
	{0x00000020, "PASSWD_NOTREQD"},
	{0x00000040, "PASSWD_CANT_CHANGE"},
	{0x00000080, "ENCRYPTED_TEXT_PWD_ALLOWED"},
	{0x00000100, "TEMP_DUPLICATE_ACCOUNT"},
	{0x00000200, "NORMAL_ACCOUNT"},
	{0x00000800, "INTERDOMAIN_TRUST_ACCOUNT"},
	{0x00001000, "WORKSTATION_TRUST_ACCOUNT"},
	{0x00002000, "SERVER_TRUST_ACCOUNT"},
	{0x00010000, "DONT_EXPIRE_PASSWORD"},
	{0x00020000, "MNS_LOGON_ACCOUNT"},
	{0x00040000, "SMARTCARD_REQUIRED"},
	{0x00080000, "TRUSTED_FOR_DELEGATION"},
	{0x00100000, "NOT_DELEGATED"},
	{0x00200000, "USE_DES_KEY_ONLY"},
	{0x00400000, "DONT_REQ_PREAUTH"},
	{0x00800000, "PASSWORD_EXPIRED"},
	{0x01000000, "TRUSTED_TO_AUTH_FOR_DELEGATION"},
	{0x04000000, "PARTIAL_SECRETS_ACCOUNT"},
}

var groupTypeBits = []bit{
	{0x00000001, "BUILTIN_LOCAL"},
	{0x00000002, "GLOBAL"},
	{0x00000004, "DOMAIN_LOCAL"},
	{0x00000008, "UNIVERSAL"},
	{0x00000010, "APP_BASIC"},
	{0x00000020, "APP_QUERY"},
	{0x80000000, "SECURITY"},
}

var samAccountTypes = map[uint32]string{
	0x00000000: "SAM_DOMAIN_OBJECT",
	0x10000000: "SAM_GROUP_OBJECT",
	0x10000001: "SAM_NON_SECURITY_GROUP_OBJECT",
	0x20000000: "SAM_ALIAS_OBJECT",
	0x20000001: "SAM_NON_SECURITY_ALIAS_OBJECT",
	0x30000000: "SAM_NORMAL_USER_ACCOUNT",
	0x30000001: "SAM_MACHINE_ACCOUNT",
	0x30000002: "SAM_TRUST_ACCOUNT",
	0x40000000: "SAM_APP_BASIC_GROUP",
	0x40000001: "SAM_APP_QUERY_GROUP",
}

// parseBitField accepts the signed decimal a directory returns and reinterprets
// it as the unsigned 32-bit value it actually is. groupType arrives negative
// whenever its top bit is set.
func parseBitField(s string) (uint32, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bit field: %q is not an integer", s)
	}
	return uint32(n), nil
}

// splitBits names every set bit, reporting unrecognised ones in hex rather
// than discarding them.
func splitBits(v uint32, known []bit) []string {
	var out []string
	var matched uint32
	for _, b := range known {
		if v&b.value != 0 {
			out = append(out, b.name)
			matched |= b.value
		}
	}
	if leftover := v &^ matched; leftover != 0 {
		out = append(out, fmt.Sprintf("unknown 0x%08X", leftover))
	}
	return out
}

// UserAccountControl names the flags packed into a userAccountControl value.
func UserAccountControl(s string) ([]string, error) {
	v, err := parseBitField(s)
	if err != nil {
		return nil, err
	}
	return splitBits(v, uacBits), nil
}

// GroupType names a group's scope and whether it is a security group.
func GroupType(s string) ([]string, error) {
	v, err := parseBitField(s)
	if err != nil {
		return nil, err
	}
	return splitBits(v, groupTypeBits), nil
}

// SAMAccountType names the kind of account. Unlike the others it is an
// enumeration, not a bit field.
func SAMAccountType(s string) (string, error) {
	v, err := parseBitField(s)
	if err != nil {
		return "", err
	}
	if name, ok := samAccountTypes[v]; ok {
		return name, nil
	}
	return fmt.Sprintf("unknown 0x%08X", v), nil
}
