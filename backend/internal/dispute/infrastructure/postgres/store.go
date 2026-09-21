// Package postgres implements the dispute application ports on PostgreSQL via sqlc-generated queries.
package postgres

import (
	"context"
	"errors"
	"math"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// Store runs application transactions on a pgx pool.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// WithTx implements application.Store. The tenant is bound to the transaction with a LOCAL setting, which the
// row-level policies and the tenant_id column defaults read; a pooled connection never carries it past commit.
func (s *Store) WithTx(ctx context.Context, fn func(application.Tx) error) error {
	id, ok := tenant.IDFrom(ctx)
	if !ok {
		return tenant.ErrMissing
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, id.String()); err != nil {
			return mapErr(err)
		}
		return fn(&txn{q: sqlcgen.New(tx)})
	})
}

// CountByState implements application.Store.
func (s *Store) CountByState(ctx context.Context) ([]application.StateCount, error) {
	rows, err := sqlcgen.New(s.pool).CountDisputesByState(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.StateCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.StateCount{TenantID: r.TenantID, Regime: domain.Regime(r.Regime), State: domain.State(r.State), N: r.N})
	}
	return out, nil
}

// CountOverdue implements application.Store.
func (s *Store) CountOverdue(ctx context.Context) ([]application.OverdueCount, error) {
	rows, err := sqlcgen.New(s.pool).CountDeadlinesOverdue(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.OverdueCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.OverdueCount{TenantID: r.TenantID, Regime: domain.Regime(r.Regime), Kind: domain.DeadlineKind(r.Kind), N: r.N})
	}
	return out, nil
}

// SuspenseBalances implements application.Store.
func (s *Store) SuspenseBalances(ctx context.Context) ([]application.SuspenseBalance, error) {
	rows, err := sqlcgen.New(s.pool).SuspenseByRegime(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.SuspenseBalance, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.SuspenseBalance{TenantID: r.TenantID, Regime: domain.Regime(r.Regime), Currency: r.Currency, Balance: r.Balance})
	}
	return out, nil
}

// PurgeLockID is the advisory lock the idempotency sweep takes; arbitrary but fixed, distinct from the migration lock.
const PurgeLockID = 72040002

// PurgeIdempotencyKeys implements application.Store; it skips silently when another session holds PurgeLockID.
// The delete crosses tenants, so it goes through the owner-defined purge_idempotency_keys function, not the table.
func (s *Store) PurgeIdempotencyKeys(ctx context.Context, before time.Time) (int64, error) {
	var n int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var got bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, PurgeLockID).Scan(&got); err != nil {
			return err
		}
		if !got {
			return nil
		}
		var err error
		n, err = sqlcgen.New(tx).PurgeIdempotencyKeys(ctx, before)
		return err
	})
	return n, mapErr(err)
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
		ID: row.ID, TenantID: row.TenantID, Regime: domain.Regime(row.Regime), State: domain.State(row.State), Appeals: int(row.Appeals),
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
	return application.StoredResponse{RequestHash: row.RequestHash, StatusCode: int(row.StatusCode), Body: row.Response, CreatedAt: row.CreatedAt}, nil
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
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505":
			return errs.Wrap(application.ErrConflict, "unique violation on %s", pgErr.ConstraintName)
		case pgErr.Code == "40001" || pgErr.Code == "40P01":
			return errs.Wrap(application.ErrConflict, "postgres %s", pgErr.Code)
		// SQLSTATE classes 08 (connection), 53 (resources), 57 (operator intervention) clear up without a code change.
		case strings.HasPrefix(pgErr.Code, "08"), strings.HasPrefix(pgErr.Code, "53"), strings.HasPrefix(pgErr.Code, "57"):
			return errs.Wrap(application.ErrUnavailable, "postgres %s", pgErr.Code)
		}
		return err
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || pgconn.Timeout(err) || pgconn.SafeToRetry(err) || errors.As(err, &netErr) {
		return errs.Wrap(application.ErrUnavailable, "postgres: %v", err)
	}
	return err
}

