package application_test

import (
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

func TestActorComesFromPrincipal(t *testing.T) {
	svc, store := newService(t)
	ctx := apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "sub-7", Display: "ana@bank.example"})
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	res, err := svc.CreateDispute(ctx, application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApplyEvent(ctx, application.ApplyEventInput{DisputeID: res.View.ID, Event: domain.EventOpenInvestigation}); err != nil {
		t.Fatal(err)
	}
	for _, ev := range store.Events[res.View.ID] {
		if ev.Actor != "ana@bank.example" || ev.ActorID != "sub-7" {
			t.Fatalf("%s actor = %q/%q", ev.Event, ev.Actor, ev.ActorID)
		}
	}
}
