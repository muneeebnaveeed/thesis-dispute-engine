// Package mockcore is a simulated core banking system behind the application's BankingCore port. It speaks in
// ISO 8583 field semantics (message type, processing code, amount in minor units, STAN, RRN, response code,
// currency code) so the adapter shape is the one a real core would need, and it is configured per tenant
// (tenants.settings.core) so two tenants can behave like two different banks: one that declines large
// credits, one that is slow, one that is down.
package mockcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// Kind is the value of tenants.settings.core.kind that selects this adapter.
const Kind = "mock"

// Settings is what a tenant may configure; every field is optional.
type Settings struct {
	// DeclineAbove refuses credits larger than this amount with response code 61 (exceeds amount limit).
	DeclineAbove *decimal.Decimal `json:"declineAbove"`
	// InsufficientFundsAccounts refuses debits (reversals) on these accounts with response code 51.
	InsufficientFundsAccounts []uuid.UUID `json:"insufficientFundsAccounts"`
	// LatencyMs is added to every answer, so a slow core is visible in traces and the timeout path is exercisable.
	LatencyMs int `json:"latencyMs"`
	// Outage makes every call fail without an answer, as a core that is down does.
	Outage bool `json:"outage"`
}

// Message is the ISO 8583 subset the mock exchanges; it is logged and traced so the thesis can show one.
type Message struct {
	MTI            string `json:"mti"`  // 0200 financial request, 0210 response
	ProcessingCode string `json:"de3"`  // 20xxxx credit to account, 01xxxx debit from account
	Amount         string `json:"de4"`  // 12 digits, minor units
	Transmission   string `json:"de7"`  // MMDDhhmmss, UTC
	STAN           string `json:"de11"` // 6 digits, per process
	RRN            string `json:"de37"` // 12 characters, derived from the instruction reference
	AuthCode       string `json:"de38,omitempty"`
	ResponseCode   string `json:"de39,omitempty"`
	Currency       string `json:"de49"`  // ISO 4217 numeric
	Account        string `json:"de102"` // account identification
}

// Core is the adapter; one per process, holding what each tenant's mock core has already seen.
type Core struct {
	log  *slog.Logger
	stan atomic.Uint32
	mu   sync.Mutex
	seen map[string]application.CoreReceipt // RRN -> first answer, for duplicate detection (DE39 94)
	now  func() time.Time
}

// New returns a mock core that logs each exchange through log.
func New(log *slog.Logger) *Core {
	if log == nil {
		log = slog.Default()
	}
	return &Core{log: log, seen: map[string]application.CoreReceipt{}, now: time.Now}
}

var currencyCodes = map[string]string{"EUR": "978", "USD": "840", "HUF": "348", "GBP": "826"}

// responseTexts are the ISO 8583 DE39 meanings the mock uses.
var responseTexts = map[string]string{
	"00": "approved",
	"51": "insufficient funds",
	"61": "exceeds amount limit",
	"94": "duplicate transmission",
	"96": "system malfunction",
}

// Post implements application.BankingCore.
func (c *Core) Post(ctx context.Context, cfg application.CoreConfig, ins application.CoreInstruction) (application.CoreReceipt, error) {
	var s Settings
	if len(cfg.Settings) > 0 {
		if err := json.Unmarshal(cfg.Settings, &s); err != nil {
			return application.CoreReceipt{}, errs.Wrap(application.ErrUnavailable, "mockcore: tenant core settings: %v", err)
		}
	}
	ctx, span := otel.Tracer("mockcore").Start(ctx, "core.post", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	req := c.request(ins)
	span.SetAttributes(attribute.String("iso8583.mti", req.MTI), attribute.String("iso8583.de3", req.ProcessingCode),
		attribute.String("iso8583.de37", req.RRN), attribute.String("iso8583.de4", req.Amount))
	started := c.now()
	if s.LatencyMs > 0 {
		select {
		case <-time.After(time.Duration(s.LatencyMs) * time.Millisecond):
		case <-ctx.Done():
			return application.CoreReceipt{}, errs.Wrap(application.ErrUnavailable, "mockcore: %v", ctx.Err())
		}
	}
	if s.Outage {
		c.log.WarnContext(ctx, "core outage", "rrn", req.RRN)
		return application.CoreReceipt{}, errs.Wrap(application.ErrUnavailable, "mockcore: outage configured for tenant")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if first, ok := c.seen[req.RRN]; ok {
		// The same instruction again: answer as before, marked duplicate, and move no money.
		dup := first
		dup.Duplicate, dup.ResponseCode, dup.ResponseText, dup.Latency = true, "94", responseTexts["94"], c.now().Sub(started)
		c.exchange(ctx, req, dup)
		return dup, nil
	}
	code := "00"
	switch {
	case ins.CreditAccount && s.DeclineAbove != nil && ins.Amount.GreaterThan(*s.DeclineAbove):
		code = "61"
	case !ins.CreditAccount && contains(s.InsufficientFundsAccounts, ins.AccountID):
		code = "51"
	}
	rcpt := application.CoreReceipt{RRN: req.RRN, ResponseCode: code, ResponseText: responseTexts[code], Approved: code == "00",
		ProcessedAt: c.now(), Latency: c.now().Sub(started)}
	if rcpt.Approved {
		rcpt.AuthCode = strings.ToUpper(req.RRN[:6])
		c.seen[req.RRN] = rcpt
	}
	c.exchange(ctx, req, rcpt)
	return rcpt, nil
}

func (c *Core) request(ins application.CoreInstruction) Message {
	proc := "010000"
	if ins.CreditAccount {
		proc = "200000"
	}
	sum := sha256.Sum256([]byte(ins.Reference))
	return Message{
		MTI: "0200", ProcessingCode: proc,
		Amount:       fmt.Sprintf("%012d", ins.Amount.Shift(2).IntPart()),
		Transmission: ins.RequestedAt.UTC().Format("0102150405"),
		STAN:         fmt.Sprintf("%06d", c.stan.Add(1)%1_000_000),
		RRN:          strings.ToUpper(hex.EncodeToString(sum[:6])),
		Currency:     currencyCodes[ins.Currency],
		Account:      ins.AccountID.String(),
	}
}

// exchange records the request and response pair the way a switch log would.
func (c *Core) exchange(ctx context.Context, req Message, rcpt application.CoreReceipt) {
	res := req
	res.MTI, res.ResponseCode, res.AuthCode = "0210", rcpt.ResponseCode, rcpt.AuthCode
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("iso8583.de39", rcpt.ResponseCode))
	c.log.InfoContext(ctx, "core exchange", "request", req, "response", res, "latency_ms", rcpt.Latency.Milliseconds())
}

func contains(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
