package application_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

var secondAnalyst = apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "analyst-two", Display: "two@example.test"})

// wonRegE is a Reg E dispute the lead investigated and won, one step before its final credit.
func wonRegE(t *testing.T) (*application.Service, uuid.UUID) {
	t.Helper()
	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "USD", "USD", "40.00")
	created, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	apply(t, svc, created.View.ID, domain.EventOpenInvestigation, domain.EventIssueRefund, domain.EventFileChargeback,
		domain.EventAcknowledgeChargeback, domain.EventWinChargeback)
	return svc, created.View.ID
}

func TestNoPrincipalIsRefused(t *testing.T) {
	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	_, err := svc.CreateDispute(tenant.WithID(context.Background(), apptest.TenantA), application.CreateDisputeInput{TransactionID: txn})
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
}

func TestSeparationOfDuties(t *testing.T) {
	svc, id := wonRegE(t)
	_, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueFinalCredit})
	if !errors.Is(err, application.ErrForbidden) || !strings.Contains(err.Error(), "sod-final-credit") {
		t.Fatalf("investigator issued final credit: %v", err)
	}
	res, err := svc.ApplyEvent(secondAnalyst, application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueFinalCredit})
	if err != nil {
		t.Fatalf("second analyst: %v", err)
	}
	last := res.View.Events[len(res.View.Events)-1]
	if !strings.Contains(string(last.Payload), `"authorizedBy":["grant:general/junior/ISSUE_FINAL_CREDIT"]`) {
		t.Fatalf("payload = %s", last.Payload)
	}
}

func TestAllowedEventsExcludeForbidden(t *testing.T) {
	svc, id := wonRegE(t)
	mine, err := svc.GetDispute(apptest.Ctx(), id)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(mine.AllowedEvents, domain.EventIssueFinalCredit) {
		t.Fatalf("investigator offered final credit: %v", mine.AllowedEvents)
	}
	theirs, err := svc.GetDispute(secondAnalyst, id)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(theirs.AllowedEvents, domain.EventIssueFinalCredit) {
		t.Fatalf("second analyst not offered final credit: %v", theirs.AllowedEvents)
	}
}

func TestKeysCannotActForPeople(t *testing.T) {
	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	key := apptest.CtxAs(principal.Principal{Kind: principal.Key, ID: uuid.NewString(), Display: "key:tk_x"})
	created, err := svc.CreateDispute(key, application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatalf("key create: %v", err)
	}
	_, err = svc.Upload(key, application.UploadInput{DisputeID: created.View.ID, Filename: "r.pdf", ContentType: "application/pdf", Content: []byte("%PDF")})
	if !errors.Is(err, application.ErrForbidden) || !strings.Contains(err.Error(), "keys-not-people") {
		t.Fatalf("key upload: %v", err)
	}
}

func TestTenantAdministrationNeedsTheRole(t *testing.T) {
	svc, _ := newService(t)
	logo := application.Image{Content: []byte("\x89PNG"), ContentType: "image/png"}
	if _, err := svc.PutTenantLogo(secondAnalyst, logo); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("non-admin logo: %v", err)
	}
	if _, err := svc.ListTemplateSettings(secondAnalyst); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("non-admin templates: %v", err)
	}
	if err := svc.Authorize(secondAnalyst, application.ActionManageKeys); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("non-admin keys: %v", err)
	}
	if _, err := svc.PutTenantLogo(apptest.Ctx(), logo); err != nil {
		t.Fatalf("admin logo: %v", err)
	}
}

// An idempotency key is the caller's own: another principal reusing it is a new request, not a replay of a
// response they were never authorized to see.
func TestReplayIsPerCaller(t *testing.T) {
	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	body := []byte(`{"transactionId":"` + txn.String() + `"}`)
	idem := application.Idempotency{Key: "shared", RequestBody: body}
	first, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn, Idempotency: idem})
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.CreateDispute(secondAnalyst, application.CreateDisputeInput{TransactionID: txn, Idempotency: idem})
	if err != nil {
		t.Fatal(err)
	}
	if again.Replayed || again.View.ID == first.View.ID {
		t.Fatalf("second caller got the first caller's response: replayed=%v", again.Replayed)
	}
	same, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn, Idempotency: idem})
	if err != nil || !same.Replayed || same.View.ID != first.View.ID {
		t.Fatalf("same caller: replayed=%v err=%v", same.Replayed, err)
	}
}

// Investigations opened before actor ids were recorded name nobody, so separation of duties cannot be checked; the
// final credit is refused to every analyst rather than allowed to all of them. A tenant key can still finish it.
func TestUnrecordedInvestigatorFailsClosed(t *testing.T) {
	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "USD", "USD", "40.00")
	created, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	id := created.View.ID
	apply(t, svc, id, domain.EventOpenInvestigation, domain.EventIssueRefund, domain.EventFileChargeback,
		domain.EventAcknowledgeChargeback, domain.EventWinChargeback)
	for i, e := range store.Events[id] {
		if e.Event == domain.EventOpenInvestigation {
			store.Events[id][i].ActorID = ""
		}
	}
	_, err = svc.ApplyEvent(secondAnalyst, application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueFinalCredit})
	if !errors.Is(err, application.ErrForbidden) || !strings.Contains(err.Error(), "sod-unrecorded-investigator") {
		t.Fatalf("analyst on an unrecorded investigation: %v", err)
	}
	key := apptest.CtxAs(principal.Principal{Kind: principal.Key, ID: uuid.NewString(), Display: "key:tk_x"})
	if _, err := svc.ApplyEvent(key, application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueFinalCredit}); err != nil {
		t.Fatalf("tenant key: %v", err)
	}
}
