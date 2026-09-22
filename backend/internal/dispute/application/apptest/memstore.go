// Package apptest provides an in-memory application.Store for fast tests; the Postgres implementation is tested separately.
package apptest

import (
	"context"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// TenantA and TenantB are the tenants unit tests use; Ctx carries TenantA.
var (
	TenantA = uuid.MustParse("00000000-0000-8000-8000-00000000a001")
	TenantB = uuid.MustParse("00000000-0000-8000-8000-00000000a002")
)

// Ctx returns a context for TenantA.
func Ctx() context.Context { return tenant.WithID(context.Background(), TenantA) }

// MemStore keeps everything in maps; WithTx snapshots and restores on error to mimic rollback, and every
// read is scoped to the tenant in the context the way row-level security scopes the real store.
type MemStore struct {
	mu           sync.Mutex
	Transactions map[uuid.UUID]application.TransactionRecord
	Disputes     map[uuid.UUID]application.DisputeRecord
	Events       map[uuid.UUID][]application.EventRecord
	Idempotent   map[string]application.StoredResponse
	Deadlines    map[uuid.UUID][]domain.Deadline
	Ledger       map[uuid.UUID][]application.LedgerEntry
	Questions    map[uuid.UUID]application.Questionnaire
	Notices      []application.NoticeRecord
	Risk         map[uuid.UUID][]application.RiskRecord
	Attachments  map[uuid.UUID]application.Attachment
	Templates    map[uuid.UUID]map[domain.NoticeKind]application.TenantTemplate
	Accounts     map[uuid.UUID]application.AccountRecord
	Names        map[uuid.UUID]string
	nextNotice   int64
	// Calendars holds a tenant's business-day calendar; a tenant without one gets the default.
	Calendars map[uuid.UUID]domain.Calendar
	// Cores holds a tenant's banking-core configuration; a tenant without one books postings without a core.
	Cores map[uuid.UUID]application.CoreConfig
}

// NewMemStore returns an empty store.
func NewMemStore() *MemStore {
	return &MemStore{
		Transactions: map[uuid.UUID]application.TransactionRecord{},
		Disputes:     map[uuid.UUID]application.DisputeRecord{},
		Events:       map[uuid.UUID][]application.EventRecord{},
		Idempotent:   map[string]application.StoredResponse{},
		Deadlines:    map[uuid.UUID][]domain.Deadline{},
		Ledger:       map[uuid.UUID][]application.LedgerEntry{},
		Questions:    map[uuid.UUID]application.Questionnaire{},
		Accounts:     map[uuid.UUID]application.AccountRecord{},
		Risk:         map[uuid.UUID][]application.RiskRecord{},
		Attachments:  map[uuid.UUID]application.Attachment{},
		Templates:    map[uuid.UUID]map[domain.NoticeKind]application.TenantTemplate{},
		Names:        map[uuid.UUID]string{},
		Calendars:    map[uuid.UUID]domain.Calendar{},
		Cores:        map[uuid.UUID]application.CoreConfig{},
	}
}

// AddTransaction seeds a transaction for TenantA and returns its ID.
func (m *MemStore) AddTransaction(rail domain.Rail, currency, accountCurrency, amount string) uuid.UUID {
	return m.AddTransactionFor(TenantA, rail, currency, accountCurrency, amount)
}

// AddTransactionFor seeds a transaction for one tenant and returns its ID.
func (m *MemStore) AddTransactionFor(tenantID uuid.UUID, rail domain.Rail, currency, accountCurrency, amount string) uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	account := uuid.New()
	opened := time.Now().AddDate(-3, 0, 0)
	m.Accounts[account] = application.AccountRecord{ID: account, Holder: "Test Holder", Currency: accountCurrency, Email: "holder@example.com", PostalAddress: "1 Test Street", OpenedAt: opened}
	m.Transactions[id] = application.TransactionRecord{
		ID: id, TenantID: tenantID, AccountID: account, Rail: rail, Amount: mustDecimal(amount), Currency: currency, AccountCurrency: accountCurrency,
		AccountOpenedAt: opened, Merchant: "ACME", OccurredAt: time.Now().Add(-48 * time.Hour),
	}
	return id
}

