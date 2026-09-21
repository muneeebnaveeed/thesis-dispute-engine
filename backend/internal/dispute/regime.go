// Package dispute is the domain core: regulatory regimes, the dispute
// lifecycle state machine, and the transitions each regime permits. It has no
// I/O and no dependencies outside the standard library so it can be tested
// exhaustively and read as the specification. See docs/adr/0002.
package dispute

import (
	"fmt"
	"time"
)

// Regime identifies the regulatory framework a dispute is handled under. It is
// derived from the transaction's payment rail when the dispute is created and
// never changes afterwards.
type Regime string

// Regimes. EU regimes are first-class and the implementation focus; US regimes
// exist as configuration and fixtures to show the model generalises.
const (
	RegimeEUSEPADirectDebit Regime = "EU_SEPA_DIRECT_DEBIT"
	RegimeEUPSD2Card        Regime = "EU_PSD2_CARD"
	RegimeUSRegE            Regime = "US_REG_E"
	RegimeUSRegZ            Regime = "US_REG_Z"
)

// AllRegimes lists every supported regime in priority order.
func AllRegimes() []Regime {
	return []Regime{RegimeEUSEPADirectDebit, RegimeEUPSD2Card, RegimeUSRegE, RegimeUSRegZ}
}

// RefundPath is how a regime makes the customer whole while the dispute runs.
type RefundPath string

// Refund paths; each maps to the state that represents it.
const (
	// RefundNone: the customer is not credited before resolution (Reg Z:
	// collection is suspended instead).
	RefundNone RefundPath = "NONE"
	// RefundProvisionalCredit: a reversible credit pending investigation (Reg E).
	RefundProvisionalCredit RefundPath = "PROVISIONAL_CREDIT"
	// RefundFast: an outright refund by the regime's deadline (PSD2 Art. 73).
	RefundFast RefundPath = "FAST_REFUND"
	// RefundNoQuestionsAsked: an unconditional refund on request (SEPA DD, 8 weeks).
	RefundNoQuestionsAsked RefundPath = "NQA_REFUND"
)

// Rules is the configuration a regime contributes to the state machine.
type Rules struct {
	Regime Regime
	// Refund is the path used to make the customer whole.
	Refund RefundPath
	// RefundSLA is how long the issuer has to complete the refund path from
	// dispute initiation. Zero means the regime has no refund step.
	RefundSLA time.Duration
	// ResolutionSLA bounds the whole investigation.
	ResolutionSLA time.Duration
	// HasAdjudication reports whether the dispute can proceed to a chargeback
	// with evidence and a win/lose outcome.
	HasAdjudication bool
	// MerchantMayContest reports whether the merchant side can defend.
	MerchantMayContest bool
	// LiabilityCapMinor is the customer's maximum liability in minor units of
	// the regime's currency for unauthorised transactions; 0 means none.
	LiabilityCapMinor int64
	// Currency is the ISO 4217 code the liability cap is expressed in.
	Currency string
	// MaxAppeals caps how many times a closed dispute may re-enter investigation.
	MaxAppeals int
}

// HasProvisionalCredit reports whether the regime issues a reversible credit
// before the outcome is known. Gate on this, not on the Regime value.
func (r Rules) HasProvisionalCredit() bool { return r.Refund == RefundProvisionalCredit }

// HasRefundStep reports whether any refund state is part of the lifecycle.
func (r Rules) HasRefundStep() bool { return r.Refund != RefundNone }

const (
	day            = 24 * time.Hour
	businessDay    = day // calendar approximation; a business-day calendar arrives with the SLA engine
	week           = 7 * day
	approxMonth    = 30 * day
	defaultAppeals = 1
)

var rules = map[Regime]Rules{
	RegimeEUSEPADirectDebit: {
		Regime:             RegimeEUSEPADirectDebit,
		Refund:             RefundNoQuestionsAsked,
		RefundSLA:          8 * week,
		ResolutionSLA:      13 * approxMonth,
		HasAdjudication:    false,
		MerchantMayContest: false,
		LiabilityCapMinor:  0,
		Currency:           "EUR",
		MaxAppeals:         defaultAppeals,
	},
	RegimeEUPSD2Card: {
		Regime:             RegimeEUPSD2Card,
		Refund:             RefundFast,
		RefundSLA:          1 * businessDay,
		ResolutionSLA:      approxMonth,
		HasAdjudication:    true,
		MerchantMayContest: true,
		LiabilityCapMinor:  50_00,
		Currency:           "EUR",
		MaxAppeals:         defaultAppeals,
	},
	RegimeUSRegE: {
		Regime:             RegimeUSRegE,
		Refund:             RefundProvisionalCredit,
		RefundSLA:          10 * businessDay,
		ResolutionSLA:      45 * day,
		HasAdjudication:    true,
		MerchantMayContest: true,
		LiabilityCapMinor:  50_00,
		Currency:           "USD",
		MaxAppeals:         defaultAppeals,
	},
	RegimeUSRegZ: {
		Regime:             RegimeUSRegZ,
		Refund:             RefundNone,
		RefundSLA:          0,
		ResolutionSLA:      90 * day,
		HasAdjudication:    true,
		MerchantMayContest: true,
		LiabilityCapMinor:  0,
		Currency:           "USD",
		MaxAppeals:         defaultAppeals,
	},
}

// RulesFor returns the configuration for a regime.
func RulesFor(r Regime) (Rules, error) {
	cfg, ok := rules[r]
	if !ok {
		return Rules{}, fmt.Errorf("dispute: unknown regime %q", r)
	}
	return cfg, nil
}

// refundState maps a refund path to the lifecycle state that represents it.
func (r Rules) refundState() State {
	switch r.Refund {
	case RefundProvisionalCredit:
		return StateProvisionalCreditIssued
	case RefundFast:
		return StateFastRefundIssued
	case RefundNoQuestionsAsked:
		return StateSEPANQARefundIssued
	default:
		return ""
	}
}
