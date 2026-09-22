package application

import (
	"context"
	"strings"

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
