package application_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

func TestRiskIsAssessedAtOpeningAndAgainWithTheQuestionnaire(t *testing.T) {
	svc, store, _ := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "125.40")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	r := created.View.Risk
	if r == nil || r.Tier != domain.RiskLow || len(r.Signals) != 7 || len(r.History) != 0 {
		t.Fatalf("risk at opening = %+v", r)
	}
	// Contradictory answers move the score, and the earlier assessment stays as history.
	apply(t, svc, created.View.ID, domain.EventOpenInvestigation, domain.EventSendQuestionnaire)
	res, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: created.View.ID, Event: domain.EventReceiveQuestionnaire,
		Payload: json.RawMessage(`{"answers":{"recognise_merchant":"yes","card_in_possession":"yes","shared_credentials":"no",
			"prior_disputes_merchant":"no","noticed_on":"2026-09-20","police_report":"yes"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	r = res.View.Risk
	if r.Score != 25 || r.Tier != domain.RiskMedium || len(r.History) != 1 || r.History[0].Tier != domain.RiskLow {
		t.Errorf("risk after two contradictions = %+v", r)
	}
}

func TestRepeatOffendersScoreHighAndCreditsNeedAJustification(t *testing.T) {
	svc, store, _ := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "1899.00")
	// Three earlier disputes on the same account within the year, one of them lost at the network.
	prior := make([]uuid.UUID, 0, 3)
	for range 3 {
		d, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
		prior = append(prior, d.View.ID)
	}
	apply(t, svc, prior[0], domain.EventOpenInvestigation, domain.EventIssueRefund, domain.EventFileChargeback, domain.EventAcknowledgeChargeback, domain.EventLoseChargeback)

	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	r := created.View.Risk
	if r.Tier != domain.RiskHigh || r.Score != 50 { // frequency 25 + one lost chargeback 15 + amount 10
		t.Fatalf("risk = %+v", r)
	}
	id := created.View.ID
	apply(t, svc, id, domain.EventOpenInvestigation)
	_, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueRefund})
	if !errors.Is(err, domain.ErrRiskHold) {
		t.Fatalf("credit on HIGH without justification: %v", err)
	}
	view, _ := svc.GetDispute(apptest.Ctx(), id)
	if view.State != domain.StateInvestigating || len(view.Ledger) != 0 {
		t.Errorf("hold left traces: %s %d", view.State, len(view.Ledger))
	}
	res, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueRefund,
		Payload: json.RawMessage(`{"riskOverride":"customer verified in branch; card confirmed stolen by police report 2026/1234"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if res.View.State != domain.StateFastRefundIssued || len(res.View.Ledger) != 1 {
		t.Errorf("after override: %s %d postings", res.View.State, len(res.View.Ledger))
	}
	// The justification is on the event for the audit trail.
	last := res.View.Events[len(res.View.Events)-1]
	var facts map[string]string
	_ = json.Unmarshal(last.Payload, &facts)
	if facts["riskOverride"] == "" {
		t.Errorf("override not on the event: %s", last.Payload)
	}

}

func TestListRowsCarryTheTier(t *testing.T) {
	svc, store, _ := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "10")
	svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn}) //nolint:errcheck // the list is what is under test
	page, err := svc.ListDisputes(apptest.Ctx(), application.ListQuery{Limit: 5})
	if err != nil || len(page.Items) != 1 || page.Items[0].Risk == nil || page.Items[0].Risk.Tier != domain.RiskLow {
		t.Errorf("list = %+v %v", page.Items, err)
	}
}
