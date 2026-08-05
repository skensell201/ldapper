package decode

import "testing"

func TestFileTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"a real timestamp", "133894094610000000", "2025-04-18 00:24:21 UTC"},
		{"never set", "0", "not set"},
		{"never expires", "9223372036854775807", "never"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FileTime(tt.in)
			if err != nil {
				t.Fatalf("FileTime(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("FileTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFileTimeRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "not a number", "-5"} {
		if _, err := FileTime(in); err == nil {
			t.Errorf("FileTime(%q) succeeded, want an error", in)
		}
	}
}

func TestGeneralizedTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"with fraction and Z", "20230914081233.0Z", "2023-09-14 08:12:33 UTC"},
		{"without fraction", "20230914081233Z", "2023-09-14 08:12:33 UTC"},
		{"with numeric offset", "20230914101233+0200", "2023-09-14 08:12:33 UTC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GeneralizedTime(tt.in)
			if err != nil {
				t.Fatalf("GeneralizedTime(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("GeneralizedTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
