package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

func newService(t *testing.T) (*application.Service, *apptest.MemStore) {
	t.Helper()
	store := apptest.NewMemStore()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	svc, err := application.NewService(store, func() time.Time { now = now.Add(time.Minute); return now })
	if err != nil {
		t.Fatal(err)
	}
	return svc, store
}

func TestCreateDerivesRegimeAndLogsOpened(t *testing.T) {
	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "125.40")

	res, err := svc.CreateDispute(context.Background(), application.CreateDisputeInput{TransactionID: txn, Actor: "customer"})
	if err != nil {
		t.Fatal(err)
	}
	v := res.View
	if v.Regime != domain.RegimeEUPSD2Card || v.State != domain.StateInitiated || v.Version != 1 {
		t.Errorf("view = %+v", v)
	}
	if v.ID.Version() != 7 {
		t.Errorf("id version = %d, want UUIDv7", v.ID.Version())
	}
	if len(v.Events) != 1 || v.Events[0].Seq != 1 || v.Events[0].Event != "OPENED" || v.Events[0].ToState != domain.StateInitiated {
		t.Errorf("events = %+v", v.Events)
	}
	if v.DisputedAmount.String() != "125.4" || v.Currency != "EUR" {
		t.Errorf("amount = %s %s", v.DisputedAmount, v.Currency)
	}
	if len(v.AllowedEvents) == 0 {
		t.Error("allowed events empty")
	}
}

func TestCreateRejectsUnknownTransactionAndRegimelessRail(t *testing.T) {
	svc, store := newService(t)
	if _, err := svc.CreateDispute(context.Background(), application.CreateDisputeInput{TransactionID: uuid.New()}); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("unknown transaction: err = %v", err)
	}
	txn := store.AddTransaction(domain.RailSEPADD, "USD", "USD", "10")
	if _, err := svc.CreateDispute(context.Background(), application.CreateDisputeInput{TransactionID: txn}); !errors.Is(err, domain.ErrNoRegime) {
		t.Errorf("SEPA in USD: err = %v", err)
	}
}

func TestApplyAdvancesVersionAndLog(t *testing.T) {
	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "USD", "USD", "50")
	created, _ := svc.CreateDispute(context.Background(), application.CreateDisputeInput{TransactionID: txn})

	res, err := svc.ApplyEvent(context.Background(), application.ApplyEventInput{
		DisputeID: created.View.ID, Event: domain.EventOpenInvestigation, Actor: "analyst:1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.View.State != domain.StateInvestigating || res.View.Version != 2 || len(res.View.Events) != 2 {
		t.Errorf("view = %+v", res.View)
	}
	last := res.View.Events[1]
	if last.Seq != 2 || last.FromState != domain.StateInitiated || last.ToState != domain.StateInvestigating || last.Actor != "analyst:1" {
		t.Errorf("event = %+v", last)
	}
	if string(last.Payload) != "{}" {
		t.Errorf("payload = %s, want {}", last.Payload)
	}
}

func TestApplyRejectsInvalidTransitionWithoutWriting(t *testing.T) {
	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "USD", "USD", "50")
	created, _ := svc.CreateDispute(context.Background(), application.CreateDisputeInput{TransactionID: txn})

	_, err := svc.ApplyEvent(context.Background(), application.ApplyEventInput{DisputeID: created.View.ID, Event: domain.EventWinChargeback})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("err = %v", err)
	}
	after, _ := svc.GetDispute(context.Background(), created.View.ID)
	if after.Version != 1 || len(after.Events) != 1 {
		t.Errorf("rejected event left a trace: %+v", after)
	}
}

func TestIdempotentReplayAndMismatch(t *testing.T) {
	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "9.99")
	body := []byte(`{"transactionId":"` + txn.String() + `"}`)

	first, err := svc.CreateDispute(context.Background(), application.CreateDisputeInput{
		TransactionID: txn, Idempotency: application.Idempotency{Key: "k1", RequestBody: body},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateDispute(context.Background(), application.CreateDisputeInput{
		TransactionID: txn, Idempotency: application.Idempotency{Key: "k1", RequestBody: body},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || second.View.ID != first.View.ID {
		t.Errorf("replay = %+v, want the first dispute back", second)
	}
	if len(store.Disputes) != 1 {
		t.Errorf("disputes = %d, want 1", len(store.Disputes))
	}

	_, err = svc.CreateDispute(context.Background(), application.CreateDisputeInput{
		TransactionID: txn, Idempotency: application.Idempotency{Key: "k1", RequestBody: []byte(`{"transactionId":"other"}`)},
	})
	if !errors.Is(err, application.ErrIdempotencyReuse) {
		t.Errorf("mismatched body: err = %v", err)
	}
	if len(store.Disputes) != 1 {
		t.Errorf("mismatch created a dispute")
	}
}

func TestApplyRollsBackWhenTheLogRejectsTheEvent(t *testing.T) {
	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "USD", "USD", "50")
	created, _ := svc.CreateDispute(context.Background(), application.CreateDisputeInput{TransactionID: txn})

	// A stray entry already holds seq 2: the append fails after the state update, and both must roll back.
	store.Events[created.View.ID] = append(store.Events[created.View.ID], application.EventRecord{Seq: 2, Event: "GHOST"})

	_, err := svc.ApplyEvent(context.Background(), application.ApplyEventInput{DisputeID: created.View.ID, Event: domain.EventOpenInvestigation})
	if !errors.Is(err, application.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	if rec := store.Disputes[created.View.ID]; rec.State != domain.StateInitiated || rec.Version != 1 {
		t.Errorf("state update survived the rollback: %+v", rec)
	}
}
