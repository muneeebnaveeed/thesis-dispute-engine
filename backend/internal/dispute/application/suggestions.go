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
