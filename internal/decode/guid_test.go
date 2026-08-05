package decode

import "testing"

func TestGUID(t *testing.T) {
	in := []byte{
		0x0C, 0x9B, 0xF1, 0xA3, // Data1, little-endian → a3f19b0c
		0x2E, 0x4D, // Data2, little-endian → 4d2e
		0x1A, 0x4F, // Data3, little-endian → 4f1a
		0x9C, 0x88, // Data4, kept in order
		0x0B, 0x7E, 0x5D, 0x2A, 0x41, 0xF6,
	}
	want := "a3f19b0c-4d2e-4f1a-9c88-0b7e5d2a41f6"

	got, err := GUID(in)
	if err != nil {
		t.Fatalf("GUID() returned error: %v", err)
	}
	if got != want {
		t.Errorf("GUID() = %q, want %q", got, want)
	}
}

func TestGUIDRejectsWrongLength(t *testing.T) {
	for _, n := range []int{0, 15, 17} {
		if _, err := GUID(make([]byte, n)); err == nil {
			t.Errorf("GUID() with %d bytes succeeded, want an error", n)
		}
	}
}