// WithTx serialises callers and rolls back on error.
func (m *MemStore) WithTx(ctx context.Context, fn func(application.Tx) error) error {
	id, ok := tenant.IDFrom(ctx)
	if !ok {
		return tenant.ErrMissing
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	snap := m.snapshot()
	if err := fn(&memTx{s: m, tenant: id}); err != nil {
		m.restore(snap)
		return err
	}
	return nil
}

// PurgeIdempotencyKeys implements application.Store.
func (m *MemStore) PurgeIdempotencyKeys(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for k, r := range m.Idempotent {
		if r.CreatedAt.Before(before) {
			delete(m.Idempotent, k)
			n++
		}
	}
	return n, nil
}

// CountByState implements application.Store.
func (m *MemStore) CountByState(_ context.Context) ([]application.StateCount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	counts := map[[3]string]int64{}
	for _, d := range m.Disputes {
		counts[[3]string{d.TenantID.String(), string(d.Regime), string(d.State)}]++
	}
	out := make([]application.StateCount, 0, len(counts))
	for k, n := range counts {
		out = append(out, application.StateCount{TenantID: uuid.MustParse(k[0]), Regime: domain.Regime(k[1]), State: domain.State(k[2]), N: n})
	}
	return out, nil
}

type snapshotT struct {
	disputes   map[uuid.UUID]application.DisputeRecord
	events     map[uuid.UUID][]application.EventRecord
	idempotent map[string]application.StoredResponse
	deadlines  map[uuid.UUID][]domain.Deadline
	ledger     map[uuid.UUID][]application.LedgerEntry
	questions  map[uuid.UUID]application.Questionnaire
	notices    []application.NoticeRecord
	risk       map[uuid.UUID][]application.RiskRecord
	files      map[uuid.UUID]application.Attachment
}

func (m *MemStore) snapshot() snapshotT {
	s := snapshotT{disputes: map[uuid.UUID]application.DisputeRecord{}, events: map[uuid.UUID][]application.EventRecord{},
		idempotent: map[string]application.StoredResponse{}, deadlines: map[uuid.UUID][]domain.Deadline{},
		ledger: map[uuid.UUID][]application.LedgerEntry{}, questions: map[uuid.UUID]application.Questionnaire{}}
	for k, v := range m.Questions {
		s.questions[k] = v
	}
	s.notices = append([]application.NoticeRecord(nil), m.Notices...)
	s.risk = map[uuid.UUID][]application.RiskRecord{}
	for k, v := range m.Risk {
		s.risk[k] = append([]application.RiskRecord(nil), v...)
	}
	s.files = map[uuid.UUID]application.Attachment{}
	for k, v := range m.Attachments {
		s.files[k] = v
	}
	for k, v := range m.Deadlines {
		s.deadlines[k] = append([]domain.Deadline(nil), v...)
	}
	for k, v := range m.Ledger {
		s.ledger[k] = append([]application.LedgerEntry(nil), v...)
	}
	for k, v := range m.Disputes {
		s.disputes[k] = v
	}
	for k, v := range m.Events {
		s.events[k] = append([]application.EventRecord(nil), v...)
	}
	for k, v := range m.Idempotent {
		s.idempotent[k] = v
	}
	return s
}

func (m *MemStore) restore(s snapshotT) {
	m.Disputes, m.Events, m.Idempotent, m.Deadlines, m.Ledger, m.Questions, m.Notices, m.Risk, m.Attachments = s.disputes, s.events, s.idempotent, s.deadlines, s.ledger, s.questions, s.notices, s.risk, s.files
}

// NoticeAttachments implements application.Store.
func (m *MemStore) NoticeAttachments(_ context.Context, noticeID int64) ([]application.Attachment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []application.Attachment
	for _, a := range m.Attachments {
		if a.NoticeID != nil && *a.NoticeID == noticeID {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UploadedAt.Before(out[j].UploadedAt) })
	return out, nil
}

// PurgeDraftAttachments implements application.Store.
func (m *MemStore) PurgeDraftAttachments(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for id, a := range m.Attachments {
		if a.NoticeID == nil && a.UploadedAt.Before(before) {
			delete(m.Attachments, id)
			n++
		}
	}
	return n, nil
}

// ClaimNotices implements application.Store; backoff is not modelled, every unsent email is offered each pass.
func (m *MemStore) ClaimNotices(_ context.Context, batch int) ([]application.NoticeRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []application.NoticeRecord
	for i := range m.Notices {
		n := &m.Notices[i]
		if n.SentAt == nil && n.Channel == domain.ChannelEmail && len(out) < batch {
			n.Attempts++
			out = append(out, *n)
		}
	}
	return out, nil
}

// OutboxBacklog implements application.Store.
func (m *MemStore) OutboxBacklog(_ context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for i := range m.Notices {
		if m.Notices[i].SentAt == nil && m.Notices[i].Channel == domain.ChannelEmail {
			n++
		}
	}
	return n, nil
}

// FinishNotice implements application.Store.
func (m *MemStore) FinishNotice(_ context.Context, id int64, failure string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.Notices {
		if m.Notices[i].ID == id {
			if failure == "" {
				now := time.Now()
				m.Notices[i].SentAt, m.Notices[i].LastError = &now, nil
			} else {
				f := failure
				m.Notices[i].LastError = &f
			}
		}
	}
	return nil
}

// SuspenseBalances implements application.Store.
func (m *MemStore) SuspenseBalances(_ context.Context) ([]application.SuspenseBalance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sums := map[[3]string]decimal.Decimal{}
	for id, entries := range m.Ledger {
		d := m.Disputes[id]
		for _, e := range entries {
			k := [3]string{d.TenantID.String(), string(d.Regime), e.Posting.Currency}
			sums[k] = sums[k].Add(domain.SuspenseBalance([]domain.Posting{e.Posting}))
		}
	}
	out := make([]application.SuspenseBalance, 0, len(sums))
	for k, b := range sums {
		out = append(out, application.SuspenseBalance{TenantID: uuid.MustParse(k[0]), Regime: domain.Regime(k[1]), Currency: k[2], Balance: b})
	}
	return out, nil
}

// CountOverdue implements application.Store.
func (m *MemStore) CountOverdue(_ context.Context) ([]application.OverdueCount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	counts := map[[3]string]int64{}
	for id, ds := range m.Deadlines {
		d := m.Disputes[id]
		for _, dl := range ds {
			if dl.Status(now) == domain.DeadlineBreached {
				counts[[3]string{d.TenantID.String(), string(d.Regime), string(dl.Kind)}]++
			}
		}
	}
	out := make([]application.OverdueCount, 0, len(counts))
	for k, n := range counts {
		out = append(out, application.OverdueCount{TenantID: uuid.MustParse(k[0]), Regime: domain.Regime(k[1]), Kind: domain.DeadlineKind(k[2]), N: n})
	}
	return out, nil
}

type memTx struct {
	s      *MemStore
	tenant uuid.UUID
}

func (t *memTx) GetTransaction(_ context.Context, id uuid.UUID) (application.TransactionRecord, error) {
	r, ok := t.s.Transactions[id]
	if !ok || r.TenantID != t.tenant {
		return application.TransactionRecord{}, application.ErrNotFound
	}
	return r, nil
}

func (t *memTx) InsertDispute(_ context.Context, d application.DisputeRecord) error {
	d.TenantID = t.tenant
	t.s.Disputes[d.ID] = d
	return nil
}

func (t *memTx) GetDispute(_ context.Context, id uuid.UUID) (application.DisputeRecord, error) {
	r, ok := t.s.Disputes[id]
	if !ok || r.TenantID != t.tenant {
		return application.DisputeRecord{}, application.ErrNotFound
	}
	return r, nil
}

func (t *memTx) UpdateDisputeState(_ context.Context, id uuid.UUID, expected int64, state domain.State, appeals int, at time.Time) error {
	r, ok := t.s.Disputes[id]
	if !ok || r.TenantID != t.tenant || r.Version != expected {
		return application.ErrConflict
	}
	r.State, r.Appeals, r.Version, r.UpdatedAt = state, appeals, expected+1, at
	t.s.Disputes[id] = r
	return nil
}

func (t *memTx) AppendEvent(_ context.Context, id uuid.UUID, e application.EventRecord) error {
	for _, existing := range t.s.Events[id] {
		if existing.Seq == e.Seq {
			return application.ErrConflict
		}
	}
	t.s.Events[id] = append(t.s.Events[id], e)
	return nil
}

func (t *memTx) ListEvents(_ context.Context, id uuid.UUID) ([]application.EventRecord, error) {
	return append([]application.EventRecord(nil), t.s.Events[id]...), nil
}

func (t *memTx) GetIdempotent(_ context.Context, scope, key string) (application.StoredResponse, error) {
	r, ok := t.s.Idempotent[t.tenant.String()+"\x00"+scope+"\x00"+key]
	if !ok {
		return application.StoredResponse{}, application.ErrNotFound
	}
	return r, nil
}

func (t *memTx) PutIdempotent(_ context.Context, scope, key string, r application.StoredResponse) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	t.s.Idempotent[t.tenant.String()+"\x00"+scope+"\x00"+key] = r
	return nil
}

