package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
)

func serviceWithDecisions(t *testing.T, d application.Decisions) *application.Service {
	t.Helper()
	svc, err := application.NewService(apptest.NewMemStore(), nil, application.WithDecisions(d))
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

type fakeDecisions struct {
	answers map[string]application.Answer
	err     error
}

func (f fakeDecisions) Decide(context.Context, map[string]string, map[string]application.Question) (map[string]application.Answer, error) {
	return f.answers, f.err
}

func TestProposeAllKeepsOnlyWhatClearedTheGate(t *testing.T) {
	questions := map[string]application.Question{
		"sure":    {Kind: application.KindYesNo, Ask: "?"},
		"unsure":  {Kind: application.KindYesNo, Ask: "?"},
		"missing": {Kind: application.KindYesNo, Ask: "?"},
	}
	model := fakeDecisions{answers: map[string]application.Answer{
		"sure":   {Value: "yes", Probability: 0.91},
		"unsure": {Value: "no", Probability: 0.6},
	}}
	got := application.ProposeAll(context.Background(), model, nil, questions)
	if len(got) != 1 || got["sure"].Value != "yes" {
		t.Errorf("proposals = %+v, want only the confident one", got)
	}
}

func TestProposeAllSurvivesAnUnavailableModel(t *testing.T) {
	questions := map[string]application.Question{"q": {Kind: application.KindYesNo, Ask: "?"}}
	if got := application.ProposeAll(context.Background(), fakeDecisions{err: errors.New("down")}, nil, questions); len(got) != 0 {
		t.Errorf("proposals = %+v, want none", got)
	}
	if got := application.ProposeAll(context.Background(), nil, nil, questions); len(got) != 0 {
		t.Errorf("proposals without a model = %+v, want none", got)
	}
}

// Text that describes no dispute must produce no proposal. Without an explicit way to say "none of these",
// a forced choice invents one: "I have a question about my account" came back as an unauthorised payment.
func TestSuggestDisputeReasonRefusesTextThatIsNotADispute(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer application.Answer
		want   bool
	}{
		{"none of the four", application.Answer{Value: "none", Probability: 0.95}, false},
		{"under the gate", application.Answer{Value: "DUPLICATE", Probability: 0.6}, false},
		{"a clear duplicate", application.Answer{Value: "DUPLICATE", Probability: 0.99}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := serviceWithDecisions(t, fakeDecisions{answers: map[string]application.Answer{"reason": tc.answer}})
			got := svc.SuggestDisputeReason(context.Background(), "whatever the customer wrote")
			if got.Confident != tc.want {
				t.Errorf("confident = %v, want %v (proposal %+v)", got.Confident, tc.want, got)
			}
		})
	}
}
