package app

import (
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The event payloads are the one part of the bridge Wails does not generate
// TypeScript for: the generator only sees types that appear in a bound
// method's signature, and these travel as events. frontend/src/events.ts
// declares them by hand, so this test is what stops the two from drifting.
func TestEventPayloadsMatchTheirTypeScript(t *testing.T) {
	source, err := os.ReadFile("../frontend/src/events.ts")
	if err != nil {
		t.Fatalf("cannot read the hand-written declarations: %v", err)
	}

	tests := []struct {
		iface string
		value any
	}{
		{"SearchRow", SearchRow{}},
		{"SearchBatch", SearchBatch{}},
		{"SearchDone", SearchDone{}},
	}

	for _, tt := range tests {
		t.Run(tt.iface, func(t *testing.T) {
			want := jsonKeys(t, tt.value)
			got := tsFields(t, string(source), tt.iface)

			if strings.Join(want, ",") != strings.Join(got, ",") {
				t.Errorf("Go emits %v but events.ts declares %v.\nUpdate frontend/src/events.ts to match.", want, got)
			}
		})
	}
}

// The event names have to agree too: a typo in either one produces a search
// that runs and reports nothing, which looks exactly like a directory with no
// matching objects.
func TestEventNamesMatchTheirTypeScript(t *testing.T) {
	source, err := os.ReadFile("../frontend/src/events.ts")
	if err != nil {
		t.Fatalf("cannot read the hand-written declarations: %v", err)
	}

	for _, name := range []string{EventSearchBatch, EventSearchDone} {
		if !strings.Contains(string(source), `"`+name+`"`) {
			t.Errorf("events.ts does not mention the event name %q", name)
		}
	}
}

// jsonKeys returns the field names a value would marshal to, sorted.
//
// It reads the struct tags rather than marshalling: a zero value drops every
// omitempty field, and those are exactly the ones a hand-written declaration
// is most likely to get wrong.
func jsonKeys(t *testing.T, v any) []string {
	t.Helper()

	typ := reflect.TypeOf(v)
	if typ.Kind() != reflect.Struct {
		t.Fatalf("%T is not a struct", v)
	}

	out := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

var tsField = regexp.MustCompile(`(?m)^\s{2}(\w+)\??:`)

// tsFields returns the property names declared on a TypeScript interface,
// sorted, ignoring the optional marker.
func tsFields(t *testing.T, source, name string) []string {
	t.Helper()

	start := strings.Index(source, "export interface "+name+" {")
	if start < 0 {
		t.Fatalf("events.ts declares no interface %s", name)
	}
	end := strings.Index(source[start:], "\n}")
	if end < 0 {
		t.Fatalf("the declaration of %s is not closed", name)
	}

	var out []string
	for _, m := range tsField.FindAllStringSubmatch(source[start:start+end], -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}
