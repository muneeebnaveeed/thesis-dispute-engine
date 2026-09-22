package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
)

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
