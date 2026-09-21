package mockcore

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

func instruction(amount string, credit bool) application.CoreInstruction {
	return application.CoreInstruction{
		Reference: "dispute:" + uuid.NewString() + ":3:FAST_REFUND", DisputeID: uuid.New(), AccountID: uuid.New(),
		Kind: domain.PostingFastRefund, CreditAccount: credit, Amount: decimal.RequireFromString(amount), Currency: "EUR",
		RequestedAt: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}
}

func cfg(settings string) application.CoreConfig {
	return application.CoreConfig{Kind: Kind, Settings: json.RawMessage(settings)}
}

func TestApprovesAndDetectsDuplicates(t *testing.T) {
	c := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ins := instruction("125.40", true)
	first, err := c.Post(context.Background(), cfg(`{}`), ins)
	if err != nil || !first.Approved || first.ResponseCode != "00" || len(first.RRN) != 12 || first.AuthCode == "" {
		t.Fatalf("first = %+v %v", first, err)
	}
	again, err := c.Post(context.Background(), cfg(`{}`), ins)
	if err != nil || !again.Approved || !again.Duplicate || again.ResponseCode != "94" || again.RRN != first.RRN || again.AuthCode != first.AuthCode {
		t.Errorf("repeat = %+v %v", again, err)
	}
	// A different reference is a different instruction, even for the same amount and account.
	other := ins
	other.Reference += ":again"
	if r, _ := c.Post(context.Background(), cfg(`{}`), other); r.Duplicate || r.RRN == first.RRN {
		t.Errorf("distinct reference treated as duplicate: %+v", r)
	}
}

func TestDeclinesPerTenantSettings(t *testing.T) {
	c := New(nil)
	if r, _ := c.Post(context.Background(), cfg(`{"declineAbove":"1000"}`), instruction("1899.00", true)); r.Approved || r.ResponseCode != "61" {
		t.Errorf("large credit = %+v", r)
	}
	if r, _ := c.Post(context.Background(), cfg(`{"declineAbove":"1000"}`), instruction("999.99", true)); !r.Approved {
		t.Errorf("credit under the limit = %+v", r)
	}
	debit := instruction("80", false)
	settings := `{"insufficientFundsAccounts":["` + debit.AccountID.String() + `"]}`
	if r, _ := c.Post(context.Background(), cfg(settings), debit); r.Approved || r.ResponseCode != "51" {
		t.Errorf("debit on an empty account = %+v", r)
	}
	// A declined instruction is not remembered: the same reference can be retried once the cause is gone.
	if r, _ := c.Post(context.Background(), cfg(`{}`), debit); !r.Approved || r.Duplicate {
		t.Errorf("retry after decline = %+v", r)
	}
}

func TestOutageAndTimeoutAreUnavailable(t *testing.T) {
	c := New(nil)
	if _, err := c.Post(context.Background(), cfg(`{"outage":true}`), instruction("1", true)); !errors.Is(err, application.ErrUnavailable) {
		t.Errorf("outage: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.Post(ctx, cfg(`{"latencyMs":500}`), instruction("1", true)); !errors.Is(err, application.ErrUnavailable) {
		t.Errorf("timeout: %v", err)
	}
}

func TestMessageFields(t *testing.T) {
	c := New(nil)
	ins := instruction("125.40", true)
	m := c.request(ins)
	if m.MTI != "0200" || m.ProcessingCode != "200000" || m.Amount != "000000012540" || m.Currency != "978" || m.Transmission != "0922100000" {
		t.Errorf("message = %+v", m)
	}
	if d := c.request(instruction("80", false)); d.ProcessingCode != "010000" {
		t.Errorf("debit processing code = %s", d.ProcessingCode)
	}
}
