package decode

import (
	"strings"
	"testing"
)

func TestUserAccountControl(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"normal account", "512", []string{"NORMAL_ACCOUNT"}},
		{"password never expires", "66048", []string{"NORMAL_ACCOUNT", "DONT_EXPIRE_PASSWORD"}},
		{"disabled account", "514", []string{"ACCOUNTDISABLE", "NORMAL_ACCOUNT"}},
		{"no flags set", "0", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UserAccountControl(tt.in)
			if err != nil {
				t.Fatalf("UserAccountControl(%q) returned error: %v", tt.in, err)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("UserAccountControl(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// Flags come back in bit order, not alphabetical order: the base account type
// reads first and the modifiers follow, which is how an administrator thinks
// about the value.
func TestUserAccountControlOrdersByBit(t *testing.T) {
	got, err := UserAccountControl("66050") // ACCOUNTDISABLE + NORMAL_ACCOUNT + DONT_EXPIRE_PASSWORD
	if err != nil {
		t.Fatalf("UserAccountControl() returned error: %v", err)
	}
	want := "ACCOUNTDISABLE,NORMAL_ACCOUNT,DONT_EXPIRE_PASSWORD"
	if strings.Join(got, ",") != want {
		t.Errorf("UserAccountControl() = %v, want %s", got, want)
	}
}

func TestUserAccountControlKeepsUnknownBits(t *testing.T) {
	// Bit 31 has no assigned meaning. It must still be reported, not dropped:
	// silently hiding a set bit is worse than showing a hex value.
	got, err := UserAccountControl("2147483648")
	if err != nil {
		t.Fatalf("UserAccountControl() returned error: %v", err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "0x80000000") {
		t.Errorf("UserAccountControl() = %v, want the unknown bit reported in hex", got)
	}
}

func TestGroupType(t *testing.T) {
	// -2147483646 is 0x80000002: a global security group.
	got, err := GroupType("-2147483646")
	if err != nil {
		t.Fatalf("GroupType() returned error: %v", err)
	}
	if strings.Join(got, ",") != "GLOBAL,SECURITY" {
		t.Errorf("GroupType() = %v, want [GLOBAL SECURITY]", got)
	}
}

func TestSAMAccountType(t *testing.T) {
	got, err := SAMAccountType("805306368")
	if err != nil {
		t.Fatalf("SAMAccountType() returned error: %v", err)
	}
	if got != "SAM_NORMAL_USER_ACCOUNT" {
		t.Errorf("SAMAccountType() = %q, want SAM_NORMAL_USER_ACCOUNT", got)
	}
}

func TestBitFieldsRejectGarbage(t *testing.T) {
	for _, in := range []string{"", "not a number"} {
		if _, err := UserAccountControl(in); err == nil {
			t.Errorf("UserAccountControl(%q) succeeded, want an error", in)
		}
		if _, err := GroupType(in); err == nil {
			t.Errorf("GroupType(%q) succeeded, want an error", in)
		}
		if _, err := SAMAccountType(in); err == nil {
			t.Errorf("SAMAccountType(%q) succeeded, want an error", in)
		}
	}
}
