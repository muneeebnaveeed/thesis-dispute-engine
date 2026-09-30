package application

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// WithAuthorizer sets the policy engine every operation is decided by; NewService refuses to build without one.
func WithAuthorizer(a Authorizer) Option { return func(s *Service) { s.authz = a } }

// authorize decides action for the caller in tx and records the decision; d nil is the tenant, d with a zero ID is
// the team a create routes to.
func (s *Service) authorize(ctx context.Context, tx Tx, action Action, d *DisputeFacts) (Decision, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return Decision{}, ErrForbidden
	}
	a, err := tx.Access(ctx, p)
	if err != nil {
		return Decision{}, err
	}
	tid, _ := tenant.IDFrom(ctx)
	dec, err := s.authz.Decide(tid, p, a, action, d)
	s.authzDecisions.Add(ctx, 1, metric.WithAttributes(tenantAttr(ctx), attribute.String("action", string(action)), attribute.Bool("allowed", dec.Allowed && err == nil)))
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("authz.action", string(action)), attribute.Bool("authz.allowed", dec.Allowed && err == nil),
		attribute.StringSlice("authz.policies", dec.Policies), attribute.Int64("authz.policy_version", a.Version))
	if err != nil {
		span.RecordError(err)
		return Decision{}, ErrForbidden
	}
	if !dec.Allowed {
		if len(dec.Policies) > 0 {
			return dec, errs.Wrap(ErrForbidden, "denied by %s", dec.Policies[0])
		}
		return dec, errs.Wrap(ErrForbidden, "no grant allows %s", action)
	}
	return dec, nil
}

// permitted filters events down to those the caller may apply, without recording each probe as a decision.
func (s *Service) permitted(ctx context.Context, tx Tx, events []domain.Event, f DisputeFacts) ([]domain.Event, error) {
	out := []domain.Event{}
	p, ok := principal.From(ctx)
	if !ok {
		return out, nil
	}
	a, err := tx.Access(ctx, p)
	if err != nil {
		return nil, err
	}
	tid, _ := tenant.IDFrom(ctx)
	for _, e := range events {
		if dec, err := s.authz.Decide(tid, p, a, EventAction(e), &f); err == nil && dec.Allowed {
			out = append(out, e)
		}
	}
	return out, nil
}

// facts is what policies may read about a dispute, including who opened its investigation.
func facts(ctx context.Context, tx Tx, rec DisputeRecord) (DisputeFacts, error) {
	events, err := tx.ListEvents(ctx, rec.ID)
	if err != nil {
		return DisputeFacts{}, err
	}
	var opener string
	for _, e := range events {
		if e.Event == domain.EventOpenInvestigation {
			opener = e.ActorID
		}
	}
	txn, err := tx.GetTransaction(ctx, rec.TransactionID)
	if err != nil {
		return DisputeFacts{}, err
	}
	return DisputeFacts{ID: rec.ID, Team: rec.Team, Amount: rec.DisputedAmount, Currency: rec.Currency,
		Rail: txn.Rail, State: rec.State, InvestigationOpenedBy: opener}, nil
}

// authorizeDispute decides action on a dispute for operations that name it but do not otherwise read it.
func (s *Service) authorizeDispute(ctx context.Context, tx Tx, action Action, disputeID uuid.UUID) error {
	rec, err := tx.GetDispute(ctx, disputeID)
	if err != nil {
		return err
	}
	f, err := facts(ctx, tx, rec)
	if err != nil {
		return err
	}
	_, err = s.authorize(ctx, tx, action, &f)
	return err
}

// withAuthorizedBy records which policies allowed the event, so the log answers "why was this permitted".
func withAuthorizedBy(payload json.RawMessage, policies []string) (json.RawMessage, error) {
	m := map[string]any{}
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, err
	}
	m["authorizedBy"] = policies
	return json.Marshal(m)
}

// Authorize checks a tenant-level action outside a dispute transaction, for handlers that own their storage.
func (s *Service) Authorize(ctx context.Context, action Action) error {
	return s.store.WithTx(ctx, func(tx Tx) error {
		_, err := s.authorize(ctx, tx, action, nil)
		return err
	})
}
