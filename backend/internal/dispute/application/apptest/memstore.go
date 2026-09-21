// Package apptest provides an in-memory application.Store for fast tests; the Postgres implementation is tested separately.
package apptest

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

// MemStore keeps everything in maps; WithTx snapshots and restores on error to mimic rollback.
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

// AddTransaction seeds a transaction and returns its ID.
func (m *MemStore) AddTransaction(rail domain.Rail, currency, accountCurrency, amount string) uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	m.Transactions[id] = application.TransactionRecord{
		ID: id, AccountID: uuid.New(), Rail: rail, Amount: mustDecimal(amount), Currency: currency, AccountCurrency: accountCurrency,
	}
	return id
}

// WithTx serialises callers and rolls back on error.
func (m *MemStore) WithTx(_ context.Context, fn func(application.Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	snap := m.snapshot()
	if err := fn(&memTx{s: m}); err != nil {
		m.restore(snap)
		return err
	}
	return nil
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

type memTx struct{ s *MemStore }

func (t *memTx) GetTransaction(_ context.Context, id uuid.UUID) (application.TransactionRecord, error) {
	r, ok := t.s.Transactions[id]
	if !ok {
		return application.TransactionRecord{}, application.ErrNotFound
	}
	return r, nil
}

func (t *memTx) InsertDispute(_ context.Context, d application.DisputeRecord) error {
	t.s.Disputes[d.ID] = d
	return nil
}

func (t *memTx) GetDispute(_ context.Context, id uuid.UUID) (application.DisputeRecord, error) {
	r, ok := t.s.Disputes[id]
	if !ok {
		return application.DisputeRecord{}, application.ErrNotFound
	}
	return r, nil
}

func (t *memTx) UpdateDisputeState(_ context.Context, id uuid.UUID, expected int64, state domain.State, appeals int, at time.Time) error {
	r, ok := t.s.Disputes[id]
	if !ok || r.Version != expected {
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
	r, ok := t.s.Idempotent[scope+"\x00"+key]
	if !ok {
		return application.StoredResponse{}, application.ErrNotFound
	}
	return r, nil
}

func (t *memTx) PutIdempotent(_ context.Context, scope, key string, r application.StoredResponse) error {
	t.s.Idempotent[scope+"\x00"+key] = r
	return nil
}
