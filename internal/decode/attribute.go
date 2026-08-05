package decode

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

// Value is one attribute value ready to be shown: the server's own bytes, and
// whatever sense could be made of them.
type Value struct {
	// Raw is the value as the server returned it — printable text as-is,
	// anything else base64-encoded.
	Raw string
	// Decoded holds the human-readable rendering, or nil when the attribute
	// has no decoder or its value could not be decoded. Bit fields produce
	// several entries; everything else produces at most one.
	Decoded []string
}

// binaryDecoder reads raw bytes; stringDecoder reads the text form.
type binaryDecoder func([]byte) (string, error)
type stringDecoder func(string) (string, error)
type multiDecoder func(string) ([]string, error)

var binaryDecoders = map[string]binaryDecoder{
	"objectsid":  SID,
	"objectguid": GUID,
}

var stringDecoders = map[string]stringDecoder{
	"pwdlastset":         FileTime,
	"lastlogon":          FileTime,
	"lastlogontimestamp": FileTime,
	"accountexpires":     FileTime,
	"badpasswordtime":    FileTime,
	"lockouttime":        FileTime,
	"whencreated":        GeneralizedTime,
	"whenchanged":        GeneralizedTime,
	"samaccounttype":     SAMAccountType,
}

var multiDecoders = map[string]multiDecoder{
	"useraccountcontrol": UserAccountControl,
	"grouptype":          GroupType,
}

// Attribute decodes every value of one attribute. It never returns an error:
// a value that cannot be decoded is returned raw, because failing to explain a
// value is not a reason to refuse to show it.
func Attribute(name string, values [][]byte) []Value {
	key := strings.ToLower(name)
	out := make([]Value, 0, len(values))

	for _, raw := range values {
		v := Value{Raw: printable(raw)}

		switch {
		case binaryDecoders[key] != nil:
			if s, err := binaryDecoders[key](raw); err == nil {
				v.Decoded = []string{s}
			}
		case stringDecoders[key] != nil:
			if s, err := stringDecoders[key](string(raw)); err == nil {
				v.Decoded = []string{s}
			}
		case multiDecoders[key] != nil:
			if list, err := multiDecoders[key](string(raw)); err == nil && len(list) > 0 {
				v.Decoded = list
			}
		}

		out = append(out, v)
	}
	return out
}

// printable returns text unchanged and encodes anything else as base64, the
// same convention LDIF uses.
func printable(b []byte) string {
	if !utf8.Valid(b) {
		return base64.StdEncoding.EncodeToString(b)
	}
	for _, r := range string(b) {
		if r < 0x20 || r == 0x7F {
			return base64.StdEncoding.EncodeToString(b)
		}
	}
	return string(b)
}
