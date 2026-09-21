package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func TestAssessTiersAndExplains(t *testing.T) {
	clean := Assess(RiskInput{Amount: decimal.NewFromInt(50), AccountAgeDays: 800, TransactionAgeDays: 3})
	if clean.Tier != RiskLow || clean.Score != 0 || len(clean.Signals) != 7 {
		t.Errorf("clean = %+v", clean)
	}
	for _, s := range clean.Signals {
		if s.Detail == "" || s.Weight == 0 {
			t.Errorf("signal without explanation: %+v", s)
		}
	}
	medium := Assess(RiskInput{DisputesLast12Months: 1, Amount: decimal.NewFromInt(1200), AccountAgeDays: 800, QuestionnaireKnown: true})
	if medium.Tier != RiskMedium || medium.Score != 20 {
		t.Errorf("medium = %d %s", medium.Score, medium.Tier)
	}
	high := Assess(RiskInput{DisputesLast12Months: 3, Inconsistencies: 1, QuestionnaireKnown: true, HighRiskMerchant: true, Amount: decimal.NewFromInt(7450), AccountAgeDays: 800})
	if high.Tier != RiskHigh || high.Score != 60 {
		t.Errorf("high = %d %s", high.Score, high.Tier)
	}
	// An unreceived questionnaire neither helps nor hurts.
	if a := Assess(RiskInput{Inconsistencies: 5, QuestionnaireKnown: false, AccountAgeDays: 800}); a.Score != 0 {
		t.Errorf("unknown questionnaire scored %d", a.Score)
	}
}

func TestCreditAllowed(t *testing.T) {
	high := &Assessment{Score: 60, Tier: RiskHigh}
	if err := CreditAllowed(high, ""); !errors.Is(err, ErrRiskHold) {
		t.Errorf("HIGH without override: %v", err)
	}
	if err := CreditAllowed(high, "customer verified in branch, card confirmed stolen"); err != nil {
		t.Errorf("HIGH with override: %v", err)
	}
	if err := CreditAllowed(&Assessment{Tier: RiskMedium}, ""); err != nil {
		t.Errorf("MEDIUM: %v", err)
	}
	if err := CreditAllowed(nil, ""); err != nil {
		t.Errorf("no assessment: %v", err)
	}
}

func TestHighRiskMCC(t *testing.T) {
	if _, ok := HighRiskMCC("5815"); !ok {
		t.Error("digital goods not high risk")
	}
	if _, ok := HighRiskMCC("5411"); ok {
		t.Error("grocery high risk")
	}
}