func (t *txn) ListDisputes(ctx context.Context, q application.ListQuery) ([]application.DisputeRecord, error) {
	params := sqlcgen.ListDisputesParams{PageSize: int32Of(q.Limit), Overdue: q.Overdue, Now: q.Now}
	if q.State != nil {
		st := string(*q.State)
		params.State = &st
	}
	if q.After != nil {
		params.BeforeOpenedAt = pgtype.Timestamptz{Time: q.After.OpenedAt, Valid: true}
		params.BeforeID = pgtype.UUID{Bytes: q.After.ID, Valid: true}
	}
	rows, err := t.q.ListDisputes(ctx, params)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.DisputeRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.DisputeRecord{ID: r.ID, Regime: domain.Regime(r.Regime), State: domain.State(r.State),
			TransactionID: r.TransactionID, DisputedAmount: r.DisputedAmount, Currency: r.Currency, OpenedAt: r.OpenedAt, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

func (t *txn) TenantCalendar(ctx context.Context) (domain.Calendar, error) {
	row, err := t.q.GetTenantCalendar(ctx)
	if err != nil {
		return domain.Calendar{}, mapErr(err)
	}
	return domain.NewCalendar(row.Timezone, row.Holidays)
}

func (t *txn) InsertDeadlines(ctx context.Context, disputeID uuid.UUID, ds []domain.Deadline) error {
	for _, d := range ds {
		if err := t.q.InsertDeadline(ctx, sqlcgen.InsertDeadlineParams{DisputeID: disputeID, Kind: string(d.Kind), Cycle: int32Of(d.Cycle),
			StartedAt: d.StartedAt, DueAt: d.DueAt, Basis: d.Basis}); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func (t *txn) ListDeadlines(ctx context.Context, disputeID uuid.UUID) ([]domain.Deadline, error) {
	rows, err := t.q.ListDeadlines(ctx, disputeID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Deadline, 0, len(rows))
	for _, r := range rows {
		out = append(out, deadlineOf(r.Kind, r.Cycle, r.StartedAt, r.DueAt, r.MetAt, r.VoidedAt, r.Basis))
	}
	return out, nil
}

func (t *txn) SettleDeadline(ctx context.Context, disputeID uuid.UUID, kind domain.DeadlineKind, cycle int, met bool, at time.Time) error {
	_, err := t.q.SettleDeadline(ctx, sqlcgen.SettleDeadlineParams{Met: met, At: at, DisputeID: disputeID, Kind: string(kind), Cycle: int32Of(cycle)})
	return mapErr(err)
}

func (t *txn) NextDeadlines(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Deadline, error) {
	rows, err := t.q.NextDeadlines(ctx, ids)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make(map[uuid.UUID]domain.Deadline, len(rows))
	for _, r := range rows {
		out[r.DisputeID] = deadlineOf(r.Kind, r.Cycle, r.StartedAt, r.DueAt, r.MetAt, r.VoidedAt, r.Basis)
	}
	return out, nil
}

func deadlineOf(kind string, cycle int32, started, due time.Time, met, voided pgtype.Timestamptz, basis string) domain.Deadline {
	d := domain.Deadline{Kind: domain.DeadlineKind(kind), Cycle: int(cycle), StartedAt: started, DueAt: due, Basis: basis}
	if met.Valid {
		t := met.Time
		d.MetAt = &t
	}
	if voided.Valid {
		t := voided.Time
		d.VoidedAt = &t
	}
	return d
}

func (t *txn) AppendLedger(ctx context.Context, disputeID uuid.UUID, entries []application.LedgerEntry) error {
	for _, e := range entries {
		if err := t.q.InsertLedgerEntry(ctx, sqlcgen.InsertLedgerEntryParams{
			DisputeID: disputeID, Seq: int32Of(e.Seq), Kind: string(e.Posting.Kind),
			DebitAccount: string(e.Posting.Debit), CreditAccount: string(e.Posting.Credit),
			Amount: e.Posting.Amount, Currency: e.Posting.Currency, Reference: e.Reference, PostedAt: e.PostedAt,
		}); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func (t *txn) ListLedger(ctx context.Context, disputeID uuid.UUID) ([]application.LedgerEntry, error) {
	rows, err := t.q.ListLedgerEntries(ctx, disputeID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.LedgerEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.LedgerEntry{
			Seq: int(r.Seq), Reference: r.Reference, PostedAt: r.PostedAt,
			Posting: domain.Posting{Kind: domain.PostingKind(r.Kind), Debit: domain.Account(r.DebitAccount),
				Credit: domain.Account(r.CreditAccount), Amount: r.Amount, Currency: r.Currency},
		})
	}
	return out, nil
}