func (t *memTx) ListDisputes(_ context.Context, q application.ListQuery) ([]application.DisputeRecord, error) {
	var all []application.DisputeRecord
	for _, d := range t.s.Disputes {
		if d.TenantID != t.tenant || (q.State != nil && d.State != *q.State) {
			continue
		}
		if q.Overdue && !t.overdue(d.ID, q.Now) {
			continue
		}
		if q.After != nil && d.OpenedAt.After(q.After.OpenedAt) {
			continue
		}
		if q.After != nil && d.OpenedAt.Equal(q.After.OpenedAt) && d.ID.String() >= q.After.ID.String() {
			continue
		}
		all = append(all, d)
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].OpenedAt.Equal(all[j].OpenedAt) {
			return all[i].OpenedAt.After(all[j].OpenedAt)
		}
		return all[i].ID.String() > all[j].ID.String()
	})
	if len(all) > q.Limit {
		all = all[:q.Limit]
	}
	return all, nil
}

func (t *memTx) overdue(id uuid.UUID, now time.Time) bool {
	for _, d := range t.s.Deadlines[id] {
		if d.Open() && now.After(d.DueAt) {
			return true
		}
	}
	return false
}

func (t *memTx) TenantCalendar(_ context.Context) (domain.Calendar, error) {
	if c, ok := t.s.Calendars[t.tenant]; ok {
		return c, nil
	}
	return domain.DefaultCalendar(), nil
}

