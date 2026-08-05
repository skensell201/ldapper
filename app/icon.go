package app

import "strings"

// iconRules maps an objectClass to an icon name. Order matters: the first rule
// that matches wins, so the most specific classes are listed first. In Active
// Directory a computer is also a user and a person, and without that ordering
// every domain controller would draw as somebody's account.
var iconRules = []struct {
	class string
	icon  string
}{
	{"computer", "computer"},
	{"organizationalunit", "ou"},
	{"group", "group"},
	{"groupofnames", "group"},
	{"groupofuniquenames", "group"},
	{"posixgroup", "group"},
	{"domaindns", "domain"},
	{"domain", "domain"},
	{"dcobject", "domain"},
	{"container", "container"},
	{"builtindomain", "container"},
	{"user", "user"},
	{"inetorgperson", "user"},
	{"posixaccount", "user"},
	{"person", "user"},
}

// iconFor picks the icon a tree row should draw.
func iconFor(classes []string) string {
	present := make(map[string]bool, len(classes))
	for _, c := range classes {
		present[strings.ToLower(c)] = true
	}

	for _, rule := range iconRules {
		if present[rule.class] {
			return rule.icon
		}
	}
	return "object"
}
