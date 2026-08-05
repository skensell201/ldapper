package filters

// Compatible reports whether f can run against a server offering the given
// dialects. A filter the server cannot answer correctly must not be run:
// returning nothing looks exactly like finding nothing.
func Compatible(f Filter, supported []Dialect) bool {
	for _, d := range supported {
		if d == f.Dialect {
			return true
		}
	}
	return false
}

// IncompatibleReason explains, in one sentence, why a filter is unavailable.
// It returns an empty string for portable filters, which are never hidden.
func IncompatibleReason(f Filter) string {
	switch f.Dialect {
	case DialectAD:
		return "This filter uses an Active Directory extension. The server you are connected to does not support it."
	case DialectPOSIX:
		return "This filter needs the POSIX schema, which the server you are connected to does not carry."
	default:
		return ""
	}
}
