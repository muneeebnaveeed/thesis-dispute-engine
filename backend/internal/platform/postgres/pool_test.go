package postgres

import "testing"

func TestSpanName(t *testing.T) {
	cases := map[string]string{
		"-- name: GetDispute :one\nSELECT 1": "GetDispute",
		"BEGIN":                              "BEGIN",
		"  select 1\n  from x":               "select 1",
	}
	for in, want := range cases {
		if got := spanName(in); got != want {
			t.Errorf("spanName(%q) = %q, want %q", in, got, want)
		}
	}
}
