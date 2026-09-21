package application_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

func TestReasonDefaultsAndIsValidated(t *testing.T) {
	svc, store, _ := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "10")
	res, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	if err != nil || res.View.Reason != domain.ReasonUnauthorised || res.View.Questionnaire != nil {
		t.Errorf("default = %s %+v %v", res.View.Reason, res.View.Questionnaire, err)
	}
	res, err = svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn, Reason: "NOT_RECEIVED"})
	if err != nil || res.View.Reason != domain.ReasonNotReceived {
		t.Errorf("explicit = %s %v", res.View.Reason, err)
	}
	if _, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn, Reason: "VIBES"}); !errors.Is(err, domain.ErrUnknownReason) {
		t.Errorf("unknown reason: %v", err)
	}
}

func TestQuestionnaireIsSentAnsweredAndChecked(t *testing.T) {
	svc, store, now := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "10")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	id := created.View.ID

	sent := apply(t, svc, id, domain.EventOpenInvestigation, domain.EventSendQuestionnaire)
	q := sent.Questionnaire
	if q == nil || q.Reason != domain.ReasonUnauthorised || len(q.Questions) != 7 || q.Answers != nil || q.ReceivedAt != nil || !q.SentAt.Equal(*now) {
		t.Fatalf("after send = %+v", q)
	}

	// Incomplete or malformed answers are refused per question and the dispute stays QUESTIONNAIRE_SENT.
	_, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: domain.EventReceiveQuestionnaire,
		Payload: json.RawMessage(`{"answers":{"recognise_merchant":"perhaps"}}`)})
	if !errors.Is(err, domain.ErrInvalidAnswers) {
		t.Fatalf("bad answers: %v", err)
	}
	if n := len(errs.FieldsOf(err)); n != 6 { // one malformed, five required missing
		t.Errorf("field errors = %d: %+v", n, errs.FieldsOf(err))
	}
	view, _ := svc.GetDispute(apptest.Ctx(), id)
	if view.State != domain.StateQuestionnaireSent || view.Questionnaire.ReceivedAt != nil {
		t.Errorf("after refused answers: %s %+v", view.State, view.Questionnaire)
	}

	*now = now.Add(time.Hour)
	res, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: id, Event: domain.EventReceiveQuestionnaire,
		Payload: json.RawMessage(`{"answers":{"recognise_merchant":"yes","card_in_possession":"yes","shared_credentials":"no",
			"prior_disputes_merchant":"no","noticed_on":"2026-09-20","police_report":"no","details":"it was me after all?"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	got := res.View.Questionnaire
	if res.View.State != domain.StateQuestionnaireReceived || got.Answers["details"] != "it was me after all?" || got.ReceivedAt == nil || !got.ReceivedAt.Equal(*now) {
		t.Errorf("after answers = %s %+v", res.View.State, got)
	}
	if len(got.Inconsistencies) != 1 {
		t.Errorf("inconsistencies = %v", got.Inconsistencies)
	}
	// The answers are also on the event, verbatim, as every payload is.
	if len(res.View.Events) != 4 || !json.Valid(res.View.Events[3].Payload) {
		t.Errorf("events = %+v", res.View.Events)
	}
}

func TestReceivingWithoutSendingIsRefused(t *testing.T) {
	// The state machine already forbids it; this pins the answer to the client.
	svc, store, _ := clockService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "10")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	apply(t, svc, created.View.ID, domain.EventOpenInvestigation)
	_, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: created.View.ID, Event: domain.EventReceiveQuestionnaire})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("err = %v", err)
	}
}
