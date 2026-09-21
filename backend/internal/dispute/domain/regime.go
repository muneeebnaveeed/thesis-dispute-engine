// Package domain is the dispute bounded context's model: regimes, lifecycle states, transitions. No I/O.
package domain

import (
	"fmt"
	"time"
)

// Regime is the regulatory framework a dispute is handled under; fixed at creation.
type Regime string

// Supported regimes.
const (
	RegimeEUSEPADirectDebit Regime = "EU_SEPA_DIRECT_DEBIT"
	RegimeEUPSD2Card        Regime = "EU_PSD2_CARD"
	RegimeUSRegE            Regime = "US_REG_E"
	RegimeUSRegZ            Regime = "US_REG_Z"
)

// AllRegimes lists every supported regime.
func AllRegimes() []Regime {
	return []Regime{RegimeEUSEPADirectDebit, RegimeEUPSD2Card, RegimeUSRegE, RegimeUSRegZ}
}

// RefundPath is how a regime makes the customer whole while the dispute runs.
type RefundPath string

// Refund paths.
const (
	RefundNone              RefundPath = "NONE"
	RefundProvisionalCredit RefundPath = "PROVISIONAL_CREDIT"
	RefundFast              RefundPath = "FAST_REFUND"
	RefundNoQuestionsAsked  RefundPath = "NQA_REFUND"
)

// Rules is what a regime contributes to the state machine (see docs/adr/0002).
type Rules struct {
	Regime             Regime
	Refund             RefundPath
	RefundSLA          time.Duration
	ResolutionSLA      time.Duration
	HasAdjudication    bool
	MerchantMayContest bool
	LiabilityCapMinor  int64
	Currency           string
	MaxAppeals         int
}

// HasProvisionalCredit reports whether a reversible credit precedes the outcome.
func (r Rules) HasProvisionalCredit() bool { return r.Refund == RefundProvisionalCredit }

// HasRefundStep reports whether any refund state is part of the lifecycle.
func (r Rules) HasRefundStep() bool { return r.Refund != RefundNone }

const (
	day            = 24 * time.Hour
	businessDay    = day // calendar approximation until the SLA engine has a business-day calendar
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
		return Rules{}, fmt.Errorf("domain: unknown regime %q", r)
	}
	return cfg, nil
}

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
