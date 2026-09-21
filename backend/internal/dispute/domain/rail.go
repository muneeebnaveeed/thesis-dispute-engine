package domain

import (
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// Rail is the payment instrument a transaction was made on.
type Rail string

// Supported rails.
const (
	RailCard       Rail = "CARD"
	RailCreditCard Rail = "CREDIT_CARD"
	RailSEPADD     Rail = "SEPA_DD"
)

// ErrNoRegime means no regime governs the rail and currency combination.
var ErrNoRegime = errs.New(errs.Unprocessable, "no-regime", "no regulatory regime covers this transaction's rail and currency")

// RegimeFor derives the regime from the rail and the account currency.
func RegimeFor(rail Rail, currency string) (Regime, error) {
	switch {
	case rail == RailSEPADD && currency == "EUR":
		return RegimeEUSEPADirectDebit, nil
	case rail == RailCard && currency == "EUR":
		return RegimeEUPSD2Card, nil
	case rail == RailCard && currency == "USD":
		return RegimeUSRegE, nil
	case rail == RailCreditCard && currency == "USD":
		return RegimeUSRegZ, nil
	default:
		return "", errs.Wrap(ErrNoRegime, "%s/%s", rail, currency)
	}
}
