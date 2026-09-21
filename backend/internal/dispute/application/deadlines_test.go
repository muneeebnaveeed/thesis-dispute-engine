package application_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

// clockService is a service whose clock the test moves by hand.
func clockService(t *testing.T) (*application.Service, *apptest.MemStore, *time.Time) {
	t.Helper()
	store := apptest.NewMemStore()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC) // a Monday
	svc, err := application.NewService(store, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return svc, store, &now
}

func apply(t *testing.T, svc *application.Service, id uuid.UUID, events ...domain.Event) application.DisputeView {
	t.Helper()
	var v application.DisputeView
	for _, e := range events {
		res, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: e, Actor: "analyst"})
		if err != nil {
			t.Fatalf("%s: %v", e, err)
		}
		v = res.View
	}
	return v
}

func byKind(v application.DisputeView, kind domain.DeadlineKind, cycle int) application.DeadlineView {
	for _, d := range v.Deadlines {
		if d.Kind == kind && d.Cycle == cycle {
			return d
		}
	}
	return application.DeadlineView{}
}

func TestCreateStartsTheRegimeClocksInTheTenantCalendar(t *testing.T) {
	svc, store, _ := clockService(t)
	cal, _ := domain.NewCalendar("Europe/Budapest", []string{"2026-09-22"}) // pretend Tuesday is a holiday
	store.Calendars[apptest.TenantA] = cal
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "125.40")

	res, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn, Actor: "customer"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.View.Deadlines) != 2 {
		t.Fatalf("deadlines = %+v", res.View.Deadlines)
	}
	refund := byKind(res.View, domain.DeadlineRefund, 0)
	// PSD2 art. 73: end of the following business day; the holiday pushes it to Wednesday, Budapest time.
	if got := refund.DueAt.In(cal.Location).Format("2006-01-02 15:04"); got != "2026-09-23 23:59" {
		t.Errorf("refund due %s", got)
	}
	if refund.Status != domain.DeadlineRunning || refund.Basis == "" {
		t.Errorf("refund = %+v", refund)
	}
}

func TestRefundClockIsMetLateOrBreachedByTime(t *testing.T) {
	svc, store, now := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "125.40")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	id := created.View.ID

	*now = now.Add(2 * time.Hour)
	v := apply(t, svc, id, domain.EventOpenInvestigation, domain.EventIssueRefund)
	refund := byKind(v, domain.DeadlineRefund, 0)
	if refund.Status != domain.DeadlineMet || refund.MetAt == nil || !refund.MetAt.Equal(*now) {
		t.Errorf("refund after a prompt credit = %+v", refund)
	}

	// A second dispute where nobody acts: the clock is BREACHED once the due time passes, and LATE when finally met.
	created2, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	*now = now.Add(72 * time.Hour)
	view, err := svc.GetDispute(apptest.Ctx(), created2.View.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := byKind(view, domain.DeadlineRefund, 0).Status; got != domain.DeadlineBreached {
		t.Errorf("status after due = %s", got)
	}
	late := apply(t, svc, created2.View.ID, domain.EventOpenInvestigation, domain.EventIssueRefund)
	if got := byKind(late, domain.DeadlineRefund, 0).Status; got != domain.DeadlineLate {
		t.Errorf("status after a late credit = %s", got)
	}
}

func TestClosingWithoutCreditVoidsTheRefundClockAndMeetsResolution(t *testing.T) {
	svc, store, _ := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "USD", "USD", "50") // Reg E: provisional credit clock
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	v := apply(t, svc, created.View.ID, domain.EventOpenInvestigation, domain.EventClose)
	if got := byKind(v, domain.DeadlineRefund, 0).Status; got != domain.DeadlineVoid {
		t.Errorf("refund clock after closing without credit = %s", got)
	}
	if got := byKind(v, domain.DeadlineResolution, 0).Status; got != domain.DeadlineMet {
		t.Errorf("resolution clock after closing = %s", got)
	}
}

func TestAppealRestartsTheResolutionClock(t *testing.T) {
	svc, store, now := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "USD", "USD", "50")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	apply(t, svc, created.View.ID, domain.EventOpenInvestigation, domain.EventClose)
	*now = now.Add(24 * time.Hour)
	v := apply(t, svc, created.View.ID, domain.EventAppeal)
	if len(v.Deadlines) != 3 {
		t.Fatalf("deadlines after appeal = %+v", v.Deadlines)
	}
	second := byKind(v, domain.DeadlineResolution, 1)
	if second.Status != domain.DeadlineRunning || !second.StartedAt.Equal(*now) || second.DueAt.Sub(*now) < 44*24*time.Hour {
		t.Errorf("appeal resolution clock = %+v", second)
	}
	if got := byKind(v, domain.DeadlineResolution, 0).Status; got != domain.DeadlineMet {
		t.Errorf("first resolution clock = %s", got)
	}
}

func TestListShowsTheNextClockAndFiltersOverdue(t *testing.T) {
	svc, store, now := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "10")
	prompt, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	apply(t, svc, prompt.View.ID, domain.EventOpenInvestigation, domain.EventIssueRefund)
	idle, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})

	*now = now.Add(72 * time.Hour)
	page, err := svc.ListDisputes(apptest.Ctx(), application.ListQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	next := map[uuid.UUID]*application.DeadlineView{}
	for _, it := range page.Items {
		next[it.ID] = it.NextDeadline
	}
	if d := next[prompt.View.ID]; d == nil || d.Kind != domain.DeadlineResolution || d.Status != domain.DeadlineRunning {
		t.Errorf("next for the credited dispute = %+v", d)
	}
	if d := next[idle.View.ID]; d == nil || d.Kind != domain.DeadlineRefund || d.Status != domain.DeadlineBreached {
		t.Errorf("next for the idle dispute = %+v", d)
	}

	overdue, err := svc.ListDisputes(apptest.Ctx(), application.ListQuery{Limit: 10, Overdue: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(overdue.Items) != 1 || overdue.Items[0].ID != idle.View.ID {
		t.Errorf("overdue page = %+v", overdue.Items)
	}
}
