package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/mockcore"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

func coreService(t *testing.T, core application.BankingCore, settings string) (*application.Service, *apptest.MemStore) {
	t.Helper()
	store := apptest.NewMemStore()
	store.Cores[apptest.TenantA] = application.CoreConfig{Kind: mockcore.Kind, Settings: json.RawMessage(settings)}
	svc, err := application.NewService(store, nil, application.WithCore(application.CoreRouter{Adapters: map[string]application.BankingCore{mockcore.Kind: core}}),
		application.WithCoreTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	return svc, store
}

func TestCreditGoesToTheCoreAndItsReceiptIsKept(t *testing.T) {
	svc, store := coreService(t, quietCore(), `{}`)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "125.40")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	v := apply(t, svc, created.View.ID, domain.EventOpenInvestigation, domain.EventIssueRefund, domain.EventClose)
	if len(v.Ledger) != 2 {
		t.Fatalf("ledger = %+v", v.Ledger)
	}
	credit, writeOff := v.Ledger[0], v.Ledger[1]
	if credit.Core == nil || credit.Core.ResponseCode != "00" || len(credit.Core.RRN) != 12 {
		t.Errorf("credit receipt = %+v", credit.Core)
	}
	if writeOff.Core != nil {
		t.Errorf("an internal posting reached the core: %+v", writeOff.Core)
	}
}

func TestADeclineRollsTheTransitionBack(t *testing.T) {
	svc, store := coreService(t, quietCore(), `{"declineAbove":"1850"}`)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "1899.00")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	id := created.View.ID
	apply(t, svc, id, domain.EventOpenInvestigation)
	_, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueRefund})
	if !errors.Is(err, application.ErrCoreDeclined) || errs.KindOf(err) != errs.Unprocessable {
		t.Fatalf("err = %v", err)
	}
	if msg := errs.UserMessage(err); msg == "" || !contains(msg, "61") {
		t.Errorf("message = %q, want the response code in it", msg)
	}
	v, _ := svc.GetDispute(apptest.Ctx(), id)
	if v.State != domain.StateInvestigating || len(v.Events) != 2 || len(v.Ledger) != 0 {
		t.Errorf("after a decline: %s, %d events, %d postings", v.State, len(v.Events), len(v.Ledger))
	}
	// With the liability bringing the credit under the limit, the same transition goes through.
	res, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueRefund, Payload: json.RawMessage(`{"liability":"50"}`)})
	if err != nil {
		t.Fatalf("second attempt: %v (the mock must not remember a decline)", err)
	}
	if res.View.State != domain.StateFastRefundIssued || res.View.Ledger[0].Amount.String() != "1849" {
		t.Errorf("view = %s %+v", res.View.State, res.View.Ledger)
	}
}

// slowThenFast answers nothing the first time (the call times out) and normally afterwards.
type slowThenFast struct {
	inner application.BankingCore
	calls atomic.Int32
}

func (s *slowThenFast) Post(ctx context.Context, cfg application.CoreConfig, ins application.CoreInstruction) (application.CoreReceipt, error) {
	if s.calls.Add(1) == 1 {
		// The core did the work but the answer never arrived.
		if _, err := s.inner.Post(context.Background(), cfg, ins); err != nil {
			return application.CoreReceipt{}, err
		}
		<-ctx.Done()
		return application.CoreReceipt{}, ctx.Err()
	}
	return s.inner.Post(ctx, cfg, ins)
}

func TestARetryAfterSilenceIsADuplicateNotADoublePayment(t *testing.T) {
	core := &slowThenFast{inner: quietCore()}
	svc, store := coreService(t, core, `{}`)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "125.40")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	id := created.View.ID
	apply(t, svc, id, domain.EventOpenInvestigation)

	body := []byte(`{"event":"ISSUE_REFUND"}`)
	in := application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueRefund, Idempotency: application.Idempotency{Key: "k1", RequestBody: body}}
	_, err := svc.ApplyEvent(apptest.Ctx(), in)
	if !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("first attempt: %v, want unavailable", err)
	}
	v, _ := svc.GetDispute(apptest.Ctx(), id)
	if v.State != domain.StateInvestigating || len(v.Ledger) != 0 {
		t.Fatalf("the silent attempt left traces: %s %d", v.State, len(v.Ledger))
	}
	res, err := svc.ApplyEvent(apptest.Ctx(), in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Replayed || len(res.View.Ledger) != 1 || res.View.Ledger[0].Core == nil || res.View.Ledger[0].Core.ResponseCode != "94" {
		t.Errorf("retry = replayed %v, ledger %+v", res.Replayed, res.View.Ledger)
	}
	if core.calls.Load() != 2 {
		t.Errorf("core calls = %d", core.calls.Load())
	}
}

func TestATenantWithoutACoreBooksOnly(t *testing.T) {
	svc, store, _ := clockService(t) // no core configured, default router
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "10")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	v := apply(t, svc, created.View.ID, domain.EventOpenInvestigation, domain.EventIssueRefund)
	if v.Ledger[0].Core == nil || v.Ledger[0].Core.RRN != "" || v.Ledger[0].Core.ResponseCode != "00" {
		t.Errorf("book-only receipt = %+v", v.Ledger[0].Core)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func quietCore() *mockcore.Core { return mockcore.New(slog.New(slog.NewTextHandler(io.Discard, nil))) }
