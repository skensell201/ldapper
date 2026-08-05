package app

import "testing"

func TestIconFor(t *testing.T) {
	tests := []struct {
		name    string
		classes []string
		want    string
	}{
		{"a person", []string{"top", "person", "organizationalPerson", "user"}, "user"},
		{"an inetOrgPerson", []string{"top", "inetOrgPerson"}, "user"},
		{"an AD group", []string{"top", "group"}, "group"},
		{"a groupOfNames", []string{"top", "groupOfNames"}, "group"},
		{"a posixGroup", []string{"posixGroup"}, "group"},
		{"an organizational unit", []string{"top", "organizationalUnit"}, "ou"},
		{"a computer", []string{"top", "person", "computer"}, "computer"},
		{"a domain", []string{"top", "domain", "domainDNS"}, "domain"},
		{"a dcObject", []string{"top", "dcObject", "organization"}, "domain"},
		{"a container", []string{"top", "container"}, "container"},
		{"anything else", []string{"top", "printQueue"}, "object"},
		{"nothing at all", nil, "object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := iconFor(tt.classes); got != tt.want {
				t.Errorf("iconFor(%v) = %q, want %q", tt.classes, got, tt.want)
			}
		})
	}
}

// A computer is also a person in Active Directory's schema. The more specific
// class has to win, or every domain controller draws as a user.
func TestIconPrefersTheMoreSpecificClass(t *testing.T) {
	if got := iconFor([]string{"user", "computer", "person"}); got != "computer" {
		t.Errorf("iconFor() = %q, want computer to beat user", got)
	}
}

func TestIconIsCaseInsensitive(t *testing.T) {
	if got := iconFor([]string{"ORGANIZATIONALUNIT"}); got != "ou" {
		t.Errorf("iconFor() = %q, want ou", got)
	}
}

// Every icon a rule can produce must exist in the frontend sprite. If a rule
// is added here without a matching path in Icon.tsx, the row draws nothing.
func TestIconNamesAreFromTheKnownSet(t *testing.T) {
	known := map[string]bool{
		"user": true, "group": true, "ou": true, "computer": true,
		"domain": true, "container": true, "object": true,
	}
	for _, rule := range iconRules {
		if !known[rule.icon] {
			t.Errorf("rule for %q produces icon %q, which the frontend sprite does not draw", rule.class, rule.icon)
		}
	}
}
