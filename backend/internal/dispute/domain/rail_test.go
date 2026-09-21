package domain

import (
	"errors"
	"testing"
)

func TestRegimeFor(t *testing.T) {
	cases := []struct {
		rail     Rail
		currency string
		want     Regime
	}{
		{RailSEPADD, "EUR", RegimeEUSEPADirectDebit},
		{RailCard, "EUR", RegimeEUPSD2Card},
		{RailCard, "USD", RegimeUSRegE},
		{RailCreditCard, "USD", RegimeUSRegZ},
	}
	for _, tc := range cases {
		got, err := RegimeFor(tc.rail, tc.currency)
		if err != nil || got != tc.want {
			t.Errorf("RegimeFor(%s, %s) = %s, %v; want %s", tc.rail, tc.currency, got, err, tc.want)
		}
	}
	for _, bad := range []struct {
		rail     Rail
		currency string
	}{{RailSEPADD, "USD"}, {RailCreditCard, "EUR"}, {Rail("CASH"), "EUR"}, {RailCard, "GBP"}} {
		if _, err := RegimeFor(bad.rail, bad.currency); !errors.Is(err, ErrNoRegime) {
			t.Errorf("RegimeFor(%s, %s): err = %v, want ErrNoRegime", bad.rail, bad.currency, err)
		}
	}
}
