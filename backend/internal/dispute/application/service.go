package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

const scopeName = "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"

// Service is the dispute use cases.
type Service struct {
	store  Store
	now    Clock
	tracer trace.Tracer

	transitions   metric.Int64Counter
	replays       metric.Int64Counter
	timeInState   metric.Float64Histogram
	deadlineSlack metric.Float64Histogram
}

// NewService wires a Service; a nil clock means time.Now.
func NewService(store Store, now Clock) (*Service, error) {
	if now == nil {
		now = time.Now
	}
	m := otel.Meter(scopeName)
	transitions, err := m.Int64Counter("dispute.transitions", metric.WithDescription("Lifecycle transitions applied"))
	if err != nil {
		return nil, err
	}
	replays, err := m.Int64Counter("dispute.idempotent_replays", metric.WithDescription("Requests answered from a stored idempotent response"))
	if err != nil {
		return nil, err
	}
	timeInState, err := m.Float64Histogram("dispute.time_in_state", metric.WithDescription("Seconds a dispute spent in the state it just left"), metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	deadlineSlack, err := m.Float64Histogram("dispute.deadline_slack", metric.WithDescription("Seconds between a regulatory clock being satisfied and its due time; negative means late"), metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	if _, err := m.Int64ObservableGauge("dispute.deadlines_overdue", metric.WithDescription("Open regulatory clocks past their due time"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			counts, err := store.CountOverdue(ctx)
			if err != nil {
				return err
			}
			for _, c := range counts {
				o.Observe(c.N, metric.WithAttributes(attribute.String("tenant", c.TenantID.String()),
					attribute.String("regime", string(c.Regime)), attribute.String("kind", string(c.Kind))))
			}
			return nil
		})); err != nil {
		return nil, err
	}
	// Observed on each metric export rather than maintained on every write: one GROUP BY is cheaper than never being wrong.
	if _, err := m.Int64ObservableGauge("dispute.by_state", metric.WithDescription("Disputes currently in each state"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			counts, err := store.CountByState(ctx)
			if err != nil {
				return err
			}
			for _, c := range counts {
				o.Observe(c.N, metric.WithAttributes(attribute.String("tenant", c.TenantID.String()),
					attribute.String("regime", string(c.Regime)), attribute.String("state", string(c.State))))
			}
			return nil
		})); err != nil {
		return nil, err
	}
	return &Service{store: store, now: now, tracer: otel.Tracer(scopeName), transitions: transitions, replays: replays,
		timeInState: timeInState, deadlineSlack: deadlineSlack}, nil
}

