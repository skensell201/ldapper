package decode

import "testing"

func TestSID(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{
			name: "domain user",
			in: []byte{
				0x01, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				0x15, 0x00, 0x00, 0x00, // 21
				0xC7, 0x51, 0xDA, 0xD8, // 3638186439
				0xBC, 0x33, 0x34, 0xCF, // 3476304828
				0x14, 0x01, 0xED, 0x01, // 32309524
				0x50, 0x04, 0x00, 0x00, // 1104
			},
			want: "S-1-5-21-3638186439-3476304828-32309524-1104",
		},
		{
			name: "well-known Everyone",
			in:   []byte{0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
			want: "S-1-1-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SID(tt.in)
			if err != nil {
				t.Fatalf("SID() returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("SID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSIDRejectsMalformed(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{"empty", nil},
		{"header only", []byte{0x01, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05}},
		{"truncated sub-authority", []byte{
			0x01, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
			0x15, 0x00, 0x00, 0x00,
			0xC7, 0x51, // one byte short of a second sub-authority
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := SID(tt.in); err == nil {
				t.Errorf("SID(%v) succeeded, want an error", tt.in)
			}
		})
	}
}
