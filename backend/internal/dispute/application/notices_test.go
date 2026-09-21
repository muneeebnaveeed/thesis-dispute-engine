package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

func kindsOf(ns []application.NoticeView) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, string(n.Kind)+"/"+string(n.Channel))
	}
	return out
}

func TestNoticesFollowTheLifecycleAndTheRegime(t *testing.T) {
	svc, store, _ := clockService(t)
	store.Names[apptest.TenantA] = "OTP Bank"

	// PSD2: email only. Opening acknowledges; the refund and the close each write to the customer.
	eur := store.AddTransaction(domain.RailCard, "EUR", "EUR", "125.40")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: eur})
	if got := kindsOf(created.View.Notices); len(got) != 1 || got[0] != "ACKNOWLEDGEMENT/EMAIL" {
		t.Errorf("PSD2 at opening = %v", got)
	}
	v := apply(t, svc, created.View.ID, domain.EventOpenInvestigation, domain.EventSendQuestionnaire)
	if got := kindsOf(v.Notices); len(got) != 2 || got[1] != "QUESTIONNAIRE/EMAIL" {
		t.Errorf("after questionnaire = %v", got)
	}
	_, doc, err := svc.GetNotice(apptest.Ctx(), created.View.ID, v.Notices[1].ID)
	if err != nil || doc.Bank != "OTP Bank" || len(doc.Paragraphs) < 8 || !strings.Contains(doc.Paragraphs[1], "recognise the merchant") {
		t.Errorf("questionnaire notice = %+v %v", doc, err)
	}

	// Reg E: written notices, so a letter accompanies every consequential email; the acknowledgement clock is met
	// by the acknowledgement itself, at opening.
	usd := store.AddTransaction(domain.RailCard, "USD", "USD", "80")
	regE, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: usd})
	if got := kindsOf(regE.View.Notices); strings.Join(got, ",") != "ACKNOWLEDGEMENT/EMAIL,ACKNOWLEDGEMENT/LETTER" {
		t.Errorf("Reg E at opening = %v", got)
	}
	done := apply(t, svc, regE.View.ID, domain.EventOpenInvestigation, domain.EventIssueRefund, domain.EventFileChargeback,
		domain.EventAcknowledgeChargeback, domain.EventLoseChargeback, domain.EventReverseProvisionalCredit, domain.EventClose)
	want := "ACKNOWLEDGEMENT/EMAIL,ACKNOWLEDGEMENT/LETTER,PROVISIONAL_CREDIT/EMAIL,PROVISIONAL_CREDIT/LETTER,REVERSAL/EMAIL,REVERSAL/LETTER,RESOLUTION/EMAIL,RESOLUTION/LETTER"
	if got := strings.Join(kindsOf(done.Notices), ","); got != want {
		t.Errorf("Reg E lifecycle notices = %s", got)
	}
	_, resolution, _ := svc.GetNotice(apptest.Ctx(), regE.View.ID, done.Notices[len(done.Notices)-1].ID)
	if !strings.Contains(resolution.Paragraphs[0], "no error occurred") || !strings.Contains(resolution.Paragraphs[0], "reversed") {
		t.Errorf("resolution after a reversal: %v", resolution.Paragraphs)
	}
	for _, n := range done.Notices {
		if n.Channel == domain.ChannelLetter && n.SentAt == nil {
			t.Errorf("letter %d not ready at once", n.ID)
		}
		if n.Channel == domain.ChannelEmail && n.SentAt != nil {
			t.Errorf("email %d marked sent before any dispatcher ran", n.ID)
		}
	}

	// Reg Z: the acknowledgement clock is met by the acknowledgement notice, not by opening the investigation.
	cc := store.AddTransaction(domain.RailCreditCard, "USD", "USD", "40")
	regZ, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: cc})
	if got := byKind(regZ.View, domain.DeadlineAcknowledge, 0).Status; got != domain.DeadlineMet {
		t.Errorf("Reg Z acknowledgement clock at opening = %s", got)
	}
}

// recorder is a Mailer that keeps what it was given and can fail on demand.
type recorder struct {
	mu   sync.Mutex
	sent []application.Mail
	fail bool
}

func (r *recorder) Send(_ context.Context, m application.Mail) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("relay refused")
	}
	r.sent = append(r.sent, m)
	return nil
}

func TestDispatcherDrainsTheOutboxAndRetriesFailures(t *testing.T) {
	svc, store, _ := clockService(t)
	mailer := &recorder{fail: true}
	d := application.NewDispatcher(store, mailer, application.RenderMail, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.Run(ctx)

	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "10")
	created, _ := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	d.Kick()
	time.Sleep(100 * time.Millisecond)
	view, _ := svc.GetDispute(apptest.Ctx(), created.View.ID)
	if n := view.Notices[0]; n.SentAt != nil || n.Error == nil || !strings.Contains(*n.Error, "relay refused") {
		t.Fatalf("after a failed attempt = %+v", n)
	}

	mailer.mu.Lock()
	mailer.fail = false
	mailer.mu.Unlock()
	d.Kick()
	time.Sleep(100 * time.Millisecond)
	view, _ = svc.GetDispute(apptest.Ctx(), created.View.ID)
	if n := view.Notices[0]; n.SentAt == nil || n.Error != nil {
		t.Fatalf("after the retry = %+v", n)
	}
	mailer.mu.Lock()
	defer mailer.mu.Unlock()
	if len(mailer.sent) != 1 || mailer.sent[0].To != "holder@example.com" || !strings.Contains(mailer.sent[0].Text, "Dear Test Holder") || !strings.Contains(mailer.sent[0].HTML, "<h1>") {
		t.Errorf("mail = %+v", mailer.sent)
	}
}
