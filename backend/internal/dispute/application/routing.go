package application

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

// Route picks a new dispute's team: the first rule whose set fields all match, else the tenant's default team.
func Route(a Access, rail domain.Rail, reason domain.Reason, tier string, amount decimal.Decimal) string {
	for _, r := range a.Routing {
		if r.Rail != nil && *r.Rail != rail {
			continue
		}
		if r.Reason != nil && *r.Reason != reason {
			continue
		}
		if r.RiskTier != nil && *r.RiskTier != tier {
			continue
		}
		if r.MinAmount != nil && amount.LessThan(*r.MinAmount) {
			continue
		}
		return r.Team
	}
	return a.DefaultTeam
}

// EventReassigned is the log entry for a move between teams; it changes no state, so it is not a lifecycle event.
const EventReassigned domain.Event = "REASSIGNED"

// ErrUnknownTeam is a reassignment to a team the tenant does not have.
var ErrUnknownTeam = errs.New(errs.Unprocessable, "unknown-team", "no such team in this tenant")

// Reassign moves a dispute to another team; the move is an event, so the log shows who moved it and why it was allowed.
func (s *Service) Reassign(ctx context.Context, disputeID uuid.UUID, team string) error {
	return s.store.WithTx(ctx, func(tx Tx) error {
		rec, err := tx.GetDispute(ctx, disputeID)
		if err != nil {
			return err
		}
		p, _ := principal.From(ctx)
		a, err := tx.Access(ctx, p)
		if err != nil {
			return err
		}
		if !slices.Contains(a.Teams, team) {
			return ErrUnknownTeam
		}
		f, err := facts(ctx, tx, rec)
		if err != nil {
			return err
		}
		dec, err := s.authorize(ctx, tx, ActionReassign, &f)
		if err != nil {
			return err
		}
		now := s.now()
		payload, err := json.Marshal(map[string]any{"from": rec.Team, "to": team, "authorizedBy": dec.Policies})
		if err != nil {
			return err
		}
		// logged before the move: afterwards the dispute may be outside the caller's teams, and its log with it
		display, actorID := actorOf(ctx)
		if err := tx.AppendEvent(ctx, rec.ID, EventRecord{Seq: int(rec.Version) + 1, Event: EventReassigned, FromState: rec.State,
			ToState: rec.State, Actor: display, ActorID: actorID, Payload: payload, TraceID: traceIDPtr(ctx), OccurredAt: now}); err != nil {
			return err
		}
		return tx.UpdateDisputeTeam(ctx, rec.ID, rec.Version, team, now)
	})
}
