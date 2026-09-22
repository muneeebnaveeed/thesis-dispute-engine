package application

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// MinConfidence is the probability below which a proposal is dropped rather than shown. A field left empty
// costs an analyst less attention than a field filled in wrongly, so the gate is deliberately high.
const MinConfidence = 0.75

// QuestionKind is the shape of an answer the model may give: one of a closed set, or yes and no.
type QuestionKind string

// Question kinds.
const (
	KindChoice QuestionKind = "choice"
	KindYesNo  QuestionKind = "yesno"
)

// Question asks for one typed value. Options carry what each value means, in the words a person would use,
// and are required for KindChoice and ignored for KindYesNo.
type Question struct {
	Kind    QuestionKind
	Ask     string
	Options map[string]string
}

// Answer is a proposed value and how sure the model was. For KindYesNo the value is "yes" or "no".
type Answer struct {
	Value       string
	Probability float64
}

// Confident reports whether a proposal is strong enough to show. See MinConfidence.
func (a Answer) Confident() bool { return a.Probability >= MinConfidence }

// Decisions turns prose into values from a closed vocabulary. It never decides anything: every answer is a
// proposal an analyst confirms, and no transition, clock, posting or letter may depend on one (ADR 0024).
// A nil Decisions is the supported case, and means every proposal is absent.
type Decisions interface {
	// Decide answers each question about the state. The returned map holds an entry per question it answered;
	// a question it could not answer is absent rather than guessed.
	Decide(ctx context.Context, state map[string]string, questions map[string]Question) (map[string]Answer, error)
}

// ErrDecisionsUnavailable means the model could not be reached. Callers degrade to an empty field: a proposal
// is a convenience, so its absence is never an error the analyst sees.
var ErrDecisionsUnavailable = errs.New(errs.Unavailable, "unavailable", "the suggestion service is temporarily unavailable")

// Outcomes a proposal can have, which is what the counter is sliced by.
const (
	outcomeShown       = "shown"
	outcomeUnsure      = "dropped_low_confidence"
	outcomeNoAnswer    = "no_answer"
	outcomeUnavailable = "unavailable"
	outcomeDisabled    = "disabled"
)

var (
	proposalMeters sync.Once
	proposals      metric.Int64Counter
	probabilities  metric.Float64Histogram
)

// meters are lazy so a caller that never proposes never registers instruments.
func decisionMeters() (metric.Int64Counter, metric.Float64Histogram) {
	proposalMeters.Do(func() {
		m := otel.Meter(scopeName)
		proposals, _ = m.Int64Counter("dispute.proposals",
			metric.WithDescription("Values a typed-decision model offered to fill in, by question and outcome"))
		// buckets around the confidence gate: what matters is whether answers pile up either side of it
		probabilities, _ = m.Float64Histogram("dispute.proposal_probability",
			metric.WithDescription("How sure the model was, whether or not the proposal was shown"),
			metric.WithExplicitBucketBoundaries(0.25, 0.5, 0.6, 0.7, 0.75, 0.8, 0.9, 0.95, 0.99))
	})
	return proposals, probabilities
}

func record(ctx context.Context, id, outcome string, probability float64) {
	counter, histogram := decisionMeters()
	if counter == nil {
		return
	}
	counter.Add(ctx, 1, metric.WithAttributes(attribute.String("question", id), attribute.String("outcome", outcome)))
	if outcome == outcomeShown || outcome == outcomeUnsure {
		histogram.Record(ctx, probability, metric.WithAttributes(attribute.String("question", id)))
	}
}

// Propose asks for one question's answer and returns it only when the model was confident. Any failure is a
// missing proposal, never an error: the caller renders an empty field either way. Every call is counted by
// outcome, because the useful question about this feature is how often it actually helps.
func Propose(ctx context.Context, d Decisions, state map[string]string, id string, q Question) (Answer, bool) {
	if d == nil {
		record(ctx, id, outcomeDisabled, 0)
		return Answer{}, false
	}
	answers, err := d.Decide(ctx, state, map[string]Question{id: q})
	if err != nil {
		record(ctx, id, outcomeUnavailable, 0)
		return Answer{}, false
	}
	answer, ok := answers[id]
	switch {
	case !ok:
		record(ctx, id, outcomeNoAnswer, 0)
		return Answer{}, false
	case !answer.Confident():
		record(ctx, id, outcomeUnsure, answer.Probability)
		return Answer{}, false
	}
	record(ctx, id, outcomeShown, answer.Probability)
	return answer, true
}

// ProposeAll asks several questions in one pass, which is what the model is built for, and returns only the
// answers that cleared the gate. The map is empty rather than nil-checked by callers.
func ProposeAll(ctx context.Context, d Decisions, state map[string]string, questions map[string]Question) map[string]Answer {
	out := map[string]Answer{}
	if d == nil {
		for id := range questions {
			record(ctx, id, outcomeDisabled, 0)
		}
		return out
	}
	answers, err := d.Decide(ctx, state, questions)
	if err != nil {
		for id := range questions {
			record(ctx, id, outcomeUnavailable, 0)
		}
		return out
	}
	for id := range questions {
		answer, ok := answers[id]
		switch {
		case !ok:
			record(ctx, id, outcomeNoAnswer, 0)
		case !answer.Confident():
			record(ctx, id, outcomeUnsure, answer.Probability)
		default:
			record(ctx, id, outcomeShown, answer.Probability)
			out[id] = answer
		}
	}
	return out
}
