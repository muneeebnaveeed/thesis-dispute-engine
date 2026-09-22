// Package postgres implements the dispute application ports on PostgreSQL via sqlc-generated queries.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/telemetry"
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

// ClaimNotices implements application.Store through the owner-defined claim_notices function.
func (s *Store) ClaimNotices(ctx context.Context, batch int) ([]application.NoticeRecord, error) {
	rows, err := sqlcgen.New(s.pool).ClaimNotices(ctx, int32Of(batch))
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.NoticeRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.NoticeRecord{ID: r.ID, TenantID: r.TenantID, DisputeID: r.DisputeID, Seq: int(r.Seq), Kind: domain.NoticeKind(r.Kind),
			Channel: domain.Channel(r.Channel), Recipient: r.Recipient, Subject: r.Subject, Document: r.Document, CreatedAt: r.CreatedAt, Attempts: int(r.Attempts),
			TraceContext: derefString(r.TraceContext)})
	}
	return out, nil
}

// FinishNotice implements application.Store.
func (s *Store) FinishNotice(ctx context.Context, id int64, failure string) error {
	var f *string
	if failure != "" {
		f = &failure
	}
	return mapErr(sqlcgen.New(s.pool).FinishNotice(ctx, sqlcgen.FinishNoticeParams{NoticeID: id, Failure: f}))
}

// OutboxBacklog implements application.Store through the owner-defined outbox_backlog function.
func (s *Store) OutboxBacklog(ctx context.Context) (int64, error) {
	n, err := sqlcgen.New(s.pool).OutboxBacklog(ctx)
	return n, mapErr(err)
}

// NoticeAttachments implements application.Store through the owner-defined notice_attachments function.
func (s *Store) NoticeAttachments(ctx context.Context, noticeID int64) ([]application.Attachment, error) {
	rows, err := sqlcgen.New(s.pool).NoticeAttachments(ctx, noticeID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.Attachment, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.Attachment{ID: r.ID, Filename: r.Filename, ContentType: r.ContentType, Size: int(r.Size), Content: r.Content})
	}
	return out, nil
}

