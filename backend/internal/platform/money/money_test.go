package money

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFormatUsesTheCurrencyMinorUnit(t *testing.T) {
	d := decimal.RequireFromString("1899.4")
	cases := map[string]string{"EUR": "1899.40", "USD": "1899.40", "HUF": "1899", "BHD": "1899.400", "XYZ": "1899.40"}
	for cur, want := range cases {
		if got := Format(d, cur); got != want {
			t.Errorf("%s: %s, want %s", cur, got, want)
		}
	}
	if got := Format(decimal.RequireFromString("0.005"), "EUR"); got != "0.01" {
		t.Errorf("rounding = %s", got)
	}
}
