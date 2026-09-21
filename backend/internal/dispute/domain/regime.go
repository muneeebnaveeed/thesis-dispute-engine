// Package domain is the dispute bounded context's model: regimes, lifecycle states, transitions. No I/O.
package domain

import (
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
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

// Rules is what a regime contributes to the state machine (see docs/adr/0002) and its clocks (docs/adr/0013).
type Rules struct {
	Regime             Regime
	Refund             RefundPath
	Clocks             []Clock
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

const defaultAppeals = 1

var rules = map[Regime]Rules{
	RegimeEUSEPADirectDebit: {
		Regime: RegimeEUSEPADirectDebit,
		Refund: RefundNoQuestionsAsked,
		Clocks: []Clock{
			{Kind: DeadlineRefund, Count: 10, Unit: BusinessDays, Basis: "PSD2 art. 77(1): refund or justification within 10 business days of the request"},
			{Kind: DeadlineResolution, Count: 15, Unit: BusinessDays, Basis: "PSD2 art. 101(2): final reply to a complaint within 15 business days"},
		},
		HasAdjudication:    false,
		MerchantMayContest: false,
		LiabilityCapMinor:  0,
		Currency:           "EUR",
		MaxAppeals:         defaultAppeals,
	},
	RegimeEUPSD2Card: {
		Regime: RegimeEUPSD2Card,
		Refund: RefundFast,
		Clocks: []Clock{
			{Kind: DeadlineRefund, Count: 1, Unit: BusinessDays, Basis: "PSD2 art. 73(1): refund by the end of the following business day"},
			{Kind: DeadlineResolution, Count: 15, Unit: BusinessDays, Basis: "PSD2 art. 101(2): final reply to a complaint within 15 business days"},
		},
		HasAdjudication:    true,
		MerchantMayContest: true,
		LiabilityCapMinor:  50_00,
		Currency:           "EUR",
		MaxAppeals:         defaultAppeals,
	},
	RegimeUSRegE: {
		Regime: RegimeUSRegE,
		Refund: RefundProvisionalCredit,
		Clocks: []Clock{
			{Kind: DeadlineRefund, Count: 10, Unit: BusinessDays, Basis: "12 CFR 1005.11(c)(1): provisional credit within 10 business days"},
			{Kind: DeadlineResolution, Count: 45, Unit: CalendarDays, Basis: "12 CFR 1005.11(c)(2): investigation complete within 45 days"},
		},
		HasAdjudication:    true,
		MerchantMayContest: true,
		LiabilityCapMinor:  50_00,
		Currency:           "USD",
		MaxAppeals:         defaultAppeals,
	},
	RegimeUSRegZ: {
		Regime: RegimeUSRegZ,
		Refund: RefundNone,
		Clocks: []Clock{
			{Kind: DeadlineAcknowledge, Count: 30, Unit: CalendarDays, Basis: "12 CFR 1026.13(c)(1): written acknowledgement within 30 days"},
			{Kind: DeadlineResolution, Count: 90, Unit: CalendarDays, Basis: "12 CFR 1026.13(c)(2): resolution within two billing cycles, at most 90 days"},
		},
		HasAdjudication:    true,
		MerchantMayContest: true,
		LiabilityCapMinor:  0,
		Currency:           "USD",
		MaxAppeals:         defaultAppeals,
	},
}

// ErrUnknownRegime means the regime string is not one this build knows.
var ErrUnknownRegime = errs.New(errs.Invalid, "unknown-regime", "unknown regulatory regime")

// RulesFor returns the configuration for a regime.
func RulesFor(r Regime) (Rules, error) {
	cfg, ok := rules[r]
	if !ok {
		return Rules{}, errs.Wrap(ErrUnknownRegime, "%q", r)
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
