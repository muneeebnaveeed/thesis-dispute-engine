package domain

import (
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// Reason is why the customer disputes the transaction; it selects the questionnaire (docs/adr/0016).
type Reason string

// Dispute reasons.
const (
	ReasonUnauthorised  Reason = "UNAUTHORISED"
	ReasonNotReceived   Reason = "NOT_RECEIVED"
	ReasonDuplicate     Reason = "DUPLICATE"
	ReasonAmountDiffers Reason = "AMOUNT_DIFFERS"
)

// AllReasons lists every reason.
func AllReasons() []Reason {
	return []Reason{ReasonUnauthorised, ReasonNotReceived, ReasonDuplicate, ReasonAmountDiffers}
}

// AnswerType is how an answer is typed and validated; every answer travels as a string.
type AnswerType string

// Answer types.
const (
	AnswerYesNo  AnswerType = "YES_NO" // "yes" or "no"
	AnswerDate   AnswerType = "DATE"   // 2006-01-02
	AnswerText   AnswerType = "TEXT"   // free text, at most 2000 characters
	AnswerAmount AnswerType = "AMOUNT" // decimal string
)

// Question is one item on a questionnaire; the set is snapshotted onto the dispute when sent, so a later
// change to the questions never orphans an answer.
type Question struct {
	ID       string     `json:"id"`
	Text     string     `json:"text"`
	Type     AnswerType `json:"type"`
	Required bool       `json:"required"`
}

// Errors for reasons and answers.
var (
	ErrUnknownReason  = errs.New(errs.Invalid, "unknown-reason", "unknown dispute reason")
	ErrInvalidAnswers = errs.New(errs.Unprocessable, "invalid-answers", "the questionnaire answers are incomplete or malformed")
)

var questionSets = map[Reason][]Question{
	ReasonUnauthorised: {
		{ID: "recognise_merchant", Text: "Do you recognise the merchant?", Type: AnswerYesNo, Required: true},
		{ID: "card_in_possession", Text: "Was your card in your possession at the time?", Type: AnswerYesNo, Required: true},
		{ID: "shared_credentials", Text: "Has anyone else had access to your card or its details?", Type: AnswerYesNo, Required: true},
		{ID: "prior_disputes_merchant", Text: "Have you disputed a transaction with this merchant before?", Type: AnswerYesNo, Required: true},
		{ID: "noticed_on", Text: "When did you notice the transaction?", Type: AnswerDate, Required: true},
		{ID: "police_report", Text: "Have you reported the card lost or stolen, or filed a police report?", Type: AnswerYesNo, Required: true},
		{ID: "details", Text: "Anything else we should know?", Type: AnswerText, Required: false},
	},
	ReasonNotReceived: {
		{ID: "expected_on", Text: "When were the goods or services due?", Type: AnswerDate, Required: true},
		{ID: "contacted_merchant", Text: "Have you contacted the merchant?", Type: AnswerYesNo, Required: true},
		{ID: "merchant_response", Text: "What did the merchant say?", Type: AnswerText, Required: false},
		{ID: "partial_delivery", Text: "Did you receive part of the order?", Type: AnswerYesNo, Required: true},
	},
	ReasonDuplicate: {
		{ID: "original_on", Text: "When was the transaction you did authorise?", Type: AnswerDate, Required: true},
		{ID: "same_merchant", Text: "Was it with the same merchant?", Type: AnswerYesNo, Required: true},
		{ID: "details", Text: "Anything else we should know?", Type: AnswerText, Required: false},
	},
	ReasonAmountDiffers: {
		{ID: "amount_agreed", Text: "What amount did you agree to?", Type: AnswerAmount, Required: true},
		{ID: "receipt_available", Text: "Do you have a receipt showing that amount?", Type: AnswerYesNo, Required: true},
		{ID: "details", Text: "Describe the difference.", Type: AnswerText, Required: true},
	},
}

// ParseReason accepts a reason string, defaulting to UNAUTHORISED when empty.
func ParseReason(raw string) (Reason, error) {
	if raw == "" {
		return ReasonUnauthorised, nil
	}
	if _, ok := questionSets[Reason(raw)]; !ok {
		return "", errs.Wrap(ErrUnknownReason, "%q", raw)
	}
	return Reason(raw), nil
}

// QuestionSet is the questionnaire for a reason, in the order it is asked.
func QuestionSet(reason Reason) []Question {
	return append([]Question(nil), questionSets[reason]...)
}

// ValidateAnswers checks answers against the questions they were asked: required ones present, every value
// well-typed, nothing answered that was not asked. Every problem is reported at once, per field.
func ValidateAnswers(questions []Question, answers map[string]string) error {
	var fields []errs.FieldError
	asked := map[string]Question{}
	for _, q := range questions {
		asked[q.ID] = q
		v, ok := answers[q.ID]
		if !ok || v == "" {
			if q.Required {
				fields = append(fields, errs.FieldError{Field: "body.payload.answers." + q.ID, Message: "an answer is required"})
			}
			continue
		}
		if msg := checkAnswer(q.Type, v); msg != "" {
			fields = append(fields, errs.FieldError{Field: "body.payload.answers." + q.ID, Message: msg})
		}
	}
	for id := range answers {
		if _, ok := asked[id]; !ok {
			fields = append(fields, errs.FieldError{Field: "body.payload.answers." + id, Message: "this question was not asked"})
		}
	}
	if len(fields) == 0 {
		return nil
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Field < fields[j].Field })
	return ErrInvalidAnswers.WithFields(fields...)
}

func checkAnswer(t AnswerType, v string) string {
	switch t {
	case AnswerYesNo:
		if v != "yes" && v != "no" {
			return `must be "yes" or "no"`
		}
	case AnswerDate:
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "must be a date as YYYY-MM-DD"
		}
	case AnswerAmount:
		if d, err := decimal.NewFromString(v); err != nil || d.IsNegative() {
			return "must be a non-negative decimal amount"
		}
	case AnswerText:
		if len(v) > 2000 {
			return fmt.Sprintf("at most 2000 characters (%d given)", len(v))
		}
	}
	return ""
}

// Inconsistencies are contradictions between answers that an investigator would want flagged; the fraud
// scoring uses them as a signal. Each is a sentence an analyst can act on.
func Inconsistencies(reason Reason, answers map[string]string) []string {
	var out []string
	switch reason {
	case ReasonUnauthorised:
		if answers["recognise_merchant"] == "yes" && answers["card_in_possession"] == "yes" && answers["shared_credentials"] == "no" {
			out = append(out, "the customer recognises the merchant, held the card and shared it with nobody, yet claims the payment was unauthorised")
		}
		if answers["police_report"] == "yes" && answers["card_in_possession"] == "yes" {
			out = append(out, "a card reported lost or stolen was said to be in the customer's possession")
		}
	case ReasonNotReceived:
		if answers["contacted_merchant"] == "no" && answers["merchant_response"] != "" {
			out = append(out, "a merchant response was given although the merchant was not contacted")
		}
	case ReasonDuplicate:
		if answers["same_merchant"] == "no" {
			out = append(out, "a duplicate is claimed against a different merchant")
		}
	case ReasonAmountDiffers:
	}
	return out
}
