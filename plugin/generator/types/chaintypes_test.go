package types

import "testing"

func TestSolidityIsReference(t *testing.T) {
	cases := map[string]bool{
		"string":   true,
		"bytes":    true,
		"int32[]":  true,
		"string[]": true,
		"bool":     false,
		"int32":    false,
		"uint256":  false,
	}
	for in, want := range cases {
		if got := SolidityIsReference(in); got != want {
			t.Errorf("SolidityIsReference(%q) = %v, want %v", in, got, want)
		}
	}
}
