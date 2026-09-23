package domain

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
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

//go:embed questionnaires/*.json
var questionnaireFiles embed.FS

// questionSets are loaded once from the embedded files: the questions are content a compliance team edits, and
// the same file shape is what a per-tenant or per-language set would take later (docs/adr/0016).
var questionSets = mustLoadQuestionSets()

type questionnaireFile struct {
	Reason    Reason     `json:"reason"`
	Questions []Question `json:"questions"`
}

func mustLoadQuestionSets() map[Reason][]Question {
	sets, err := LoadQuestionSets(questionnaireFiles)
	if err != nil {
		panic(err)
	}
	return sets
}

// LoadQuestionSets reads every questionnaires/*.json in fsys and checks each: a known reason, at least one
// required question, unique ids, known answer types. Every reason must have exactly one file.
func LoadQuestionSets(fsys fs.FS) (map[Reason][]Question, error) {
	files, err := fs.Glob(fsys, "questionnaires/*.json")
	if err != nil {
		return nil, err
	}
	sets := make(map[Reason][]Question, len(files))
	for _, name := range files {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var f questionnaireFile
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("questionnaire %s: %w", name, err)
		}
		if err := checkQuestionSet(f); err != nil {
			return nil, fmt.Errorf("questionnaire %s: %w", name, err)
		}
		if _, dup := sets[f.Reason]; dup {
			return nil, fmt.Errorf("questionnaire %s: reason %s defined twice", name, f.Reason)
		}
		sets[f.Reason] = f.Questions
	}
	for _, r := range AllReasons() {
		if _, ok := sets[r]; !ok {
			return nil, fmt.Errorf("questionnaire for %s missing", r)
		}
	}
	return sets, nil
}

func checkQuestionSet(f questionnaireFile) error {
	if !slices.Contains(AllReasons(), f.Reason) {
		return fmt.Errorf("unknown reason %q", f.Reason)
	}
	seen := map[string]bool{}
	required := false
	for _, q := range f.Questions {
		switch {
		case q.ID == "" || q.Text == "":
			return fmt.Errorf("question %+v needs an id and text", q)
		case seen[q.ID]:
			return fmt.Errorf("question id %q repeated", q.ID)
		case !slices.Contains([]AnswerType{AnswerYesNo, AnswerDate, AnswerText, AnswerAmount}, q.Type):
			return fmt.Errorf("question %q: unknown type %q", q.ID, q.Type)
		}
		seen[q.ID] = true
		required = required || q.Required
	}
	if !required {
		return fmt.Errorf("no required question")
	}
	return nil
}

// ReasonMeaning says what a reason covers, in a customer's words rather than the code's. It is the option
// list a typed-decision model is given, so it belongs beside the reasons themselves.
func ReasonMeaning(r Reason) string {
	switch r {
	case ReasonUnauthorised:
		return "the customer says they did not make or authorise the payment at all"
	case ReasonNotReceived:
		return "the customer paid but the goods or services never arrived, or arrived only in part"
	case ReasonDuplicate:
		return "the same purchase was charged more than once"
	case ReasonAmountDiffers:
		return "the customer made the purchase but was charged a different amount than agreed"
	}
	return ""
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
				fields = append(fields, errs.FieldError{Field: "body.payload.answers." + q.ID, Message: missingAnswer(q.Type)})
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

// Messages are shown to the analyst under the input, so they say what to do, not what rule was broken.
func missingAnswer(t AnswerType) string {
	switch t {
	case AnswerYesNo:
		return "Please choose yes or no"
	case AnswerDate:
		return "Please select a date"
	case AnswerAmount:
		return "Please enter an amount"
	default:
		return "Please enter an answer"
	}
}

func checkAnswer(t AnswerType, v string) string {
	switch t {
	case AnswerYesNo:
		if v != "yes" && v != "no" {
			return "Please choose yes or no"
		}
	case AnswerDate:
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "Please enter a date as YYYY-MM-DD"
		}
	case AnswerAmount:
		if d, err := decimal.NewFromString(v); err != nil || d.IsNegative() {
			return "Please enter an amount of zero or more, such as 12.50"
		}
	case AnswerText:
		if len(v) > 2000 {
			return fmt.Sprintf("Please keep the answer under 2000 characters (%d given)", len(v))
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