func (t *memTx) InsertDeadlines(_ context.Context, id uuid.UUID, ds []domain.Deadline) error {
	if _, err := t.GetDispute(context.Background(), id); err != nil {
		return err
	}
	t.s.Deadlines[id] = append(t.s.Deadlines[id], ds...)
	return nil
}

func (t *memTx) ListDeadlines(_ context.Context, id uuid.UUID) ([]domain.Deadline, error) {
	if _, err := t.GetDispute(context.Background(), id); err != nil {
		return nil, err
	}
	return append([]domain.Deadline(nil), t.s.Deadlines[id]...), nil
}

func (t *memTx) SettleDeadline(_ context.Context, id uuid.UUID, kind domain.DeadlineKind, cycle int, met bool, at time.Time) error {
	if _, err := t.GetDispute(context.Background(), id); err != nil {
		return err
	}
	ds := t.s.Deadlines[id]
	for i := range ds {
		if ds[i].Kind == kind && ds[i].Cycle == cycle && ds[i].Open() {
			stamp := at
			if met {
				ds[i].MetAt = &stamp
			} else {
				ds[i].VoidedAt = &stamp
			}
		}
	}
	return nil
}

func (t *memTx) NextDeadlines(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Deadline, error) {
	out := map[uuid.UUID]domain.Deadline{}
	for _, id := range ids {
		if d, ok := t.s.Disputes[id]; !ok || d.TenantID != t.tenant {
			continue
		}
		for _, dl := range t.s.Deadlines[id] {
			if !dl.Open() {
				continue
			}
			if cur, ok := out[id]; !ok || dl.DueAt.Before(cur.DueAt) {
				out[id] = dl
			}
		}
	}
	return out, nil
}

