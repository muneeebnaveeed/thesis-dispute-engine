package application

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

// SearchFilters is what an analyst's sentence appears to ask for. A nil field is one nobody was sure about,
// which the workbench leaves as it found it.
type SearchFilters struct {
	State   *domain.State
	Reason  *domain.Reason
	Overdue *bool
}

// anyOption lets the model say the sentence names no particular state or reason, which is different from
// being unsure: without it, "show me everything" would have to land on some state.
const anyOption = "any"

func stateOptions() map[string]string {
	out := map[string]string{anyOption: "no particular state"}
	for _, s := range domain.AllStates() {
		out[string(s)] = strings.ToLower(strings.ReplaceAll(string(s), "_", " "))
	}
	return out
}

func reasonOptions() map[string]string {
	out := map[string]string{anyOption: "no particular reason"}
	for _, r := range domain.AllReasons() {
		out[string(r)] = domain.ReasonMeaning(r)
	}
	return out
}

// openedPayload records what the analyst was shown beside what they chose. An opened dispute with no
// suggestion keeps the empty payload it has always had.
func openedPayload(proposal *ReasonProposal, chosen domain.Reason) []byte {
	if proposal == nil || !proposal.Confident {
		return []byte("{}")
	}
	payload, err := json.Marshal(map[string]any{"suggestion": map[string]any{
		"field":       "reason",
		"proposed":    proposal.Reason,
		"probability": proposal.Probability,
		"accepted":    proposal.Reason == chosen,
	}})
	if err != nil {
		return []byte("{}")
	}
	return payload
}

// recordAcceptance is the only measure of whether any of this helps: how often an analyst keeps what the
// model offered. The label is the field, never the value, so the counter stays a small fixed set.
func (s *Service) recordAcceptance(ctx context.Context, field string, proposal *ReasonProposal, chosen domain.Reason) {
	if proposal == nil || !proposal.Confident {
		return
	}
	counter, _ := decisionMeters()
	if counter == nil {
		return
	}
	accepted, _ := otel.Meter(scopeName).Int64Counter("dispute.proposals_accepted",
		metric.WithDescription("Proposals an analyst kept or overrode, by field"))
	if accepted == nil {
		return
	}
	accepted.Add(ctx, 1, metric.WithAttributes(
		attribute.String("field", field), attribute.Bool("accepted", proposal.Reason == chosen)))
}

// ReasonProposal is a reason the model offered and how sure it was. Confident is false when it offered
// nothing, which is the normal case for prose that does not clearly describe one of the four.
type ReasonProposal struct {
	Reason      domain.Reason
	Probability float64
	Confident   bool
}

// SuggestDisputeReason reads what a customer wrote as one of the four reasons. The description is read and
// not stored: the record holds the reason a person chose, never the prose the machine read (ADR 0024).
func (s *Service) SuggestDisputeReason(ctx context.Context, description string) ReasonProposal {
	answer, ok := Propose(ctx, s.decisions, map[string]string{"complaint": description}, "reason", Question{
		Kind:    KindChoice,
		Ask:     "Why is the customer disputing this payment?",
		Options: intakeOptions(),
	})
	if !ok || answer.Value == noneOption {
		return ReasonProposal{}
	}
	reason, err := domain.ParseReason(answer.Value)
	if err != nil {
		return ReasonProposal{}
	}
	return ReasonProposal{Reason: reason, Probability: answer.Probability, Confident: true}
}

// noneOption is how the model says the text describes no dispute at all. Without it a forced choice makes
// something up: "I have a question about my account" came back as an unauthorised payment, confidently.
const noneOption = "none"

func intakeOptions() map[string]string {
	out := map[string]string{noneOption: "the text does not describe a disputed payment at all"}
	for _, r := range domain.AllReasons() {
		out[string(r)] = domain.ReasonMeaning(r)
	}
	return out
}

// SuggestSearchFilters reads a sentence as filters. It asks all three questions in one pass, which is what
// the model is for, and returns only the answers that cleared the confidence gate (ADR 0024).
func (s *Service) SuggestSearchFilters(ctx context.Context, query string) SearchFilters {
	answers := ProposeAll(ctx, s.decisions, map[string]string{"query": query}, map[string]Question{
		"state":   {Kind: KindChoice, Ask: "Which dispute state is the analyst asking for?", Options: stateOptions()},
		"reason":  {Kind: KindChoice, Ask: "Which dispute reason is the analyst asking for?", Options: reasonOptions()},
		"overdue": {Kind: KindYesNo, Ask: "Is the analyst asking only for disputes that are past a deadline?"},
	})

	var out SearchFilters
	if answer, ok := answers["state"]; ok && answer.Value != anyOption {
		if state, err := domain.ParseState(answer.Value); err == nil {
			out.State = &state
		}
	}
	if answer, ok := answers["reason"]; ok && answer.Value != anyOption {
		if reason, err := domain.ParseReason(answer.Value); err == nil {
			out.Reason = &reason
		}
	}
	// only a yes is worth sending: "not overdue" is the unfiltered list, which is what the page already shows
	if answer, ok := answers["overdue"]; ok && answer.Value == "yes" {
		yes := true
		out.Overdue = &yes
	}
	return out
}

// relevanceGate is asked alongside the real questions; its id cannot collide because question ids come from
// the questionnaire files, which are lower snake case.
const relevanceGate = "__is_about_this_dispute"

// QuestionnaireProposal is one answer the model offered for one question.
type QuestionnaireProposal struct {
	Value       string // "yes" or "no"
	Probability float64
}

// SuggestQuestionnaireAnswers reads a customer's reply as answers to this dispute's questionnaire. Only the
// yes and no questions are asked: the model has no notion of a date, and free text is not a decision. The
// reply is read and not stored, and every answer is a proposal the analyst confirms (ADR 0024).
func (s *Service) SuggestQuestionnaireAnswers(ctx context.Context, disputeID uuid.UUID, reply string) (map[string]QuestionnaireProposal, error) {
	var questions []domain.Question
	if err := s.store.WithTx(ctx, func(tx Tx) error {
		rec, err := tx.GetDispute(ctx, disputeID)
		if err != nil {
			return err
		}
		// the questions actually sent, not the ones the reason would produce today, in case the set has moved
		if q, err := tx.GetQuestionnaire(ctx, disputeID); err == nil && len(q.Questions) > 0 {
			questions = q.Questions
			return nil
		}
		questions = domain.QuestionSet(rec.Reason)
		return nil
	}); err != nil {
		return nil, err
	}

	asked := map[string]Question{}
	for _, q := range questions {
		if q.Type == domain.AnswerYesNo {
			asked[q.ID] = Question{Kind: KindYesNo, Ask: q.Text}
		}
	}
	if len(asked) == 0 {
		return map[string]QuestionnaireProposal{}, nil
	}
	// A yes/no question cannot abstain: asked about prose that answers nothing, the model returns a confident
	// "no" to every one of them. So the batch carries a gate, and nothing is proposed unless the reply is
	// actually about this dispute.
	asked[relevanceGate] = Question{
		Kind: KindYesNo,
		Ask:  "Is this message a customer answering questions about a disputed card payment?",
	}

	answers := ProposeAll(ctx, s.decisions, map[string]string{"reply": reply}, asked)
	out := make(map[string]QuestionnaireProposal, len(answers))
	if gate, ok := answers[relevanceGate]; !ok || gate.Value != "yes" {
		return out, nil
	}
	for id, answer := range answers {
		if id == relevanceGate {
			continue
		}
		out[id] = QuestionnaireProposal(answer)
	}
	return out, nil
}