// PurgeDraftAttachments implements application.Store.
func (s *Store) PurgeDraftAttachments(ctx context.Context, before time.Time) (int64, error) {
	n, err := sqlcgen.New(s.pool).PurgeDraftAttachments(ctx, before)
	return n, mapErr(err)
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
		Amount: row.Amount, Currency: row.Currency, AccountCurrency: row.AccountCurrency, AccountOpenedAt: row.AccountOpenedAt,
		Merchant: row.Merchant, OccurredAt: row.OccurredAt, MCC: derefString(row.Mcc),
	}, nil
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (t *txn) InsertDispute(ctx context.Context, d application.DisputeRecord) error {
	return mapErr(t.q.InsertDispute(ctx, sqlcgen.InsertDisputeParams{
		ID: d.ID, Regime: string(d.Regime), State: string(d.State), Appeals: int32Of(d.Appeals), Version: d.Version,
		TransactionID: d.TransactionID, AccountID: d.AccountID, DisputedAmount: d.DisputedAmount,
		Currency: d.Currency, OpenedAt: d.OpenedAt, Reason: string(d.Reason),
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
		Reason: domain.Reason(row.Reason),
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
			TransactionID: r.TransactionID, DisputedAmount: r.DisputedAmount, Currency: r.Currency, OpenedAt: r.OpenedAt, UpdatedAt: r.UpdatedAt,
			Reason: domain.Reason(r.Reason)})
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
		params := sqlcgen.InsertLedgerEntryParams{
			DisputeID: disputeID, Seq: int32Of(e.Seq), Kind: string(e.Posting.Kind),
			DebitAccount: string(e.Posting.Debit), CreditAccount: string(e.Posting.Credit),
			Amount: e.Posting.Amount, Currency: e.Posting.Currency, Reference: e.Reference, PostedAt: e.PostedAt,
		}
		if e.Core != nil {
			rrn, code, ms := e.Core.RRN, e.Core.ResponseCode, int32Of(int(e.Core.Latency.Milliseconds()))
			params.CoreRrn, params.CoreResponseCode, params.CoreLatencyMs = &rrn, &code, &ms
		}
		if err := t.q.InsertLedgerEntry(ctx, params); err != nil {
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
		e := application.LedgerEntry{
			Seq: int(r.Seq), Reference: r.Reference, PostedAt: r.PostedAt,
			Posting: domain.Posting{Kind: domain.PostingKind(r.Kind), Debit: domain.Account(r.DebitAccount),
				Credit: domain.Account(r.CreditAccount), Amount: r.Amount, Currency: r.Currency},
		}
		if r.CoreResponseCode != nil {
			e.Core = &application.CoreReceipt{ResponseCode: *r.CoreResponseCode, Approved: true, ProcessedAt: r.PostedAt}
			if r.CoreRrn != nil {
				e.Core.RRN = *r.CoreRrn
			}
			if r.CoreLatencyMs != nil {
				e.Core.Latency = time.Duration(*r.CoreLatencyMs) * time.Millisecond
			}
		}
		out = append(out, e)
	}
	return out, nil
}

func (t *txn) TenantCore(ctx context.Context) (application.CoreConfig, error) {
	row, err := t.q.GetTenantCore(ctx)
	if err != nil {
		return application.CoreConfig{}, mapErr(err)
	}
	return application.CoreConfig{Kind: row.Kind, Settings: row.Settings}, nil
}

func (t *txn) SendQuestionnaire(ctx context.Context, disputeID uuid.UUID, q application.Questionnaire) error {
	questions, err := json.Marshal(q.Questions)
	if err != nil {
		return err
	}
	return mapErr(t.q.UpsertQuestionnaire(ctx, sqlcgen.UpsertQuestionnaireParams{DisputeID: disputeID, Reason: string(q.Reason), Questions: questions, SentAt: q.SentAt}))
}

func (t *txn) AnswerQuestionnaire(ctx context.Context, disputeID uuid.UUID, answers map[string]string, at time.Time) error {
	raw, err := json.Marshal(answers)
	if err != nil {
		return err
	}
	n, err := t.q.AnswerQuestionnaire(ctx, sqlcgen.AnswerQuestionnaireParams{DisputeID: disputeID, Answers: raw, ReceivedAt: pgtype.Timestamptz{Time: at, Valid: true}})
	if err != nil {
		return mapErr(err)
	}
	if n == 0 {
		return errs.Wrap(application.ErrNotFound, "no unanswered questionnaire on dispute %s", disputeID)
	}
	return nil
}

func (t *txn) GetQuestionnaire(ctx context.Context, disputeID uuid.UUID) (application.Questionnaire, error) {
	row, err := t.q.GetQuestionnaire(ctx, disputeID)
	if err != nil {
		return application.Questionnaire{}, mapErr(err)
	}
	q := application.Questionnaire{Reason: domain.Reason(row.Reason), SentAt: row.SentAt}
	if err := json.Unmarshal(row.Questions, &q.Questions); err != nil {
		return application.Questionnaire{}, fmt.Errorf("postgres: questionnaire questions: %w", err)
	}
	if row.ReceivedAt.Valid {
		at := row.ReceivedAt.Time
		q.ReceivedAt = &at
		if err := json.Unmarshal(row.Answers, &q.Answers); err != nil {
			return application.Questionnaire{}, fmt.Errorf("postgres: questionnaire answers: %w", err)
		}
	}
	return q, nil
}

func (t *txn) GetAccount(ctx context.Context, id uuid.UUID) (application.AccountRecord, error) {
	row, err := t.q.GetAccount(ctx, id)
	if err != nil {
		return application.AccountRecord{}, mapErr(err)
	}
	a := application.AccountRecord{ID: row.ID, Holder: row.HolderName, Currency: row.Currency, OpenedAt: row.OpenedAt}
	if row.Email != nil {
		a.Email = *row.Email
	}
	if row.PostalAddress != nil {
		a.PostalAddress = *row.PostalAddress
	}
	return a, nil
}

func (t *txn) TenantName(ctx context.Context) (string, error) {
	name, err := t.q.GetTenantName(ctx)
	return name, mapErr(err)
}

func (t *txn) InsertNotice(ctx context.Context, n application.NoticeRecord) (int64, error) {
	var sentAt pgtype.Timestamptz
	if n.SentAt != nil {
		sentAt = pgtype.Timestamptz{Time: *n.SentAt, Valid: true}
	}
	var actor *string
	if n.Actor != "" {
		actor = &n.Actor
	}
	// the request's trace context rides with the row so the dispatcher's span can link back to it
	var traceContext *string
	if tp := telemetry.Traceparent(ctx); tp != "" {
		traceContext = &tp
	}
	id, err := t.q.InsertNotice(ctx, sqlcgen.InsertNoticeParams{DisputeID: n.DisputeID, Seq: int32Of(n.Seq), Kind: string(n.Kind), Channel: string(n.Channel),
		Recipient: n.Recipient, Subject: n.Subject, Document: n.Document, CreatedAt: n.CreatedAt, SentAt: sentAt, Actor: actor, ResendOf: n.ResendOf, TraceContext: traceContext})
	return id, mapErr(err)
}

func (t *txn) ListNotices(ctx context.Context, disputeID uuid.UUID) ([]application.NoticeRecord, error) {
	rows, err := t.q.ListNotices(ctx, disputeID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.NoticeRecord, 0, len(rows))
	for _, r := range rows {
		n := noticeOf(r.ID, r.DisputeID, r.Seq, r.Kind, r.Channel, r.Recipient, r.Subject, r.Document, r.CreatedAt, r.SentAt, r.Attempts, r.LastError, r.Actor)
		n.ResendOf = r.ResendOf
		out = append(out, n)
	}
	return out, nil
}

func (t *txn) GetNotice(ctx context.Context, disputeID uuid.UUID, id int64) (application.NoticeRecord, error) {
	r, err := t.q.GetNotice(ctx, sqlcgen.GetNoticeParams{ID: id, DisputeID: disputeID})
	if err != nil {
		return application.NoticeRecord{}, mapErr(err)
	}
	n := noticeOf(r.ID, r.DisputeID, r.Seq, r.Kind, r.Channel, r.Recipient, r.Subject, r.Document, r.CreatedAt, r.SentAt, r.Attempts, r.LastError, r.Actor)
	n.ResendOf = r.ResendOf
	return n, nil
}

func noticeOf(id int64, disputeID uuid.UUID, seq int32, kind, channel, recipient, subject string, document []byte, createdAt time.Time,
	sentAt pgtype.Timestamptz, attempts int32, lastError, actor *string) application.NoticeRecord {
	n := application.NoticeRecord{ID: id, DisputeID: disputeID, Seq: int(seq), Kind: domain.NoticeKind(kind), Channel: domain.Channel(channel),
		Recipient: recipient, Subject: subject, Document: document, CreatedAt: createdAt, Attempts: int(attempts), LastError: lastError, Actor: derefString(actor)}
	if sentAt.Valid {
		at := sentAt.Time
		n.SentAt = &at
	}
	return n
}

func (t *txn) AccountHistory(ctx context.Context, accountID, exclude uuid.UUID, since time.Time) (int, int, error) {
	disputes, err := t.q.CountAccountDisputesSince(ctx, sqlcgen.CountAccountDisputesSinceParams{AccountID: accountID, OpenedAt: since, ID: exclude})
	if err != nil {
		return 0, 0, mapErr(err)
	}
	lost, err := t.q.CountAccountLostChargebacks(ctx, sqlcgen.CountAccountLostChargebacksParams{AccountID: accountID, ID: exclude})
	if err != nil {
		return 0, 0, mapErr(err)
	}
	return int(disputes), int(lost), nil
}

func (t *txn) InsertRisk(ctx context.Context, disputeID uuid.UUID, r application.RiskRecord) error {
	signals, err := json.Marshal(r.Assessment.Signals)
	if err != nil {
		return err
	}
	return mapErr(t.q.InsertRiskAssessment(ctx, sqlcgen.InsertRiskAssessmentParams{DisputeID: disputeID, Seq: int32Of(r.Seq), Score: int32Of(r.Assessment.Score),
		Tier: string(r.Assessment.Tier), Signals: signals, AssessedAt: r.AssessedAt}))
}

func (t *txn) ListRisk(ctx context.Context, disputeID uuid.UUID) ([]application.RiskRecord, error) {
	rows, err := t.q.ListRiskAssessments(ctx, disputeID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.RiskRecord, 0, len(rows))
	for _, r := range rows {
		rec := application.RiskRecord{Seq: int(r.Seq), AssessedAt: r.AssessedAt, Assessment: domain.Assessment{Score: int(r.Score), Tier: domain.RiskTier(r.Tier)}}
		if err := json.Unmarshal(r.Signals, &rec.Assessment.Signals); err != nil {
			return nil, fmt.Errorf("postgres: risk signals: %w", err)
		}
		out = append(out, rec)
	}
	return out, nil
}

func (t *txn) LatestRisk(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Assessment, error) {
	rows, err := t.q.LatestRiskTiers(ctx, ids)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make(map[uuid.UUID]domain.Assessment, len(rows))
	for _, r := range rows {
		out[r.DisputeID] = domain.Assessment{Score: int(r.Score), Tier: domain.RiskTier(r.Tier)}
	}
	return out, nil
}

func (t *txn) InsertAttachment(ctx context.Context, disputeID uuid.UUID, a application.Attachment) error {
	return mapErr(t.q.InsertAttachment(ctx, sqlcgen.InsertAttachmentParams{ID: a.ID, DisputeID: disputeID, Filename: a.Filename, ContentType: a.ContentType,
		Size: int32Of(a.Size), Content: a.Content, UploadedBy: a.UploadedBy, UploadedAt: a.UploadedAt}))
}

func (t *txn) ClaimAttachments(ctx context.Context, disputeID uuid.UUID, noticeID int64, ids []uuid.UUID) (int, error) {
	n, err := t.q.ClaimAttachments(ctx, sqlcgen.ClaimAttachmentsParams{NoticeID: &noticeID, DisputeID: disputeID, Ids: ids})
	return int(n), mapErr(err)
}

func (t *txn) ListAttachmentMeta(ctx context.Context, disputeID uuid.UUID, noticeIDs []int64) ([]application.Attachment, error) {
	rows, err := t.q.ListAttachmentMeta(ctx, sqlcgen.ListAttachmentMetaParams{DisputeID: disputeID, NoticeIds: noticeIDs})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.Attachment, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.Attachment{ID: r.ID, DisputeID: disputeID, NoticeID: r.NoticeID, Filename: r.Filename, ContentType: r.ContentType, Size: int(r.Size), UploadedBy: r.UploadedBy, UploadedAt: r.UploadedAt})
	}
	return out, nil
}

