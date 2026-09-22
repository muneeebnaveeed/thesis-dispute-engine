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
	Reason         domain.Reason
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
	AccountOpenedAt time.Time
	Merchant        string
	MCC             string
	OccurredAt      time.Time
}

// AccountRecord is the customer as the notices need them.
type AccountRecord struct {
	ID            uuid.UUID
	Holder        string
	Currency      string
	Email         string
	PostalAddress string
	OpenedAt      time.Time
}

// Attachment is a file an analyst attached to an email; Content is loaded only when the bytes are needed.
type Attachment struct {
	ID          uuid.UUID
	DisputeID   uuid.UUID
	NoticeID    *int64 // nil while a draft
	Filename    string
	ContentType string
	Size        int
	Content     []byte
	UploadedBy  string
	UploadedAt  time.Time
}

// RiskRecord is one stored assessment of a dispute.
type RiskRecord struct {
	Seq        int
	Assessment domain.Assessment
	AssessedAt time.Time
}

// NoticeRecord is one communication owed or sent; Document is the composed notice as JSON (notice.Document).
type NoticeRecord struct {
	ID        int64
	TenantID  uuid.UUID
	DisputeID uuid.UUID
	Seq       int
	Kind      domain.NoticeKind
	Channel   domain.Channel
	Recipient string
	Subject   string
	Document  []byte
	CreatedAt time.Time
	SentAt    *time.Time
	Attempts  int
	LastError *string
	Actor     string // the analyst who composed it; empty for the engine's own notices
	ResendOf  *int64 // the notice this one repeats, for a resend
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
	AppendLedger(ctx context.Context, disputeID uuid.UUID, entries []LedgerEntry) error
	ListLedger(ctx context.Context, disputeID uuid.UUID) ([]LedgerEntry, error)
	// TenantCore is the current tenant's banking-core configuration; a zero value means book entries only.
	TenantCore(ctx context.Context) (CoreConfig, error)
	// SendQuestionnaire records the questions asked; asking again replaces them and clears any answers.
	SendQuestionnaire(ctx context.Context, disputeID uuid.UUID, q Questionnaire) error
	// AnswerQuestionnaire records the answers to an unanswered questionnaire.
	AnswerQuestionnaire(ctx context.Context, disputeID uuid.UUID, answers map[string]string, at time.Time) error
	// GetQuestionnaire returns ErrNotFound when none was sent.
	GetQuestionnaire(ctx context.Context, disputeID uuid.UUID) (Questionnaire, error)
	GetAccount(ctx context.Context, id uuid.UUID) (AccountRecord, error)
	TenantName(ctx context.Context) (string, error)
	// InsertNotice stores one composed notice; letters are complete at once (SentAt set), emails wait for the dispatcher.
	InsertNotice(ctx context.Context, n NoticeRecord) (int64, error)
	ListNotices(ctx context.Context, disputeID uuid.UUID) ([]NoticeRecord, error)
	GetNotice(ctx context.Context, disputeID uuid.UUID, id int64) (NoticeRecord, error)
	// AccountHistory counts the account's other disputes since a date and the ones lost at the network, for scoring.
	AccountHistory(ctx context.Context, accountID, excludeDispute uuid.UUID, since time.Time) (disputes, lostChargebacks int, err error)
	InsertAttachment(ctx context.Context, disputeID uuid.UUID, a Attachment) error
	// ClaimAttachments binds this dispute's drafts to a notice; it reports how many it found.
	ClaimAttachments(ctx context.Context, disputeID uuid.UUID, noticeID int64, ids []uuid.UUID) (int, error)
	// ListAttachmentMeta returns, without content, the attachments of the given notices and this dispute's drafts.
	ListAttachmentMeta(ctx context.Context, disputeID uuid.UUID, noticeIDs []int64) ([]Attachment, error)
	GetAttachment(ctx context.Context, disputeID, id uuid.UUID) (Attachment, error)
	InsertRisk(ctx context.Context, disputeID uuid.UUID, r RiskRecord) error
	ListRisk(ctx context.Context, disputeID uuid.UUID) ([]RiskRecord, error)
	// LatestRisk returns the newest assessment per dispute that has one.
	LatestRisk(ctx context.Context, disputeIDs []uuid.UUID) (map[uuid.UUID]domain.Assessment, error)
}

// Questionnaire is what was asked of the customer and, once received, what they answered.
type Questionnaire struct {
	Reason     domain.Reason
	Questions  []domain.Question
	Answers    map[string]string // nil until received
	SentAt     time.Time
	ReceivedAt *time.Time
}

// LedgerEntry is one persisted posting: the domain movement plus the event that caused it and the reference the
// banking core is given, unique per posting so a repeated instruction cannot post twice.
type LedgerEntry struct {
	Seq       int
	Posting   domain.Posting
	Reference string
	PostedAt  time.Time
	Core      *CoreReceipt // what the core answered, for postings that moved the customer's money
}

// SuspenseBalance is what one tenant has advanced and not yet cleared under one regime.
type SuspenseBalance struct {
	TenantID uuid.UUID
	Regime   domain.Regime
	Currency string
	Balance  decimal.Decimal
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
	// SuspenseBalances feeds the suspense gauge; it runs outside any transaction.
	SuspenseBalances(ctx context.Context) ([]SuspenseBalance, error)
	// ClaimNotices takes up to batch unsent emails across tenants for one delivery attempt each; a claimed notice
	// is not offered again until its backoff passes, so a crash mid-send retries rather than repeats at once.
	ClaimNotices(ctx context.Context, batch int) ([]NoticeRecord, error)
	// FinishNotice records the outcome of an attempt: sent when failure is empty, otherwise the error for the next try.
	FinishNotice(ctx context.Context, id int64, failure string) error
	// NoticeAttachments loads a notice's files with content, across tenants, for the dispatcher.
	NoticeAttachments(ctx context.Context, noticeID int64) ([]Attachment, error)
	// PurgeDraftAttachments deletes uploads never attached to a notice, older than before.
	PurgeDraftAttachments(ctx context.Context, before time.Time) (int64, error)
	// PurgeIdempotencyKeys deletes stored responses older than before and reports how many went; when another
	// replica holds the sweep it returns 0 and no error.
	PurgeIdempotencyKeys(ctx context.Context, before time.Time) (int64, error)
}

// Clock is injectable time for deterministic tests.
type Clock func() time.Time
