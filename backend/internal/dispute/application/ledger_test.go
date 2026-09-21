package application_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

func TestRefundPostsTheCreditLessLiabilityAndCloseSettlesSuspense(t *testing.T) {
	svc, store, _ := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "125.40")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	id := created.View.ID

	apply(t, svc, id, domain.EventOpenInvestigation)
	res, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueRefund,
		Payload: json.RawMessage(`{"liability":"50.00","note":"card reported lost"}`)})
	if err != nil {
		t.Fatal(err)
	}
	v := res.View
	if len(v.Ledger) != 1 || v.Ledger[0].Kind != domain.PostingFastRefund || v.Ledger[0].Amount.String() != "75.4" {
		t.Fatalf("ledger after refund = %+v", v.Ledger)
	}
	if v.Ledger[0].Seq != 3 || v.Ledger[0].Reference == "" || v.Ledger[0].Debit != domain.AccountSuspense || v.Ledger[0].Credit != domain.AccountCustomer {
		t.Errorf("entry = %+v", v.Ledger[0])
	}
	if v.Balances.Customer.String() != "75.4" || v.Balances.Suspense.String() != "75.4" {
		t.Errorf("balances = %+v", v.Balances)
	}
	// The note travelled with the event untouched.
	if string(v.Events[2].Payload) != `{"liability":"50.00","note":"card reported lost"}` {
		t.Errorf("payload = %s", v.Events[2].Payload)
	}

	closed := apply(t, svc, id, domain.EventClose)
	if len(closed.Ledger) != 2 || closed.Ledger[1].Kind != domain.PostingWriteOff || !closed.Balances.Suspense.IsZero() || closed.Balances.Loss.String() != "75.4" {
		t.Errorf("after close: ledger %+v balances %+v", closed.Ledger, closed.Balances)
	}
}

func TestBadLiabilityOrSettlementIsRefusedWithoutWriting(t *testing.T) {
	svc, store, _ := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "125.40")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	id := created.View.ID
	apply(t, svc, id, domain.EventOpenInvestigation)

	for _, raw := range []string{`{"liability":"50.01"}`, `{"liability":"-1"}`, `{"liability":"lots"}`} {
		_, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueRefund, Payload: json.RawMessage(raw)})
		if !errors.Is(err, domain.ErrInvalidLiability) || errs.KindOf(err) != errs.Unprocessable {
			t.Errorf("%s: err = %v", raw, err)
		}
	}
	view, _ := svc.GetDispute(apptest.Ctx(), id)
	if view.State != domain.StateInvestigating || len(view.Events) != 2 || len(view.Ledger) != 0 {
		t.Errorf("a refused refund left traces: state %s, %d events, %d postings", view.State, len(view.Events), len(view.Ledger))
	}

	apply(t, svc, id, domain.EventIssueRefund)
	_, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: domain.EventClose, Payload: json.RawMessage(`{"settlement":"MAYBE"}`)})
	if !errors.Is(err, domain.ErrInvalidSettlement) {
		t.Errorf("bad settlement: %v", err)
	}
	view, _ = svc.GetDispute(apptest.Ctx(), id)
	if view.State != domain.StateFastRefundIssued || len(view.Ledger) != 1 {
		t.Errorf("a refused close left traces: state %s, %d postings", view.State, len(view.Ledger))
	}
}

func TestProvisionalCreditRoundTrip(t *testing.T) {
	svc, store, _ := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "USD", "USD", "80")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	id := created.View.ID
	v := apply(t, svc, id, domain.EventOpenInvestigation, domain.EventIssueRefund, domain.EventFileChargeback,
		domain.EventAcknowledgeChargeback, domain.EventLoseChargeback, domain.EventReverseProvisionalCredit, domain.EventClose)
	if len(v.Ledger) != 2 || v.Ledger[1].Kind != domain.PostingProvisionalCreditReversal {
		t.Fatalf("ledger = %+v", v.Ledger)
	}
	if !v.Balances.Customer.IsZero() || !v.Balances.Suspense.IsZero() || !v.Balances.Loss.IsZero() || !v.Balances.Recovery.IsZero() {
		t.Errorf("balances after a reversed provisional credit = %+v", v.Balances)
	}
	// The counter saw one posting per movement; the suspense gauge is empty once everything cleared.
	balances, _ := store.SuspenseBalances(apptest.Ctx())
	for _, b := range balances {
		if !b.Balance.IsZero() {
			t.Errorf("suspense by regime = %+v", b)
		}
	}
}
