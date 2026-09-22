package typedmodel_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/typedmodel"
)

func serve(t *testing.T, status int, body string, seen *map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			_ = json.NewDecoder(r.Body).Decode(seen)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer k" {
			t.Errorf("authorization = %q", got)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDecideMapsBothQuestionKinds(t *testing.T) {
	var sent map[string]any
	srv := serve(t, http.StatusOK, `{"answers":{
		"reason":{"type":"choice","choice":"DUPLICATE","confidence":0.93},
		"seen_before":{"type":"noul","noul":0.82},
		"police":{"type":"noul","noul":0.10}}}`, &sent)
	client := typedmodel.New(srv.URL, "m-1", "k", time.Second)

	answers, err := client.Decide(context.Background(), map[string]string{"body": "charged twice"}, map[string]application.Question{
		"reason":      {Kind: application.KindChoice, Ask: "why?", Options: map[string]string{"DUPLICATE": "charged twice"}},
		"seen_before": {Kind: application.KindYesNo, Ask: "seen before?"},
		"police":      {Kind: application.KindYesNo, Ask: "police?"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := answers["reason"]; got.Value != "DUPLICATE" || got.Probability != 0.93 {
		t.Errorf("choice = %+v", got)
	}
	// a yes/no answer is a probability of yes: confidence is its distance from the middle, either way
	if got := answers["seen_before"]; got.Value != "yes" || got.Probability != 0.82 {
		t.Errorf("yes = %+v", got)
	}
	if got := answers["police"]; got.Value != "no" || got.Probability != 0.90 {
		t.Errorf("no = %+v", got)
	}
	questions, _ := sent["questions"].(map[string]any)
	reason, _ := questions["reason"].(map[string]any)
	police, _ := questions["police"].(map[string]any)
	if reason["type"] != "choice" || police["type"] != "noul" {
		t.Errorf("wire question types: %v", questions)
	}
	if sent["model"] != "m-1" {
		t.Errorf("model = %v", sent["model"])
	}
}

func TestDecideReportsUnavailable(t *testing.T) {
	srv := serve(t, http.StatusInternalServerError, `nope`, nil)
	client := typedmodel.New(srv.URL, "m-1", "k", time.Second)
	if _, err := client.Decide(context.Background(), nil, map[string]application.Question{
		"q": {Kind: application.KindYesNo, Ask: "?"},
	}); !errors.Is(err, application.ErrDecisionsUnavailable) {
		t.Errorf("err = %v, want ErrDecisionsUnavailable", err)
	}
}

func TestNewWithoutKeyIsNil(t *testing.T) {
	if client := typedmodel.New("", "", "", time.Second); client != nil {
		t.Errorf("a client without a key should be nil, got %+v", client)
	}
}

// Propose is the whole contract callers see: a proposal, or nothing at all.
func TestProposeDropsWhatItIsNotSureOf(t *testing.T) {
	srv := serve(t, http.StatusOK, `{"answers":{"reason":{"type":"choice","choice":"DUPLICATE","confidence":0.51}}}`, nil)
	client := typedmodel.New(srv.URL, "m-1", "k", time.Second)
	question := application.Question{Kind: application.KindChoice, Ask: "why?", Options: map[string]string{"DUPLICATE": "twice"}}

	if _, ok := application.Propose(context.Background(), client, nil, "reason", question); ok {
		t.Error("a proposal under the confidence gate should not be shown")
	}
	if _, ok := application.Propose(context.Background(), nil, nil, "reason", question); ok {
		t.Error("no configured model should mean no proposal")
	}

	sure := serve(t, http.StatusOK, `{"answers":{"reason":{"type":"choice","choice":"DUPLICATE","confidence":0.97}}}`, nil)
	answer, ok := application.Propose(context.Background(), typedmodel.New(sure.URL, "m-1", "k", time.Second), nil, "reason", question)
	if !ok || answer.Value != "DUPLICATE" {
		t.Errorf("confident proposal = %+v, %v", answer, ok)
	}
}
