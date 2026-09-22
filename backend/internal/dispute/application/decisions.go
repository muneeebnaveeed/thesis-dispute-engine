package application

import (
	"context"

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

// Propose asks for one question's answer and returns it only when the model was confident. Any failure is a
// missing proposal, never an error: the caller renders an empty field either way.
func Propose(ctx context.Context, d Decisions, state map[string]string, id string, q Question) (Answer, bool) {
	if d == nil {
		return Answer{}, false
	}
	answers, err := d.Decide(ctx, state, map[string]Question{id: q})
	if err != nil {
		return Answer{}, false
	}
	answer, ok := answers[id]
	if !ok || !answer.Confident() {
		return Answer{}, false
	}
	return answer, true
}
