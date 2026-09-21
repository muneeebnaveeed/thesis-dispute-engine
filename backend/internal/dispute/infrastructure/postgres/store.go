// Package postgres implements the dispute application ports on PostgreSQL via sqlc-generated queries.
package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// Store runs application transactions on a pgx pool.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// WithTx implements application.Store.
func (s *Store) WithTx(ctx context.Context, fn func(application.Tx) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(&txn{q: sqlcgen.New(tx)})
	})
}

type txn struct {
	q *sqlcgen.Queries
}

func (t *txn) GetTransaction(ctx context.Context, id uuid.UUID) (application.TransactionRecord, error) {
	row, err := t.q.GetTransaction(ctx, id)
	if err != nil {
		return application.TransactionRecord{}, mapErr(err)
	}
	return application.TransactionRecord{
		ID: row.ID, AccountID: row.AccountID, Rail: domain.Rail(row.Rail),
		Amount: row.Amount, Currency: row.Currency, AccountCurrency: row.AccountCurrency,
	}, nil
}

func (t *txn) InsertDispute(ctx context.Context, d application.DisputeRecord) error {
	return mapErr(t.q.InsertDispute(ctx, sqlcgen.InsertDisputeParams{
		ID: d.ID, Regime: string(d.Regime), State: string(d.State), Appeals: int32Of(d.Appeals), Version: d.Version,
		TransactionID: d.TransactionID, AccountID: d.AccountID, DisputedAmount: d.DisputedAmount,
		Currency: d.Currency, OpenedAt: d.OpenedAt,
	}))
}

func (t *txn) GetDispute(ctx context.Context, id uuid.UUID) (application.DisputeRecord, error) {
	row, err := t.q.GetDispute(ctx, id)
	if err != nil {
		return application.DisputeRecord{}, mapErr(err)
	}
	return application.DisputeRecord{
		ID: row.ID, Regime: domain.Regime(row.Regime), State: domain.State(row.State), Appeals: int(row.Appeals),
		Version: row.Version, TransactionID: row.TransactionID, AccountID: row.AccountID,
		DisputedAmount: row.DisputedAmount, Currency: row.Currency, OpenedAt: row.OpenedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (t *txn) UpdateDisputeState(ctx context.Context, id uuid.UUID, expectedVersion int64, state domain.State, appeals int, at time.Time) error {
	n, err := t.q.UpdateDisputeState(ctx, sqlcgen.UpdateDisputeStateParams{
		ID: id, State: string(state), Appeals: int32Of(appeals), UpdatedAt: at, Version: expectedVersion,
	})
	if err != nil {
		return mapErr(err)
	}
	if n == 0 {
		return application.ErrConflict
	}
	return nil
}

func (t *txn) AppendEvent(ctx context.Context, disputeID uuid.UUID, e application.EventRecord) error {
	_, err := t.q.InsertDisputeEvent(ctx, sqlcgen.InsertDisputeEventParams{
		DisputeID: disputeID, Seq: int32Of(e.Seq), Event: string(e.Event), FromState: string(e.FromState),
		ToState: string(e.ToState), Actor: e.Actor, Payload: e.Payload, IdempotencyKey: e.IdempotencyKey,
		TraceID: e.TraceID, OccurredAt: e.OccurredAt,
	})
	return mapErr(err)
}

func (t *txn) ListEvents(ctx context.Context, disputeID uuid.UUID) ([]application.EventRecord, error) {
	rows, err := t.q.ListDisputeEvents(ctx, disputeID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.EventRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.EventRecord{
			Seq: int(r.Seq), Event: domain.Event(r.Event), FromState: domain.State(r.FromState), ToState: domain.State(r.ToState),
			Actor: r.Actor, Payload: r.Payload, IdempotencyKey: r.IdempotencyKey, TraceID: r.TraceID, OccurredAt: r.OccurredAt,
		})
	}
	return out, nil
}

func (t *txn) GetIdempotent(ctx context.Context, scope, key string) (application.StoredResponse, error) {
	row, err := t.q.GetIdempotencyKey(ctx, sqlcgen.GetIdempotencyKeyParams{Scope: scope, Key: key})
	if err != nil {
		return application.StoredResponse{}, mapErr(err)
	}
	return application.StoredResponse{RequestHash: row.RequestHash, StatusCode: int(row.StatusCode), Body: row.Response}, nil
}

func (t *txn) PutIdempotent(ctx context.Context, scope, key string, r application.StoredResponse) error {
	return mapErr(t.q.InsertIdempotencyKey(ctx, sqlcgen.InsertIdempotencyKeyParams{
		Scope: scope, Key: key, RequestHash: r.RequestHash, StatusCode: int32Of(r.StatusCode), Response: r.Body,
	}))
}

// int32Of narrows the small counters the schema stores as int (appeals, seq); the bound check is what gosec wants to see.
func int32Of(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}

// mapErr turns driver errors into the application's sentinels; a unique violation on (dispute_id, seq) is a lost race.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errs.Wrap(application.ErrConflict, "unique violation on %s", pgErr.ConstraintName)
	}
	return err
}
