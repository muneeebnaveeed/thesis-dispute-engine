// Package apptest provides an in-memory application.Store for fast tests; the Postgres implementation is tested separately.
package apptest

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

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
	// Calendars holds a tenant's business-day calendar; a tenant without one gets the default.
	Calendars map[uuid.UUID]domain.Calendar
}

// NewMemStore returns an empty store.
func NewMemStore() *MemStore {
	return &MemStore{
		Transactions: map[uuid.UUID]application.TransactionRecord{},
		Disputes:     map[uuid.UUID]application.DisputeRecord{},
		Events:       map[uuid.UUID][]application.EventRecord{},
		Idempotent:   map[string]application.StoredResponse{},
		Deadlines:    map[uuid.UUID][]domain.Deadline{},
		Calendars:    map[uuid.UUID]domain.Calendar{},
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
	m.Transactions[id] = application.TransactionRecord{
		ID: id, TenantID: tenantID, AccountID: uuid.New(), Rail: rail, Amount: mustDecimal(amount), Currency: currency, AccountCurrency: accountCurrency,
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
}

func (m *MemStore) snapshot() snapshotT {
	s := snapshotT{disputes: map[uuid.UUID]application.DisputeRecord{}, events: map[uuid.UUID][]application.EventRecord{},
		idempotent: map[string]application.StoredResponse{}, deadlines: map[uuid.UUID][]domain.Deadline{}}
	for k, v := range m.Deadlines {
		s.deadlines[k] = append([]domain.Deadline(nil), v...)
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
	m.Disputes, m.Events, m.Idempotent, m.Deadlines = s.disputes, s.events, s.idempotent, s.deadlines
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
