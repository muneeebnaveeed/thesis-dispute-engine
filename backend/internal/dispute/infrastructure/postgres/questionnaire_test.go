//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// The reason persists, the questionnaire round-trips as JSON under RLS, and refused answers leave it unanswered.
func TestQuestionnaireThroughPostgres(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	svc, err := application.NewService(disputepg.NewStore(app), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctxA := tenant.WithID(context.Background(), apptest.TenantA)
	ctxB := tenant.WithID(context.Background(), apptest.TenantB)
	txn := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	seedFor(t, owner, apptest.TenantB, domain.RailCard, "EUR")

	created, err := svc.CreateDispute(ctxA, application.CreateDisputeInput{TransactionID: txn, Reason: "DUPLICATE"})
	if err != nil {
		t.Fatal(err)
	}
	id := created.View.ID
	for _, e := range []domain.Event{domain.EventOpenInvestigation, domain.EventSendQuestionnaire} {
		if _, err := svc.ApplyEvent(ctxA, application.ApplyEventInput{DisputeID: id, Event: e}); err != nil {
			t.Fatal(err)
		}
	}
	view, _ := svc.GetDispute(ctxA, id)
	if view.Reason != domain.ReasonDuplicate || view.Questionnaire == nil || len(view.Questionnaire.Questions) != 3 {
		t.Fatalf("after send: %s %+v", view.Reason, view.Questionnaire)
	}
	if _, err := svc.GetDispute(ctxB, id); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-tenant: %v", err)
	}

	if _, err := svc.ApplyEvent(ctxA, application.ApplyEventInput{DisputeID: id, Event: domain.EventReceiveQuestionnaire,
		Payload: json.RawMessage(`{"answers":{"original_on":"soon"}}`)}); !errors.Is(err, domain.ErrInvalidAnswers) {
		t.Fatalf("bad answers: %v", err)
	}
	view, _ = svc.GetDispute(ctxA, id)
	if view.State != domain.StateQuestionnaireSent || view.Questionnaire.ReceivedAt != nil {
		t.Errorf("refused answers left traces: %s %+v", view.State, view.Questionnaire)
	}
	res, err := svc.ApplyEvent(ctxA, application.ApplyEventInput{DisputeID: id, Event: domain.EventReceiveQuestionnaire,
		Payload: json.RawMessage(`{"answers":{"original_on":"2026-09-10","same_merchant":"no"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	q := res.View.Questionnaire
	if q.Answers["same_merchant"] != "no" || q.ReceivedAt == nil || len(q.Inconsistencies) != 1 {
		t.Errorf("after answers = %+v", q)
	}
	// The list carries the reason too.
	page, _ := svc.ListDisputes(ctxA, application.ListQuery{Limit: 5})
	if len(page.Items) != 1 || page.Items[0].Reason != domain.ReasonDuplicate {
		t.Errorf("list = %+v", page.Items)
	}
}
