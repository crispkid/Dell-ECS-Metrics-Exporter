package profile

import (
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		input       string
		core        string
		suffix      string
		wantErrPart string
	}{
		{name: "four parts", input: "3.8.1.7", core: "3.8.1.7"},
		{
			name:   "fixture build suffix",
			input:  "3.8.1.7.123456.synthetic",
			core:   "3.8.1.7",
			suffix: ".123456.synthetic",
		},
		{name: "hyphen suffix", input: "3.6.2.6-build_12", core: "3.6.2.6", suffix: "-build_12"},
		{name: "empty", wantErrPart: "empty"},
		{name: "three parts", input: "3.8.1", wantErrPart: "four numeric parts"},
		{name: "negative", input: "3.8.-1.0", wantErrPart: "four numeric parts"},
		{name: "whitespace", input: " 3.8.1.7", wantErrPart: "whitespace"},
		{name: "non numeric", input: "3.8.x.7", wantErrPart: "four numeric parts"},
		{
			name:        "overflow",
			input:       "3.8.1.4294967296",
			wantErrPart: "value out of range",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseVersion(test.input)
			if test.wantErrPart != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErrPart) {
					t.Fatalf("ParseVersion(%q) error = %v, want containing %q", test.input, err, test.wantErrPart)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVersion(%q): %v", test.input, err)
			}
			if got.Core() != test.core || got.Suffix() != test.suffix || got.String() != test.input {
				t.Fatalf("ParseVersion(%q) = core %q suffix %q raw %q", test.input, got.Core(), got.Suffix(), got.String())
			}
		})
	}
}

func TestVersionCompareIgnoresSuffix(t *testing.T) {
	t.Parallel()
	first := mustVersion(t, "3.8.1.7.100")
	sameCore := mustVersion(t, "3.8.1.7.999")
	later := mustVersion(t, "3.8.2.0")

	if got := first.Compare(sameCore); got != 0 {
		t.Fatalf("same core comparison = %d, want 0", got)
	}
	if got := first.Compare(later); got >= 0 {
		t.Fatalf("earlier comparison = %d, want negative", got)
	}
	if got := later.Compare(first); got <= 0 {
		t.Fatalf("later comparison = %d, want positive", got)
	}
}

func mustVersion(t *testing.T, value string) Version {
	t.Helper()
	version, err := ParseVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
