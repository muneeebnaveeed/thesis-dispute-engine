package domain

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

func TestEveryReasonHasAQuestionSetWithUniqueIDs(t *testing.T) {
	for _, r := range AllReasons() {
		qs := QuestionSet(r)
		if len(qs) == 0 {
			t.Errorf("%s: no questions", r)
		}
		seen := map[string]bool{}
		for _, q := range qs {
			if seen[q.ID] || q.ID == "" || q.Text == "" || q.Type == "" {
				t.Errorf("%s: bad question %+v", r, q)
			}
			seen[q.ID] = true
		}
	}
	if r, err := ParseReason(""); err != nil || r != ReasonUnauthorised {
		t.Errorf("empty reason = %s %v", r, err)
	}
	if _, err := ParseReason("VIBES"); !errors.Is(err, ErrUnknownReason) {
		t.Errorf("unknown reason: %v", err)
	}
}

func TestValidateAnswersReportsEveryProblemPerField(t *testing.T) {
	qs := QuestionSet(ReasonUnauthorised)
	err := ValidateAnswers(qs, map[string]string{
		"recognise_merchant": "maybe", "noticed_on": "yesterday", "card_in_possession": "yes",
		"shared_credentials": "no", "police_report": "no", "favourite_colour": "blue",
	})
	if !errors.Is(err, ErrInvalidAnswers) || errs.KindOf(err) != errs.Unprocessable {
		t.Fatalf("err = %v", err)
	}
	got := map[string]string{}
	for _, f := range errs.FieldsOf(err) {
		got[f.Field] = f.Message
	}
	want := []string{
		"body.payload.answers.recognise_merchant",      // not yes/no
		"body.payload.answers.noticed_on",              // not a date
		"body.payload.answers.prior_disputes_merchant", // required, missing
		"body.payload.answers.favourite_colour",        // not asked
	}
	for _, w := range want {
		if got[w] == "" {
			t.Errorf("missing field error for %s; got %v", w, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("field errors = %v", got)
	}
	// Optional questions may be left out; a complete set passes.
	ok := map[string]string{"recognise_merchant": "no", "card_in_possession": "yes", "shared_credentials": "no",
		"prior_disputes_merchant": "no", "noticed_on": "2026-09-20", "police_report": "no"}
	if err := ValidateAnswers(qs, ok); err != nil {
		t.Errorf("complete answers: %v", err)
	}
	if err := ValidateAnswers(QuestionSet(ReasonAmountDiffers), map[string]string{"amount_agreed": "-5", "receipt_available": "yes", "details": "x"}); err == nil {
		t.Error("negative amount accepted")
	}
}

func TestInconsistencies(t *testing.T) {
	flags := Inconsistencies(ReasonUnauthorised, map[string]string{"recognise_merchant": "yes", "card_in_possession": "yes", "shared_credentials": "no"})
	if len(flags) != 1 {
		t.Errorf("flags = %v", flags)
	}
	if n := len(Inconsistencies(ReasonUnauthorised, map[string]string{"recognise_merchant": "no", "card_in_possession": "no", "shared_credentials": "yes"})); n != 0 {
		t.Errorf("clean answers flagged %d", n)
	}
	if n := len(Inconsistencies(ReasonDuplicate, map[string]string{"same_merchant": "no"})); n != 1 {
		t.Errorf("duplicate across merchants flagged %d", n)
	}
}

func TestLoadQuestionSetsRefusesBadFiles(t *testing.T) {
	good := `{"reason":"UNAUTHORISED","questions":[{"id":"a","text":"A?","type":"YES_NO","required":true}]}`
	full := func(override map[string]string) fstest.MapFS {
		m := fstest.MapFS{}
		for _, r := range AllReasons() {
			body := strings.Replace(good, "UNAUTHORISED", string(r), 1)
			if o, ok := override[string(r)]; ok {
				body = o
			}
			m["questionnaires/"+string(r)+".json"] = &fstest.MapFile{Data: []byte(body)}
		}
		return m
	}
	if _, err := LoadQuestionSets(full(nil)); err != nil {
		t.Fatalf("well-formed set: %v", err)
	}
	cases := map[string]fstest.MapFS{
		"missing reason":   func() fstest.MapFS { m := full(nil); delete(m, "questionnaires/DUPLICATE.json"); return m }(),
		"unknown reason":   full(map[string]string{"DUPLICATE": `{"reason":"VIBES","questions":[{"id":"a","text":"A?","type":"YES_NO","required":true}]}`}),
		"repeated id":      full(map[string]string{"DUPLICATE": `{"reason":"DUPLICATE","questions":[{"id":"a","text":"A?","type":"YES_NO","required":true},{"id":"a","text":"B?","type":"TEXT","required":false}]}`}),
		"unknown type":     full(map[string]string{"DUPLICATE": `{"reason":"DUPLICATE","questions":[{"id":"a","text":"A?","type":"COLOUR","required":true}]}`}),
		"nothing required": full(map[string]string{"DUPLICATE": `{"reason":"DUPLICATE","questions":[{"id":"a","text":"A?","type":"TEXT","required":false}]}`}),
		"not json":         full(map[string]string{"DUPLICATE": `{"reason":`}),
	}
	for name, fsys := range cases {
		if _, err := LoadQuestionSets(fsys); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
