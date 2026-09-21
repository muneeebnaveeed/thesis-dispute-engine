package domain

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// RiskTier is what the score means for handling (docs/adr/0018): LOW proceeds, MEDIUM is flagged for the analyst,
// HIGH holds any credit until an analyst records why they are overriding it.
type RiskTier string

// Risk tiers.
const (
	RiskLow    RiskTier = "LOW"
	RiskMedium RiskTier = "MEDIUM"
	RiskHigh   RiskTier = "HIGH"
)

// Tier thresholds on the score.
const (
	mediumFrom = 20
	highFrom   = 45
)

// Signal is one contribution to the score: what was looked at, how much it can weigh, how much it did, and why.
type Signal struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"` // the most this signal can add
	Points int    `json:"points"`
	Detail string `json:"detail"`
}

// RiskInput is what the assessment looks at; the service gathers it from the account's history.
type RiskInput struct {
	DisputesLast12Months int  // other disputes on the account
	LostChargebacks      int  // other disputes on the account the customer lost at the network
	TransactionAgeDays   int  // between the transaction and the dispute
	HighRiskMerchant     bool // by merchant category code
	Amount               decimal.Decimal
	AccountAgeDays       int
	Inconsistencies      int  // contradictions in the questionnaire
	QuestionnaireKnown   bool // false before the questionnaire is received
}

// Assessment is the score, its tier and the signals that made it.
type Assessment struct {
	Score   int      `json:"score"`
	Tier    RiskTier `json:"tier"`
	Signals []Signal `json:"signals"`
}

// highRiskMCCs are merchant categories where friendly fraud concentrates: digital goods, gambling, quasi-cash,
// money transfer. A tenant with better data would make this its own list.
var highRiskMCCs = map[string]string{
	"5815": "digital goods: media", "5816": "digital goods: games", "5817": "digital goods: software", "5818": "digital goods: large merchant",
	"7995": "gambling", "6051": "quasi cash", "4829": "money transfer",
}

// HighRiskMCC reports whether a merchant category code is on the list, with its label.
func HighRiskMCC(mcc string) (string, bool) {
	label, ok := highRiskMCCs[mcc]
	return label, ok
}

// Assess scores the input with fixed, explainable rules. Every signal reports even when it adds nothing, so an
// analyst sees what was checked, not only what fired.
func Assess(in RiskInput) Assessment {
	var signals []Signal
	add := func(name string, weight, points int, detail string) {
		signals = append(signals, Signal{Name: name, Weight: weight, Points: points, Detail: detail})
	}

	switch n := in.DisputesLast12Months; {
	case n >= 3:
		add("dispute frequency", 25, 25, fmt.Sprintf("%d other disputes on this account in the last twelve months", n))
	case n == 2:
		add("dispute frequency", 25, 18, "two other disputes on this account in the last twelve months")
	case n == 1:
		add("dispute frequency", 25, 10, "one other dispute on this account in the last twelve months")
	default:
		add("dispute frequency", 25, 0, "no other dispute on this account in the last twelve months")
	}

	switch {
	case !in.QuestionnaireKnown:
		add("questionnaire consistency", 25, 0, "questionnaire not yet received")
	case in.Inconsistencies >= 2:
		add("questionnaire consistency", 25, 25, fmt.Sprintf("%d contradictions between the answers", in.Inconsistencies))
	case in.Inconsistencies == 1:
		add("questionnaire consistency", 25, 15, "one contradiction between the answers")
	default:
		add("questionnaire consistency", 25, 0, "answers are consistent")
	}

	switch n := in.LostChargebacks; {
	case n >= 2:
		add("chargeback history", 25, 25, fmt.Sprintf("%d earlier disputes on this account were lost at the network", n))
	case n == 1:
		add("chargeback history", 25, 15, "one earlier dispute on this account was lost at the network")
	default:
		add("chargeback history", 25, 0, "no earlier dispute on this account was lost at the network")
	}

	switch d := in.TransactionAgeDays; {
	case d > 60:
		add("transaction age", 10, 10, fmt.Sprintf("disputed %d days after the transaction", d))
	case d > 30:
		add("transaction age", 10, 5, fmt.Sprintf("disputed %d days after the transaction", d))
	default:
		add("transaction age", 10, 0, fmt.Sprintf("disputed %d days after the transaction", d))
	}

	if in.HighRiskMerchant {
		add("merchant category", 10, 10, "merchant category is one where disputes concentrate")
	} else {
		add("merchant category", 10, 0, "merchant category is not on the watch list")
	}

	switch {
	case in.Amount.GreaterThanOrEqual(decimal.NewFromInt(1000)):
		add("amount", 10, 10, "amount is 1000 or more")
	case in.Amount.GreaterThanOrEqual(decimal.NewFromInt(500)):
		add("amount", 10, 5, "amount is 500 or more")
	default:
		add("amount", 10, 0, "amount is under 500")
	}

	switch d := in.AccountAgeDays; {
	case d < 90:
		add("account age", 5, 5, fmt.Sprintf("account is %d days old", d))
	case d < 365:
		add("account age", 5, 2, fmt.Sprintf("account is %d days old", d))
	default:
		add("account age", 5, 0, "account is more than a year old")
	}

	a := Assessment{Signals: signals}
	for _, s := range signals {
		a.Score += s.Points
	}
	switch {
	case a.Score >= highFrom:
		a.Tier = RiskHigh
	case a.Score >= mediumFrom:
		a.Tier = RiskMedium
	default:
		a.Tier = RiskLow
	}
	return a
}

// ErrRiskHold means a credit was attempted on a HIGH-risk dispute without an analyst's recorded justification.
var ErrRiskHold = errs.New(errs.Unprocessable, "risk-hold", "this dispute is on hold for fraud review; a credit needs a recorded justification")

// CreditAllowed says whether a credit may go out given the latest assessment and the analyst's override text;
// the override is the audit trail of a human deciding against the score.
func CreditAllowed(a *Assessment, override string) error {
	if a == nil || a.Tier != RiskHigh || override != "" {
		return nil
	}
	return ErrRiskHold.WithFields(errs.FieldError{Field: "body.payload.riskOverride",
		Message: fmt.Sprintf("risk score %d (HIGH): explain why the credit should go out anyway", a.Score)})
}

// DaysBetween is whole days from a to b, never negative.
func DaysBetween(a, b time.Time) int {
	if d := int(b.Sub(a).Hours() / 24); d > 0 {
		return d
	}
	return 0
}
