package notice

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

var testFacts = FactValues{Customer: "Kovács Anna", Bank: "OTP Bank", Amount: "1899.00 EUR", Merchant: "MediaMarkt",
	Dispute: "01a0c601", Today: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)}

func TestEveryManualKindHasATemplate(t *testing.T) {
	for _, k := range domain.ManualNoticeKinds() {
		if _, ok := TemplateFor(k); !ok {
			t.Errorf("%s has no template", k)
		}
	}
	if n := len(Templates()); n != len(domain.ManualNoticeKinds()) {
		t.Errorf("%d templates", n)
	}
}

func TestRequestForInformationFillsListsAndDueDate(t *testing.T) {
	tpl, _ := TemplateFor(domain.NoticeRequestForInformation)
	values, err := Validate(tpl, map[string]string{"items": "receipt,merchant_contact", "other": "the serial number\n", "days": "10"})
	if err != nil {
		t.Fatal(err)
	}
	d := Fill(tpl, testFacts, values)
	text := d.Text()
	for _, want := range []string{"1899.00 EUR payment to MediaMarkt", "- a copy of the receipt", "- any correspondence with the merchant",
		"- the serial number", "reply by 2 October 2026 (10 days from today)", "Dispute reference 01a0c601", "Dear Kovács Anna,"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if d.Bank != "OTP Bank" || d.Subject == "" {
		t.Errorf("document = %+v", d)
	}
	// The optional paragraph disappears when its field is empty.
	values, _ = Validate(tpl, map[string]string{"items": "receipt", "days": "5"})
	if got := len(Fill(tpl, testFacts, values).Paragraphs); got != 4 {
		t.Errorf("paragraphs without 'other' = %d, want 4", got)
	}
}

func TestSectionsAndSelectText(t *testing.T) {
	tpl, _ := TemplateFor(domain.NoticeStatusUpdate)
	values, err := Validate(tpl, map[string]string{"stage": "merchant", "nextBy": "2026-10-01"})
	if err != nil {
		t.Fatal(err)
	}
	text := Fill(tpl, testFacts, values).Text()
	if !strings.Contains(text, "waiting for their response") || !strings.Contains(text, "write to you again by 1 October 2026") {
		t.Errorf("status update:\n%s", text)
	}
	values, _ = Validate(tpl, map[string]string{"stage": "final"})
	if strings.Contains(Fill(tpl, testFacts, values).Text(), "write to you again") {
		t.Error("section rendered without its field")
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	tpl, _ := TemplateFor(domain.NoticeRequestForInformation)
	_, err := Validate(tpl, map[string]string{"items": "receipt,unicorn", "days": "90", "colour": "blue"})
	if !errors.Is(err, ErrInvalidFields) {
		t.Fatalf("err = %v", err)
	}
	got := map[string]string{}
	for _, f := range errs.FieldsOf(err) {
		got[f.Field] = f.Message
	}
	for _, want := range []string{"body.fields.items", "body.fields.days", "body.fields.colour"} {
		if got[want] == "" {
			t.Errorf("no error for %s: %v", want, got)
		}
	}
	if _, err := Validate(tpl, map[string]string{}); err == nil {
		t.Error("required fields missing but accepted")
	}
	custom, _ := TemplateFor(domain.NoticeCustom)
	if _, err := Validate(custom, map[string]string{"subject": strings.Repeat("x", 201), "body": "hi"}); err == nil {
		t.Error("over-long subject accepted")
	}
}

func TestPlaceholdersKeepFieldsForTheBrowser(t *testing.T) {
	tpl, _ := TemplateFor(domain.NoticeRequestForInformation)
	p := Placeholders(tpl, testFacts)
	joined := strings.Join(p.Paragraphs, " ")
	if !strings.Contains(joined, "1899.00 EUR payment to MediaMarkt") || !strings.Contains(joined, "{{items}}") || !strings.Contains(joined, "{{dueDate}}") {
		t.Errorf("placeholders = %v", p.Paragraphs)
	}
	su, _ := TemplateFor(domain.NoticeStatusUpdate)
	if q := Placeholders(su, testFacts); !strings.Contains(strings.Join(q.Paragraphs, " "), "{{#nextBy}}") {
		t.Error("section markers lost")
	}
}

func TestLoadTemplatesRefusesBadFiles(t *testing.T) {
	bad := map[string]string{
		"not manual":     `{"kind":"REFUND","label":"x","subject":"s","paragraphs":["p"],"fields":[]}`,
		"unknown holder": `{"kind":"CUSTOM","label":"x","subject":"s","paragraphs":["{{nothing}}"],"fields":[]}`,
		"select no opts": `{"kind":"CUSTOM","label":"x","subject":"s","paragraphs":["p"],"fields":[{"id":"a","label":"A","type":"SELECT"}]}`,
		"shadowed fact":  `{"kind":"CUSTOM","label":"x","subject":"s","paragraphs":["p"],"fields":[{"id":"amount","label":"A","type":"TEXT"}]}`,
		"comma in key":   `{"kind":"CUSTOM","label":"x","subject":"s","paragraphs":["p"],"fields":[{"id":"a","label":"A","type":"SELECT","options":[{"key":"a,b","label":"l","text":"t"}]}]}`,
	}
	for name, body := range bad {
		if _, err := LoadTemplates(fstest.MapFS{"templates/x.json": &fstest.MapFile{Data: []byte(body)}}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestApplyOverrideChangesWordsNotForm(t *testing.T) {
	base, _ := TemplateFor(domain.NoticeRequestForInformation)
	got, err := Apply(base, Override{
		Label:       "Kérjük, küldjön dokumentumokat",
		Subject:     "A little more about your {{merchant}} payment",
		Paragraphs:  []string{"Kedves ügyfelünk, we need:", "{{items}}", "By {{dueDate}}.", "Ref {{dispute}}."},
		OptionTexts: map[string]string{"items.receipt": "a copy of the receipt (számla)"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Label == base.Label || got.Fields[0].Options[0].Text != "a copy of the receipt (számla)" || base.Fields[0].Options[0].Text == got.Fields[0].Options[0].Text {
		t.Errorf("override not applied or base mutated: %+v", got.Fields[0].Options[0])
	}
	if len(got.Fields) != len(base.Fields) || got.Fields[2].ID != "days" {
		t.Errorf("form changed: %+v", got.Fields)
	}
	bad := []Override{
		{Paragraphs: []string{"{{nothing}}"}},
		{OptionTexts: map[string]string{"items.unicorn": "x"}},
		{OptionTexts: map[string]string{"days.1": "x"}},
		{OptionTexts: map[string]string{"items.receipt": " "}},
	}
	for i, o := range bad {
		if _, err := Apply(base, o); !errors.Is(err, ErrInvalidOverride) {
			t.Errorf("bad override %d accepted: %v", i, err)
		}
	}
}