func (t *memTx) AppendLedger(_ context.Context, id uuid.UUID, entries []application.LedgerEntry) error {
	if _, err := t.GetDispute(context.Background(), id); err != nil {
		return err
	}
	for _, e := range entries {
		for _, have := range t.s.Ledger[id] {
			if have.Reference == e.Reference {
				return application.ErrConflict
			}
		}
		t.s.Ledger[id] = append(t.s.Ledger[id], e)
	}
	return nil
}

func (t *memTx) ListLedger(_ context.Context, id uuid.UUID) ([]application.LedgerEntry, error) {
	if _, err := t.GetDispute(context.Background(), id); err != nil {
		return nil, err
	}
	return append([]application.LedgerEntry(nil), t.s.Ledger[id]...), nil
}

func (t *memTx) TenantCore(_ context.Context) (application.CoreConfig, error) {
	return t.s.Cores[t.tenant], nil
}

func (t *memTx) SendQuestionnaire(_ context.Context, id uuid.UUID, q application.Questionnaire) error {
	if _, err := t.GetDispute(context.Background(), id); err != nil {
		return err
	}
	q.Answers, q.ReceivedAt = nil, nil
	t.s.Questions[id] = q
	return nil
}

func (t *memTx) AnswerQuestionnaire(_ context.Context, id uuid.UUID, answers map[string]string, at time.Time) error {
	q, err := t.GetQuestionnaire(context.Background(), id)
	if err != nil {
		return err
	}
	if q.ReceivedAt != nil {
		return application.ErrNotFound
	}
	q.Answers, q.ReceivedAt = answers, &at
	t.s.Questions[id] = q
	return nil
}

func (t *memTx) GetQuestionnaire(_ context.Context, id uuid.UUID) (application.Questionnaire, error) {
	if _, err := t.GetDispute(context.Background(), id); err != nil {
		return application.Questionnaire{}, err
	}
	q, ok := t.s.Questions[id]
	if !ok {
		return application.Questionnaire{}, application.ErrNotFound
	}
	return q, nil
}

func (t *memTx) GetAccount(_ context.Context, id uuid.UUID) (application.AccountRecord, error) {
	a, ok := t.s.Accounts[id]
	if !ok {
		return application.AccountRecord{}, application.ErrNotFound
	}
	return a, nil
}

func (t *memTx) TenantName(_ context.Context) (string, error) {
	if n, ok := t.s.Names[t.tenant]; ok {
		return n, nil
	}
	return "Test Bank", nil
}

func (t *memTx) InsertNotice(_ context.Context, n application.NoticeRecord) (int64, error) {
	if _, err := t.GetDispute(context.Background(), n.DisputeID); err != nil {
		return 0, err
	}
	t.s.nextNotice++
	n.ID, n.TenantID = t.s.nextNotice, t.tenant
	t.s.Notices = append(t.s.Notices, n)
	return n.ID, nil
}

