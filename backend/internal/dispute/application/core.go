package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// CoreConfig is the tenant's banking-core connection as stored in tenants.settings.core: which adapter and its
// settings, opaque to the use cases. An empty Kind means the tenant has no core and postings are book entries only.
type CoreConfig struct {
	Kind     string          `json:"kind"`
	Settings json.RawMessage `json:"-"`
}

// CoreInstruction is one movement on the customer's account the core must carry out (docs/adr/0015). Reference
// is unique per posting and stable across retries, so the core can recognise a repeat.
type CoreInstruction struct {
	Reference     string
	DisputeID     uuid.UUID
	AccountID     uuid.UUID
	TransactionID uuid.UUID
	Kind          domain.PostingKind
	CreditAccount bool // true credits the customer; false debits them (a reversal)
	Amount        decimal.Decimal
	Currency      string
	RequestedAt   time.Time
}

// CoreReceipt is the core's answer, in ISO 8583 terms: a retrieval reference number, a two-character response
// code, and whether the instruction was carried out (now, or earlier under the same reference).
type CoreReceipt struct {
	RRN          string
	ResponseCode string
	ResponseText string
	Approved     bool
	Duplicate    bool
	AuthCode     string
	ProcessedAt  time.Time
	Latency      time.Duration
}

// BankingCore posts to a tenant's core banking system. Implementations return an error only when no answer was
// obtained (the caller treats that as unavailable); a decline is a receipt with Approved false.
type BankingCore interface {
	Post(ctx context.Context, cfg CoreConfig, ins CoreInstruction) (CoreReceipt, error)
}

// ErrCoreDeclined means the tenant's core refused to move the money; the transition is rolled back.
var ErrCoreDeclined = errs.New(errs.Unprocessable, "core-declined", "the banking core declined the posting")

// CoreRouter picks the adapter by the tenant's configured kind; an empty kind books the posting without a core.
type CoreRouter struct {
	Adapters map[string]BankingCore
}

// Post implements BankingCore.
func (r CoreRouter) Post(ctx context.Context, cfg CoreConfig, ins CoreInstruction) (CoreReceipt, error) {
	if cfg.Kind == "" {
		return bookOnly{}.Post(ctx, cfg, ins)
	}
	adapter, ok := r.Adapters[cfg.Kind]
	if !ok {
		return CoreReceipt{}, errs.Wrap(ErrUnavailable, "no banking core adapter of kind %q", cfg.Kind)
	}
	return adapter.Post(ctx, cfg, ins)
}

// bookOnly approves everything: the ledger is the only record, as for a tenant that has not connected a core.
type bookOnly struct{}

func (bookOnly) Post(_ context.Context, _ CoreConfig, ins CoreInstruction) (CoreReceipt, error) {
	return CoreReceipt{RRN: "", ResponseCode: "00", ResponseText: "booked without a core", Approved: true, ProcessedAt: ins.RequestedAt}, nil
}