func (t *txn) GetAttachment(ctx context.Context, disputeID, id uuid.UUID) (application.Attachment, error) {
	r, err := t.q.GetAttachment(ctx, sqlcgen.GetAttachmentParams{ID: id, DisputeID: disputeID})
	if err != nil {
		return application.Attachment{}, mapErr(err)
	}
	return application.Attachment{ID: r.ID, DisputeID: disputeID, NoticeID: r.NoticeID, Filename: r.Filename, ContentType: r.ContentType, Size: int(r.Size), Content: r.Content, UploadedBy: r.UploadedBy, UploadedAt: r.UploadedAt}, nil
}

func (t *txn) TenantTemplates(ctx context.Context) ([]application.TenantTemplate, error) {
	rows, err := t.q.ListTenantTemplates(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]application.TenantTemplate, 0, len(rows))
	for _, r := range rows {
		out = append(out, application.TenantTemplate{Kind: domain.NoticeKind(r.Kind), Override: r.Override, UpdatedBy: r.UpdatedBy, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

func (t *txn) PutTenantTemplate(ctx context.Context, tt application.TenantTemplate) error {
	return mapErr(t.q.UpsertTenantTemplate(ctx, sqlcgen.UpsertTenantTemplateParams{Kind: string(tt.Kind), Override: tt.Override, UpdatedBy: tt.UpdatedBy, UpdatedAt: tt.UpdatedAt}))
}

func (t *txn) DeleteTenantTemplate(ctx context.Context, kind domain.NoticeKind) (bool, error) {
	n, err := t.q.DeleteTenantTemplate(ctx, string(kind))
	return n > 0, mapErr(err)
}
