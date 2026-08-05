package decode

import "testing"

func TestAttributeDecodesKnownNames(t *testing.T) {
	got := Attribute("userAccountControl", [][]byte{[]byte("66048")})
	if len(got) != 1 {
		t.Fatalf("Attribute() returned %d values, want 1", len(got))
	}
	if got[0].Raw != "66048" {
		t.Errorf("Raw = %q, want 66048", got[0].Raw)
	}
	if len(got[0].Decoded) != 2 || got[0].Decoded[0] != "NORMAL_ACCOUNT" {
		t.Errorf("Decoded = %v, want both flags in bit order", got[0].Decoded)
	}
}

func TestAttributeIsCaseInsensitive(t *testing.T) {
	// Directories are inconsistent about attribute name casing.
	got := Attribute("PWDLASTSET", [][]byte{[]byte("0")})
	if len(got[0].Decoded) != 1 || got[0].Decoded[0] != "not set" {
		t.Errorf("Decoded = %v, want [not set]", got[0].Decoded)
	}
}

func TestAttributeFallsBackToRaw(t *testing.T) {
	// An unknown attribute is passed through untouched.
	got := Attribute("displayName", [][]byte{[]byte("Anna Volkova")})
	if got[0].Raw != "Anna Volkova" {
		t.Errorf("Raw = %q, want the value unchanged", got[0].Raw)
	}
	if got[0].Decoded != nil {
		t.Errorf("Decoded = %v, want nil for an attribute with no decoder", got[0].Decoded)
	}
}

func TestAttributeSurvivesUndecodableValues(t *testing.T) {
	// objectSid has a decoder, but this is not a valid SID. The value must
	// still come back — as base64, since it is not printable text.
	got := Attribute("objectSid", [][]byte{{0xFF, 0xFE}})
	if len(got) != 1 {
		t.Fatalf("Attribute() returned %d values, want 1", len(got))
	}
	if got[0].Raw != "//4=" {
		t.Errorf("Raw = %q, want the base64 of the undecodable bytes", got[0].Raw)
	}
	if got[0].Decoded != nil {
		t.Errorf("Decoded = %v, want nil when decoding failed", got[0].Decoded)
	}
}

func TestAttributeDecodesABinaryValue(t *testing.T) {
	sid := []byte{
		0x01, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
		0x15, 0x00, 0x00, 0x00,
		0xC7, 0x51, 0xDA, 0xD8,
		0xBC, 0x33, 0x34, 0xCF,
		0x14, 0x01, 0xED, 0x01,
		0x50, 0x04, 0x00, 0x00,
	}
	got := Attribute("objectSid", [][]byte{sid})
	if len(got[0].Decoded) != 1 || got[0].Decoded[0] != "S-1-5-21-3638186439-3476304828-32309524-1104" {
		t.Errorf("Decoded = %v, want the formatted SID", got[0].Decoded)
	}
	if got[0].Raw == "" {
		t.Error("Raw is empty; the server's own bytes must always be shown")
	}
}

func TestAttributeHandlesEveryValueOfAMultiValuedAttribute(t *testing.T) {
	got := Attribute("objectClass", [][]byte{[]byte("top"), []byte("person"), []byte("user")})
	if len(got) != 3 {
		t.Fatalf("Attribute() returned %d values, want 3", len(got))
	}
}

func TestAttributeHandlesNoValues(t *testing.T) {
	if got := Attribute("cn", nil); len(got) != 0 {
		t.Errorf("Attribute() = %v, want an empty slice", got)
	}
}
