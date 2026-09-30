package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

func TestReassign(t *testing.T) {
	svc, store := newService(t)
	store.Access = map[uuid.UUID]*apptest.MemAccess{apptest.TenantA: twoTeams()}
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	created, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	id := created.View.ID
	junior := apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "junior-general", Display: "j@x"})
	if err := svc.Reassign(junior, id, "fraud"); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("junior reassign: %v", err)
	}
	if err := svc.Reassign(apptest.Ctx(), id, "nope"); !errors.Is(err, application.ErrUnknownTeam) {
		t.Fatalf("unknown team: %v", err)
	}
	if err := svc.Reassign(apptest.Ctx(), id, "fraud"); err != nil {
		t.Fatal(err)
	}
	rec := store.Disputes[id]
	if rec.Team != "fraud" || rec.Version != 2 {
		t.Fatalf("record = team %q version %d", rec.Team, rec.Version)
	}
	if _, err := svc.GetDispute(apptest.Ctx(), id); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("lead of general still sees it: %v", err)
	}
	evs := store.Events[id]
	last := evs[len(evs)-1]
	if last.Event != application.EventReassigned || last.ActorID != apptest.Lead.ID ||
		!strings.Contains(string(last.Payload), `"from":"general"`) || !strings.Contains(string(last.Payload), `"to":"fraud"`) {
		t.Fatalf("audit event = %+v %s", last, last.Payload)
	}
	fraud := apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "fraud-only", Display: "f@x"})
	if _, err := svc.GetDispute(fraud, id); err != nil {
		t.Fatalf("fraud lead cannot see it: %v", err)
	}
}
