package filters

import "testing"

func TestCompatible(t *testing.T) {
	tests := []struct {
		name      string
		dialect   Dialect
		supported []Dialect
		want      bool
	}{
		{"generic runs anywhere", DialectGeneric, []Dialect{DialectGeneric}, true},
		{"AD filter on AD", DialectAD, []Dialect{DialectGeneric, DialectAD}, true},
		{"AD filter on OpenLDAP", DialectAD, []Dialect{DialectGeneric, DialectPOSIX}, false},
		{"POSIX filter on OpenLDAP", DialectPOSIX, []Dialect{DialectGeneric, DialectPOSIX}, true},
		{"POSIX filter on AD", DialectPOSIX, []Dialect{DialectGeneric, DialectAD}, false},
		{"nothing supported", DialectGeneric, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compatible(Filter{Dialect: tt.dialect}, tt.supported)
			if got != tt.want {
				t.Errorf("Compatible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIncompatibleReasonNamesTheServer(t *testing.T) {
	got := IncompatibleReason(Filter{Name: "Disabled accounts", Dialect: DialectAD})
	if got == "" {
		t.Fatal("IncompatibleReason() returned an empty string")
	}
}

func TestIncompatibleReasonIsEmptyForGeneric(t *testing.T) {
	if got := IncompatibleReason(Filter{Dialect: DialectGeneric}); got != "" {
		t.Errorf("IncompatibleReason() = %q, want empty for a portable filter", got)
	}
}
