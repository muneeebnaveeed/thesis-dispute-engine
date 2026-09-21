// Package application holds the dispute use cases; storage and transport are behind the ports declared here.
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// Errors the use cases return; infrastructure maps storage failures onto them.
var (
	ErrNotFound         = errs.New(errs.NotFound, "not-found", "the requested resource does not exist")
	ErrConflict         = errs.New(errs.Conflict, "concurrent-update", "the dispute changed while this request was in flight; reload and try again")
	ErrIdempotencyReuse = errs.New(errs.Unprocessable, "idempotency-key-reuse", "this Idempotency-Key was already used with a different request")
	ErrUnavailable      = errs.New(errs.Unavailable, "unavailable", "a dependency is temporarily unavailable")
)

// DisputeRecord is the persisted state of a dispute.
type DisputeRecord struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	Regime         domain.Regime
	State          domain.State
	Appeals        int
	Version        int64
	TransactionID  uuid.UUID
	AccountID      uuid.UUID
	DisputedAmount decimal.Decimal
	Currency       string
	OpenedAt       time.Time
	UpdatedAt      time.Time
}

// Lifecycle projects the record onto the domain type.
func (r DisputeRecord) Lifecycle() domain.Dispute {
	return domain.Dispute{Regime: r.Regime, State: r.State, Appeals: r.Appeals}
}

// EventRecord is one appended lifecycle event.
type EventRecord struct {
	Seq            int
	Event          domain.Event
	FromState      domain.State
	ToState        domain.State
	Actor          string
	Payload        []byte
	IdempotencyKey *string
	TraceID        *string
	OccurredAt     time.Time
}

// TransactionRecord is what the use cases need from a transaction to open a dispute.
type TransactionRecord struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	AccountID       uuid.UUID
	Rail            domain.Rail
	Amount          decimal.Decimal
	Currency        string
	AccountCurrency string
}

// StoredResponse is a prior answer kept for idempotent replay.
type StoredResponse struct {
	RequestHash []byte
	StatusCode  int
	Body        []byte
	CreatedAt   time.Time
}

// Tx is the set of storage operations available inside one transaction.
type Tx interface {
	GetTransaction(ctx context.Context, id uuid.UUID) (TransactionRecord, error)
	InsertDispute(ctx context.Context, d DisputeRecord) error
	GetDispute(ctx context.Context, id uuid.UUID) (DisputeRecord, error)
	// UpdateDisputeState applies a compare-and-set on version; ErrConflict when it no longer matches.
	UpdateDisputeState(ctx context.Context, id uuid.UUID, expectedVersion int64, state domain.State, appeals int, at time.Time) error
	AppendEvent(ctx context.Context, disputeID uuid.UUID, e EventRecord) error
	ListEvents(ctx context.Context, disputeID uuid.UUID) ([]EventRecord, error)
	GetIdempotent(ctx context.Context, scope, key string) (StoredResponse, error)
	PutIdempotent(ctx context.Context, scope, key string, r StoredResponse) error
	// ListDisputes returns up to limit records newest first, after the cursor when one is given.
	ListDisputes(ctx context.Context, q ListQuery) ([]DisputeRecord, error)
	// TenantCalendar is the current tenant's business-day calendar from its settings; defaults when unset.
	TenantCalendar(ctx context.Context) (domain.Calendar, error)
	InsertDeadlines(ctx context.Context, disputeID uuid.UUID, ds []domain.Deadline) error
	ListDeadlines(ctx context.Context, disputeID uuid.UUID) ([]domain.Deadline, error)
	// SettleDeadline closes an open clock as met or void at the given time; a clock already settled is left alone.
	SettleDeadline(ctx context.Context, disputeID uuid.UUID, kind domain.DeadlineKind, cycle int, met bool, at time.Time) error
	// NextDeadlines returns, per dispute that has one, the open clock that runs out first.
	NextDeadlines(ctx context.Context, disputeIDs []uuid.UUID) (map[uuid.UUID]domain.Deadline, error)
}

// ListQuery is a page request; After is exclusive and comes from the previous page's last record. Overdue keeps
// only disputes with an open clock past due at Now.
type ListQuery struct {
	State   *domain.State
	After   *Cursor
	Limit   int
	Overdue bool
	Now     time.Time
}

// Cursor is the keyset position (opened_at, id) of the last record on a page.
type Cursor struct {
	OpenedAt time.Time
	ID       uuid.UUID
}

// StateCount is how many disputes sit in one state under one regime.
type StateCount struct {
	TenantID uuid.UUID
	Regime   domain.Regime
	State    domain.State
	N        int64
}

// OverdueCount is how many open clocks of one kind are past due under one regime.
type OverdueCount struct {
	TenantID uuid.UUID
	Regime   domain.Regime
	Kind     domain.DeadlineKind
	N        int64
}

// Store runs fn inside one transaction; a returned error rolls it back.
type Store interface {
	WithTx(ctx context.Context, fn func(Tx) error) error
	// CountByState feeds the disputes-by-state gauge; it runs outside any transaction.
	CountByState(ctx context.Context) ([]StateCount, error)
	// CountOverdue feeds the overdue-deadlines gauge; it runs outside any transaction.
	CountOverdue(ctx context.Context) ([]OverdueCount, error)
	// PurgeIdempotencyKeys deletes stored responses older than before and reports how many went; when another
	// replica holds the sweep it returns 0 and no error.
	PurgeIdempotencyKeys(ctx context.Context, before time.Time) (int64, error)
}

// Clock is injectable time for deterministic tests.
type Clock func() time.Time
