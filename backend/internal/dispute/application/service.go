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
)

const scopeName = "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"

// Service is the dispute use cases.
type Service struct {
	store  Store
	now    Clock
	tracer trace.Tracer

	transitions metric.Int64Counter
	replays     metric.Int64Counter
	timeInState metric.Float64Histogram
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
	// Observed on each metric export rather than maintained on every write: one GROUP BY is cheaper than never being wrong.
	if _, err := m.Int64ObservableGauge("dispute.by_state", metric.WithDescription("Disputes currently in each state"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			counts, err := store.CountByState(ctx)
			if err != nil {
				return err
			}
			for _, c := range counts {
				o.Observe(c.N, metric.WithAttributes(attribute.String("regime", string(c.Regime)), attribute.String("state", string(c.State))))
			}
			return nil
		})); err != nil {
		return nil, err
	}
	return &Service{store: store, now: now, tracer: otel.Tracer(scopeName), transitions: transitions, replays: replays, timeInState: timeInState}, nil
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
		span.SetAttributes(attribute.String("dispute.id", id.String()), attribute.String("dispute.regime", string(regime)))
		s.transitions.Add(ctx, 1, metric.WithAttributes(
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
		span.SetAttributes(attribute.String("dispute.regime", string(rec.Regime)),
			attribute.String("dispute.from", string(rec.State)), attribute.String("dispute.to", string(next.State)))
		attrs := metric.WithAttributes(attribute.String("regime", string(rec.Regime)),
			attribute.String("event", string(in.Event)), attribute.String("to", string(next.State)))
		s.transitions.Add(ctx, 1, attrs)
		s.timeInState.Record(ctx, now.Sub(rec.UpdatedAt).Seconds(), metric.WithAttributes(
			attribute.String("regime", string(rec.Regime)), attribute.String("state", string(rec.State))))

		rec.State, rec.Appeals, rec.Version, rec.UpdatedAt = next.State, next.Appeals, rec.Version+1, now
		return s.view(ctx, tx, rec)
	})
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
	return DisputeView{
		ID: rec.ID, Regime: rec.Regime, State: rec.State, Appeals: rec.Appeals, Version: rec.Version,
		TransactionID: rec.TransactionID, AccountID: rec.AccountID, DisputedAmount: rec.DisputedAmount,
		Currency: rec.Currency, OpenedAt: rec.OpenedAt, UpdatedAt: rec.UpdatedAt,
		AllowedEvents: allowed, Events: views,
	}, nil
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
