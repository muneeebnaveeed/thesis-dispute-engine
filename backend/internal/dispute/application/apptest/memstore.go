// Package apptest provides an in-memory application.Store for fast tests; the Postgres implementation is tested separately.
package apptest

import (
	"context"
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
}

// NewMemStore returns an empty store.
func NewMemStore() *MemStore {
	return &MemStore{
		Transactions: map[uuid.UUID]application.TransactionRecord{},
		Disputes:     map[uuid.UUID]application.DisputeRecord{},
		Events:       map[uuid.UUID][]application.EventRecord{},
		Idempotent:   map[string]application.StoredResponse{},
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
}

func (m *MemStore) snapshot() snapshotT {
	s := snapshotT{disputes: map[uuid.UUID]application.DisputeRecord{}, events: map[uuid.UUID][]application.EventRecord{}, idempotent: map[string]application.StoredResponse{}}
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
	m.Disputes, m.Events, m.Idempotent = s.disputes, s.events, s.idempotent
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