// DisputeView is the API representation of a dispute and its log; it is also what idempotent replays return.
type DisputeView struct {
	ID             uuid.UUID       `json:"id"`
	Regime         domain.Regime   `json:"regime"`
	State          domain.State    `json:"state"`
	Appeals        int             `json:"appeals"`
	Version        int64           `json:"version"`
	TransactionID  uuid.UUID       `json:"transactionId"`
	AccountID      uuid.UUID       `json:"accountId"`
	DisputedAmount decimal.Decimal `json:"disputedAmount"`
	Currency       string          `json:"currency"`
	OpenedAt       time.Time       `json:"openedAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	AllowedEvents  []domain.Event  `json:"allowedEvents"`
	Events         []EventView     `json:"events"`
	Deadlines      []DeadlineView  `json:"deadlines"`
}

// DeadlineView is one regulatory clock as exposed by the API; Status is evaluated when the view is built.
type DeadlineView struct {
	Kind      domain.DeadlineKind   `json:"kind"`
	Cycle     int                   `json:"cycle"`
	StartedAt time.Time             `json:"startedAt"`
	DueAt     time.Time             `json:"dueAt"`
	MetAt     *time.Time            `json:"metAt,omitempty"`
	Status    domain.DeadlineStatus `json:"status"`
	Basis     string                `json:"basis"`
}

// EventView is one log entry as exposed by the API.
type EventView struct {
	Seq        int             `json:"seq"`
	Event      domain.Event    `json:"event"`
	FromState  domain.State    `json:"fromState"`
	ToState    domain.State    `json:"toState"`
	Actor      string          `json:"actor"`
	Payload    json.RawMessage `json:"payload"`
	TraceID    *string         `json:"traceId,omitempty"`
	OccurredAt time.Time       `json:"occurredAt"`
}

// Idempotency identifies a request for replay; a zero value means the request is not idempotent.
type Idempotency struct {
	Key         string
	RequestBody []byte
}

// CreateDisputeInput opens a dispute against a transaction.
type CreateDisputeInput struct {
	TransactionID uuid.UUID
	Actor         string
	Idempotency   Idempotency
}

// ApplyEventInput moves a dispute along its lifecycle.
type ApplyEventInput struct {
	DisputeID   uuid.UUID
	Event       domain.Event
	Actor       string
	Payload     json.RawMessage
	Idempotency Idempotency
}

// Result carries the view plus whether it came from a stored idempotent response.
type Result struct {
	View     DisputeView
	Replayed bool
}

// CreateDispute derives the regime from the transaction and records INITIATED as event 1.
func (s *Service) CreateDispute(ctx context.Context, in CreateDisputeInput) (Result, error) {
	ctx, span := s.tracer.Start(ctx, "dispute.create")
	defer span.End()

	return s.idempotent(ctx, "dispute.create", in.Idempotency, func(tx Tx) (DisputeView, error) {
		txn, err := tx.GetTransaction(ctx, in.TransactionID)
		if err != nil {
			return DisputeView{}, err
		}
		regime, err := domain.RegimeFor(txn.Rail, txn.AccountCurrency)
		if err != nil {
			return DisputeView{}, err
		}
		now := s.now()
		id, err := uuid.NewV7()
		if err != nil {
			return DisputeView{}, err
		}
		rec := DisputeRecord{
			ID: id, Regime: regime, State: domain.StateInitiated, Version: 1,
			TransactionID: txn.ID, AccountID: txn.AccountID,
			DisputedAmount: txn.Amount, Currency: txn.Currency,
			OpenedAt: now, UpdatedAt: now,
		}
		if err := tx.InsertDispute(ctx, rec); err != nil {
			return DisputeView{}, err
		}
		ev := EventRecord{
			Seq: 1, Event: "OPENED", FromState: "", ToState: domain.StateInitiated,
			Actor: in.Actor, Payload: []byte("{}"), IdempotencyKey: keyPtr(in.Idempotency),
			TraceID: traceIDPtr(ctx), OccurredAt: now,
		}
		if err := tx.AppendEvent(ctx, id, ev); err != nil {
			return DisputeView{}, err
		}
		// The regime's clocks start now, in the tenant's calendar; they are rows, not recomputed, so a later
		// change to a calendar or a rule never moves a deadline that was already communicated.
		rules, err := domain.RulesFor(regime)
		if err != nil {
			return DisputeView{}, err
		}
		cal, err := tx.TenantCalendar(ctx)
		if err != nil {
			return DisputeView{}, err
		}
		if err := tx.InsertDeadlines(ctx, id, rules.OpeningDeadlines(now, cal)); err != nil {
			return DisputeView{}, err
		}
		span.SetAttributes(attribute.String("dispute.id", id.String()), attribute.String("dispute.regime", string(regime)))
		s.transitions.Add(ctx, 1, metric.WithAttributes(tenantAttr(ctx),
			attribute.String("regime", string(regime)), attribute.String("event", "OPENED"), attribute.String("to", string(domain.StateInitiated))))
		return s.view(ctx, tx, rec)
	})
}

// ApplyEvent runs the state machine and, if it accepts, appends the event and advances the record under a version check.
func (s *Service) ApplyEvent(ctx context.Context, in ApplyEventInput) (Result, error) {
	ctx, span := s.tracer.Start(ctx, "dispute.apply_event", trace.WithAttributes(
		attribute.String("dispute.id", in.DisputeID.String()), attribute.String("dispute.event", string(in.Event))))
	defer span.End()

	return s.idempotent(ctx, "dispute.apply_event", in.Idempotency, func(tx Tx) (DisputeView, error) {
		rec, err := tx.GetDispute(ctx, in.DisputeID)
		if err != nil {
			return DisputeView{}, err
		}
		next, err := rec.Lifecycle().Apply(in.Event)
		if err != nil {
			return DisputeView{}, err
		}
		now := s.now()
		if err := tx.UpdateDisputeState(ctx, rec.ID, rec.Version, next.State, next.Appeals, now); err != nil {
			return DisputeView{}, err
		}
		payload := in.Payload
		if len(payload) == 0 {
			payload = json.RawMessage("{}")
		}
		ev := EventRecord{
			Seq: int(rec.Version) + 1, Event: in.Event, FromState: rec.State, ToState: next.State,
			Actor: in.Actor, Payload: payload, IdempotencyKey: keyPtr(in.Idempotency),
			TraceID: traceIDPtr(ctx), OccurredAt: now,
		}
		if err := tx.AppendEvent(ctx, rec.ID, ev); err != nil {
			return DisputeView{}, err
		}
		if err := s.settleDeadlines(ctx, tx, rec, next, now); err != nil {
			return DisputeView{}, err
		}
		span.SetAttributes(attribute.String("dispute.regime", string(rec.Regime)),
			attribute.String("dispute.from", string(rec.State)), attribute.String("dispute.to", string(next.State)))
		attrs := metric.WithAttributes(tenantAttr(ctx), attribute.String("regime", string(rec.Regime)),
			attribute.String("event", string(in.Event)), attribute.String("to", string(next.State)))
		s.transitions.Add(ctx, 1, attrs)
		s.timeInState.Record(ctx, now.Sub(rec.UpdatedAt).Seconds(), metric.WithAttributes(
			attribute.String("regime", string(rec.Regime)), attribute.String("state", string(rec.State))))

		rec.State, rec.Appeals, rec.Version, rec.UpdatedAt = next.State, next.Appeals, rec.Version+1, now
		return s.view(ctx, tx, rec)
	})
}

// settleDeadlines closes the clocks the new state satisfies or voids, and starts the appeal's resolution clock.
func (s *Service) settleDeadlines(ctx context.Context, tx Tx, rec DisputeRecord, next domain.Dispute, now time.Time) error {
	open, err := tx.ListDeadlines(ctx, rec.ID)
	if err != nil {
		return err
	}
	for _, d := range open {
		if !d.Open() {
			continue
		}
		switch domain.Settle(d.Kind, next.State) {
		case domain.Met:
			if err := tx.SettleDeadline(ctx, rec.ID, d.Kind, d.Cycle, true, now); err != nil {
				return err
			}
			s.deadlineSlack.Record(ctx, d.DueAt.Sub(now).Seconds(), metric.WithAttributes(
				attribute.String("regime", string(rec.Regime)), attribute.String("kind", string(d.Kind))))
		case domain.Void:
			if err := tx.SettleDeadline(ctx, rec.ID, d.Kind, d.Cycle, false, now); err != nil {
				return err
			}
		case domain.Untouched:
		}
	}
	if next.Appeals > rec.Appeals {
		rules, err := domain.RulesFor(rec.Regime)
		if err != nil {
			return err
		}
		cal, err := tx.TenantCalendar(ctx)
		if err != nil {
			return err
		}
		return tx.InsertDeadlines(ctx, rec.ID, rules.AppealDeadlines(next.Appeals, now, cal))
	}
	return nil
}

// DisputeSummary is one row of a list; NextDeadline is the open clock that runs out first, if any.
type DisputeSummary struct {
	ID             uuid.UUID
	Regime         domain.Regime
	State          domain.State
	TransactionID  uuid.UUID
	DisputedAmount decimal.Decimal
	Currency       string
	OpenedAt       time.Time
	UpdatedAt      time.Time
	NextDeadline   *DeadlineView
}

// Page is one page of a list plus where the next one starts.
type Page struct {
	Items []DisputeSummary
	Next  *Cursor
}

// ListDisputes pages through the tenant's disputes, newest first; the store's row-level security scopes the tenant.
func (s *Service) ListDisputes(ctx context.Context, q ListQuery) (Page, error) {
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 25
	}
	now := s.now()
	var page Page
	err := s.store.WithTx(ctx, func(tx Tx) error {
		// Ask for one more than the page to learn whether a next page exists without a second query.
		recs, err := tx.ListDisputes(ctx, ListQuery{State: q.State, After: q.After, Limit: q.Limit + 1, Overdue: q.Overdue, Now: now})
		if err != nil {
			return err
		}
		if len(recs) > q.Limit {
			last := recs[q.Limit-1]
			page.Next = &Cursor{OpenedAt: last.OpenedAt, ID: last.ID}
			recs = recs[:q.Limit]
		}
		ids := make([]uuid.UUID, 0, len(recs))
		for _, r := range recs {
			ids = append(ids, r.ID)
		}
		next, err := tx.NextDeadlines(ctx, ids)
		if err != nil {
			return err
		}
		page.Items = make([]DisputeSummary, 0, len(recs))
		for _, r := range recs {
			item := DisputeSummary{ID: r.ID, Regime: r.Regime, State: r.State, TransactionID: r.TransactionID,
				DisputedAmount: r.DisputedAmount, Currency: r.Currency, OpenedAt: r.OpenedAt, UpdatedAt: r.UpdatedAt}
			if d, ok := next[r.ID]; ok {
				v := deadlineView(d, now)
				item.NextDeadline = &v
			}
			page.Items = append(page.Items, item)
		}
		return nil
	})
	return page, err
}

// GetDispute returns the current state and full log.
func (s *Service) GetDispute(ctx context.Context, id uuid.UUID) (DisputeView, error) {
	var view DisputeView
	err := s.store.WithTx(ctx, func(tx Tx) error {
		rec, err := tx.GetDispute(ctx, id)
		if err != nil {
			return err
		}
		view, err = s.view(ctx, tx, rec)
		return err
	})
	return view, err
}

// idempotent wraps a write so a repeated key returns the first response and a mismatched body is refused.
func (s *Service) idempotent(ctx context.Context, scope string, idem Idempotency, fn func(Tx) (DisputeView, error)) (Result, error) {
	var out Result
	err := s.store.WithTx(ctx, func(tx Tx) error {
		var hash []byte
		if idem.Key != "" {
			sum := sha256.Sum256(idem.RequestBody)
			hash = sum[:]
			stored, err := tx.GetIdempotent(ctx, scope, idem.Key)
			switch {
			case err == nil:
				if !bytes.Equal(stored.RequestHash, hash) {
					return ErrIdempotencyReuse
				}
				if err := json.Unmarshal(stored.Body, &out.View); err != nil {
					return fmt.Errorf("application: decode stored response: %w", err)
				}
				out.Replayed = true
				s.replays.Add(ctx, 1, metric.WithAttributes(attribute.String("scope", scope)))
				return nil
			case !errors.Is(err, ErrNotFound):
				return err
			}
		}
		view, err := fn(tx)
		if err != nil {
			return err
		}
		if idem.Key != "" {
			body, err := json.Marshal(view)
			if err != nil {
				return err
			}
			if err := tx.PutIdempotent(ctx, scope, idem.Key, StoredResponse{RequestHash: hash, StatusCode: 0, Body: body}); err != nil {
				return err
			}
		}
		out = Result{View: view}
		return nil
	})
	return out, err
}

func (s *Service) view(ctx context.Context, tx Tx, rec DisputeRecord) (DisputeView, error) {
	events, err := tx.ListEvents(ctx, rec.ID)
	if err != nil {
		return DisputeView{}, err
	}
	views := make([]EventView, 0, len(events))
	for _, e := range events {
		views = append(views, EventView{Seq: e.Seq, Event: e.Event, FromState: e.FromState, ToState: e.ToState,
			Actor: e.Actor, Payload: json.RawMessage(e.Payload), TraceID: e.TraceID, OccurredAt: e.OccurredAt})
	}
	allowed := rec.Lifecycle().Allowed()
	if allowed == nil {
		allowed = []domain.Event{}
	}
	deadlines, err := tx.ListDeadlines(ctx, rec.ID)
	if err != nil {
		return DisputeView{}, err
	}
	now := s.now()
	dviews := make([]DeadlineView, 0, len(deadlines))
	for _, d := range deadlines {
		dviews = append(dviews, deadlineView(d, now))
	}
	return DisputeView{
		ID: rec.ID, Regime: rec.Regime, State: rec.State, Appeals: rec.Appeals, Version: rec.Version,
		TransactionID: rec.TransactionID, AccountID: rec.AccountID, DisputedAmount: rec.DisputedAmount,
		Currency: rec.Currency, OpenedAt: rec.OpenedAt, UpdatedAt: rec.UpdatedAt,
		AllowedEvents: allowed, Events: views, Deadlines: dviews,
	}, nil
}

func deadlineView(d domain.Deadline, now time.Time) DeadlineView {
	return DeadlineView{Kind: d.Kind, Cycle: d.Cycle, StartedAt: d.StartedAt, DueAt: d.DueAt, MetAt: d.MetAt, Status: d.Status(now), Basis: d.Basis}
}

func keyPtr(i Idempotency) *string {
	if i.Key == "" {
		return nil
	}
	return &i.Key
}

func traceIDPtr(ctx context.Context) *string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	id := sc.TraceID().String()
	return &id
}

// tenantAttr labels business metrics by tenant; cardinality is the tenant count, which stays small by design (ADR 0008).
func tenantAttr(ctx context.Context) attribute.KeyValue {
	id, _ := tenant.IDFrom(ctx)
	return attribute.String("tenant", id.String())
}
