package filters

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed builtin.json
var builtinJSON []byte

// builtins is parsed once at start-up. A malformed builtin.json is a build
// mistake, not a runtime condition, so it panics rather than degrading.
var builtins = mustParseBuiltins()

func mustParseBuiltins() []Filter {
	var out []Filter
	if err := json.Unmarshal(builtinJSON, &out); err != nil {
		panic(fmt.Sprintf("filters: builtin.json is malformed: %v", err))
	}
	for i := range out {
		out[i].BuiltIn = true
	}
	return out
}

// Builtins returns a copy of the filter set shipped with Ldapper.
func Builtins() []Filter {
	out := make([]Filter, len(builtins))
	copy(out, builtins)
	return out
}