func (t *memTx) ListNotices(_ context.Context, disputeID uuid.UUID) ([]application.NoticeRecord, error) {
	if _, err := t.GetDispute(context.Background(), disputeID); err != nil {
		return nil, err
	}
	var out []application.NoticeRecord
	for _, n := range t.s.Notices {
		if n.DisputeID == disputeID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (t *memTx) GetNotice(_ context.Context, disputeID uuid.UUID, id int64) (application.NoticeRecord, error) {
	list, err := t.ListNotices(context.Background(), disputeID)
	if err != nil {
		return application.NoticeRecord{}, err
	}
	for _, n := range list {
		if n.ID == id {
			return n, nil
		}
	}
	return application.NoticeRecord{}, application.ErrNotFound
}

func (t *memTx) AccountHistory(_ context.Context, accountID, exclude uuid.UUID, since time.Time) (int, int, error) {
	var disputes, lost int
	for id, d := range t.s.Disputes {
		if d.TenantID != t.tenant || d.AccountID != accountID || id == exclude {
			continue
		}
		if !d.OpenedAt.Before(since) {
			disputes++
		}
		for _, e := range t.s.Events[id] {
			if e.ToState == domain.StateChargebackLost {
				lost++
				break
			}
		}
	}
	return disputes, lost, nil
}

func (t *memTx) InsertRisk(_ context.Context, id uuid.UUID, r application.RiskRecord) error {
	if _, err := t.GetDispute(context.Background(), id); err != nil {
		return err
	}
	t.s.Risk[id] = append(t.s.Risk[id], r)
	return nil
}

func (t *memTx) ListRisk(_ context.Context, id uuid.UUID) ([]application.RiskRecord, error) {
	if _, err := t.GetDispute(context.Background(), id); err != nil {
		return nil, err
	}
	return append([]application.RiskRecord(nil), t.s.Risk[id]...), nil
}

func (t *memTx) LatestRisk(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Assessment, error) {
	out := map[uuid.UUID]domain.Assessment{}
	for _, id := range ids {
		if d, ok := t.s.Disputes[id]; !ok || d.TenantID != t.tenant {
			continue
		}
		if rs := t.s.Risk[id]; len(rs) > 0 {
			out[id] = rs[len(rs)-1].Assessment
		}
	}
	return out, nil
}

func (t *memTx) InsertAttachment(_ context.Context, disputeID uuid.UUID, a application.Attachment) error {
	if _, err := t.GetDispute(context.Background(), disputeID); err != nil {
		return err
	}
	a.DisputeID = disputeID
	t.s.Attachments[a.ID] = a
	return nil
}

func (t *memTx) ClaimAttachments(_ context.Context, disputeID uuid.UUID, noticeID int64, ids []uuid.UUID) (int, error) {
	n := 0
	for _, id := range ids {
		a, ok := t.s.Attachments[id]
		if !ok || a.NoticeID != nil || a.DisputeID != disputeID {
			continue
		}
		nid := noticeID
		a.NoticeID = &nid
		t.s.Attachments[id] = a
		n++
	}
	return n, nil
}

func (t *memTx) ListAttachmentMeta(_ context.Context, disputeID uuid.UUID, noticeIDs []int64) ([]application.Attachment, error) {
	var out []application.Attachment
	for _, a := range t.s.Attachments {
		if a.DisputeID != disputeID {
			continue
		}
		if a.NoticeID == nil || slices.Contains(noticeIDs, *a.NoticeID) {
			a.Content = nil
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UploadedAt.Before(out[j].UploadedAt) })
	return out, nil
}

func (t *memTx) GetAttachment(_ context.Context, disputeID, id uuid.UUID) (application.Attachment, error) {
	a, ok := t.s.Attachments[id]
	if !ok || a.DisputeID != disputeID {
		return application.Attachment{}, application.ErrNotFound
	}
	return a, nil
}

func (t *memTx) TenantTemplates(_ context.Context) ([]application.TenantTemplate, error) {
	out := make([]application.TenantTemplate, 0, len(t.s.Templates[t.tenant]))
	for _, tt := range t.s.Templates[t.tenant] {
		out = append(out, tt)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out, nil
}

func (t *memTx) PutTenantTemplate(_ context.Context, tt application.TenantTemplate) error {
	if t.s.Templates[t.tenant] == nil {
		t.s.Templates[t.tenant] = map[domain.NoticeKind]application.TenantTemplate{}
	}
	t.s.Templates[t.tenant][tt.Kind] = tt
	return nil
}

func (t *memTx) DeleteTenantTemplate(_ context.Context, kind domain.NoticeKind) (bool, error) {
	_, ok := t.s.Templates[t.tenant][kind]
	delete(t.s.Templates[t.tenant], kind)
	return ok, nil
}
