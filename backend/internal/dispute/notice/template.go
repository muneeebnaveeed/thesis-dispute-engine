package notice

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// FieldType is how an analyst's input is collected and validated.
type FieldType string

// Field types.
const (
	FieldText        FieldType = "TEXT"
	FieldTextarea    FieldType = "TEXTAREA" // lines become a list when the paragraph is only the field
	FieldNumber      FieldType = "NUMBER"
	FieldDate        FieldType = "DATE"        // YYYY-MM-DD
	FieldSelect      FieldType = "SELECT"      // one option key; its text goes into the message
	FieldMultiselect FieldType = "MULTISELECT" // comma-separated option keys; their texts become a list
)

// Option is one choice of a select field: the key the form sends, the label the analyst sees, the text the
// customer reads.
type Option struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

// Field is one input on a template's form.
type Field struct {
	ID       string    `json:"id"`
	Label    string    `json:"label"`
	Type     FieldType `json:"type"`
	Required bool      `json:"required"`
	Options  []Option  `json:"options,omitempty"`
	Default  string    `json:"default,omitempty"`
	Min      *int      `json:"min,omitempty"`
	Max      *int      `json:"max,omitempty"`
}

// Template is an analyst-initiated email: the form and the words. Subject and paragraphs carry {{field}} and
// {{fact}} placeholders and {{#field}}...{{/field}} sections that appear only when the field has a value.
type Template struct {
	Kind        domain.NoticeKind `json:"kind"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Letter      bool              `json:"letter"` // also a letter, where the regime's notices must be written
	Fields      []Field           `json:"fields"`
	Subject     string            `json:"subject"`
	Paragraphs  []string          `json:"paragraphs"`
}

//go:embed templates/*.json
var templateFiles embed.FS

var templates = mustLoadTemplates()

// Facts are the values the engine fills in itself; the analyst never types them.
var factNames = []string{"customer", "bank", "amount", "merchant", "dispute", "today", "dueDate"}

// Templates lists every analyst-initiated template, in a stable order.
func Templates() []Template {
	out := make([]Template, 0, len(templates))
	for _, t := range templates {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

// TemplateFor returns the template of a kind.
func TemplateFor(kind domain.NoticeKind) (Template, bool) {
	t, ok := templates[kind]
	return t, ok
}

func mustLoadTemplates() map[domain.NoticeKind]Template {
	ts, err := LoadTemplates(templateFiles)
	if err != nil {
		panic(err)
	}
	return ts
}

var placeholder = regexp.MustCompile(`\{\{[#/]?([A-Za-z][A-Za-z0-9]*)\}\}`)

// LoadTemplates reads templates/*.json and checks each: a manual kind, unique field ids, options on selects,
// every placeholder a field or a fact.
func LoadTemplates(fsys fs.FS) (map[domain.NoticeKind]Template, error) {
	files, err := fs.Glob(fsys, "templates/*.json")
	if err != nil {
		return nil, err
	}
	out := make(map[domain.NoticeKind]Template, len(files))
	for _, name := range files {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var t Template
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("template %s: %w", name, err)
		}
		if err := checkTemplate(t); err != nil {
			return nil, fmt.Errorf("template %s: %w", name, err)
		}
		if _, dup := out[t.Kind]; dup {
			return nil, fmt.Errorf("template %s: kind %s defined twice", name, t.Kind)
		}
		out[t.Kind] = t
	}
	return out, nil
}

func checkTemplate(t Template) error {
	if !slices.Contains(domain.ManualNoticeKinds(), t.Kind) {
		return fmt.Errorf("kind %q is not a manual notice kind", t.Kind)
	}
	if t.Label == "" || t.Subject == "" || len(t.Paragraphs) == 0 {
		return fmt.Errorf("needs a label, a subject and paragraphs")
	}
	ids := map[string]bool{}
	for _, f := range t.Fields {
		switch {
		case f.ID == "" || f.Label == "":
			return fmt.Errorf("field %+v needs an id and a label", f)
		case ids[f.ID] || slices.Contains(factNames, f.ID):
			return fmt.Errorf("field id %q repeated or shadows a fact", f.ID)
		case !slices.Contains([]FieldType{FieldText, FieldTextarea, FieldNumber, FieldDate, FieldSelect, FieldMultiselect}, f.Type):
			return fmt.Errorf("field %q: unknown type %q", f.ID, f.Type)
		case (f.Type == FieldSelect || f.Type == FieldMultiselect) && len(f.Options) == 0:
			return fmt.Errorf("field %q: a select needs options", f.ID)
		}
		for _, o := range f.Options {
			if o.Key == "" || o.Label == "" || o.Text == "" || strings.Contains(o.Key, ",") {
				return fmt.Errorf("field %q: option %+v needs key, label and text, and no comma in the key", f.ID, o)
			}
		}
		ids[f.ID] = true
	}
	for _, s := range append([]string{t.Subject}, t.Paragraphs...) {
		for _, m := range placeholder.FindAllStringSubmatch(s, -1) {
			if !ids[m[1]] && !slices.Contains(factNames, m[1]) {
				return fmt.Errorf("placeholder %q is neither a field nor a fact", m[1])
			}
		}
	}
	return nil
}

// ErrInvalidFields means the analyst's inputs do not satisfy the template's form.
var ErrInvalidFields = errs.New(errs.Unprocessable, "invalid-fields", "the email form is incomplete or malformed")

// ErrUnknownTemplate means the kind is not one an analyst can send.
var ErrUnknownTemplate = errs.New(errs.Invalid, "unknown-template", "unknown email template")

// FactValues are the engine-known values as the customer should read them.
type FactValues struct {
	Customer string
	Bank     string
	Amount   string // "125.40 EUR"
	Merchant string
	Dispute  string
	Today    time.Time
}

// Validate checks inputs against the template's fields and returns the display value of each: option texts for
// selects, formatted dates, trimmed text. Every problem is reported at once, per field.
func Validate(t Template, inputs map[string]string) (map[string]string, error) {
	var fields []errs.FieldError
	refuse := func(id, msg string) {
		fields = append(fields, errs.FieldError{Field: "body.fields." + id, Message: msg})
	}
	values := map[string]string{}
	known := map[string]bool{}
	for _, f := range t.Fields {
		known[f.ID] = true
		raw := strings.TrimSpace(inputs[f.ID])
		if raw == "" {
			raw = f.Default
		}
		if raw == "" {
			if f.Required {
				refuse(f.ID, "required")
			}
			continue
		}
		v, msg := displayValue(f, raw)
		if msg != "" {
			refuse(f.ID, msg)
			continue
		}
		values[f.ID] = v
	}
	for id := range inputs {
		if !known[id] {
			refuse(id, "this template has no such field")
		}
	}
	if len(fields) > 0 {
		sort.Slice(fields, func(i, j int) bool { return fields[i].Field < fields[j].Field })
		return nil, ErrInvalidFields.WithFields(fields...)
	}
	return values, nil
}

func displayValue(f Field, raw string) (string, string) {
	switch f.Type {
	case FieldText:
		if len(raw) > 200 {
			return "", "at most 200 characters"
		}
		return raw, ""
	case FieldTextarea:
		if len(raw) > 4000 {
			return "", "at most 4000 characters"
		}
		return raw, ""
	case FieldNumber:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return "", "must be a whole number"
		}
		if (f.Min != nil && n < *f.Min) || (f.Max != nil && n > *f.Max) {
			return "", fmt.Sprintf("must be between %d and %d", deref(f.Min, 0), deref(f.Max, n))
		}
		return strconv.Itoa(n), ""
	case FieldDate:
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return "", "must be a date as YYYY-MM-DD"
		}
		return d.Format("2 January 2006"), ""
	case FieldSelect:
		for _, o := range f.Options {
			if o.Key == raw {
				return o.Text, ""
			}
		}
		return "", "not one of the options"
	case FieldMultiselect:
		var texts []string
		for _, key := range strings.Split(raw, ",") {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			i := slices.IndexFunc(f.Options, func(o Option) bool { return o.Key == key })
			if i < 0 {
				return "", fmt.Sprintf("%q is not one of the options", key)
			}
			texts = append(texts, f.Options[i].Text)
		}
		if len(texts) == 0 {
			return "", "choose at least one"
		}
		return strings.Join(texts, "\n"), ""
	}
	return raw, ""
}

func deref(p *int, d int) int {
	if p == nil {
		return d
	}
	return *p
}

// Fill composes the final document: facts and validated field values substituted, sections resolved, empty
// paragraphs dropped, list-valued fields rendered as bullet lines. dueDate is today plus the "days" field.
func Fill(t Template, facts FactValues, values map[string]string) Document {
	all := factMap(facts)
	for k, v := range values {
		all[k] = v
	}
	if days, err := strconv.Atoi(values["days"]); err == nil {
		all["dueDate"] = facts.Today.AddDate(0, 0, days).Format("2 January 2006")
	}
	d := Document{Bank: facts.Bank, Subject: substitute(t.Subject, all, nil), Greeting: "Dear " + facts.Customer + ",",
		Closing: "Yours sincerely,\n" + facts.Bank + " disputes team"}
	listy := map[string]bool{}
	for _, f := range t.Fields {
		listy[f.ID] = f.Type == FieldMultiselect || f.Type == FieldTextarea
	}
	for _, p := range t.Paragraphs {
		out := strings.TrimSpace(substitute(p, all, listy))
		if out != "" {
			d.Paragraphs = append(d.Paragraphs, out)
		}
	}
	return d
}

// Placeholders renders a template for the live preview: facts substituted, fields left as {{id}} so the browser
// can fill them as the analyst types.
func Placeholders(t Template, facts FactValues) Template {
	all := factMap(facts)
	out := t
	out.Subject = substituteFacts(t.Subject, all)
	out.Paragraphs = make([]string, len(t.Paragraphs))
	for i, p := range t.Paragraphs {
		out.Paragraphs[i] = substituteFacts(p, all)
	}
	return out
}

func factMap(f FactValues) map[string]string {
	return map[string]string{"customer": f.Customer, "bank": f.Bank, "amount": f.Amount, "merchant": f.Merchant,
		"dispute": f.Dispute, "today": f.Today.Format("2 January 2006"), "dueDate": ""}
}

var section = regexp.MustCompile(`\{\{#([A-Za-z][A-Za-z0-9]*)\}\}(.*?)\{\{/([A-Za-z][A-Za-z0-9]*)\}\}`)

// substitute resolves sections then placeholders; a paragraph that is only a list-valued field becomes bullets.
func substitute(s string, values map[string]string, listy map[string]bool) string {
	s = section.ReplaceAllStringFunc(s, func(m string) string {
		sm := section.FindStringSubmatch(m)
		if values[sm[1]] == "" {
			return ""
		}
		return sm[2]
	})
	if m := placeholder.FindStringSubmatch(strings.TrimSpace(s)); len(m) == 2 && strings.TrimSpace(s) == m[0] && listy[m[1]] {
		var lines []string
		for _, line := range strings.Split(values[m[1]], "\n") {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, "- "+strings.TrimPrefix(line, "- "))
			}
		}
		return strings.Join(lines, "\n")
	}
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		return values[placeholder.FindStringSubmatch(m)[1]]
	})
}

// substituteFacts fills only the facts and keeps every field placeholder and section for the browser.
func substituteFacts(s string, facts map[string]string) string {
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		sm := placeholder.FindStringSubmatch(m)
		if strings.HasPrefix(m, "{{#") || strings.HasPrefix(m, "{{/") || sm[1] == "dueDate" {
			return m
		}
		if v, ok := facts[sm[1]]; ok {
			return v
		}
		return m
	})
}
