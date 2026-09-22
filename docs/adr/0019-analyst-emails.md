# 0019: Analyst-composed emails: templates as data, one substitution language shared by preview and send

**Status:** accepted, 2026-09-22; extends ADR 0017

## Context

ADR 0017 gave the engine the notices it must send on its own. Analysts also write to customers on
their own initiative: to ask for documents, to report where the case stands, to confirm what
arrived, or to say something the templates do not cover. A comparable production system (the
standardized email pipeline in casap) keeps a catalogue of template kinds per organisation, each
with a form definition and option texts, returns the message HTML with known facts filled and the
analyst's fields left as `{{placeholders}}`, previews live in the browser by substitution, and
renders the final message on the server on send. That split is worth copying: the browser never
needs the template engine's rules for facts, and the server never trusts the browser's rendering.

## Decision

- Templates are data files, one per kind under `notice/templates/`, embedded and validated at
  start like the questionnaires: a label, the form (typed fields; select options carry the key the
  form sends, the label the analyst sees and the text the customer reads), a subject and
  paragraphs in a small language: `{{field}}` and `{{fact}}` placeholders, `{{#field}}...{{/field}}`
  sections present only when the field has a value, a paragraph that is only a list-valued field
  rendered as bullet lines, empty paragraphs dropped. Four kinds to start: request for information,
  status update, documents received, custom message. Facts are the engine's (customer, bank,
  amount, merchant, reference, today, a due date derived from a days field) and are never typed.
- `GET /disputes/{id}/email-templates` returns the catalogue with facts substituted and fields
  left as placeholders; the browser implements the same substitution (`frontend/src/email/render.ts`)
  for the live preview and marks unfilled fields inside the message. `POST /disputes/{id}/notices`
  validates the fields against the template (one error per field, 422 `invalid-fields`), composes
  the document server-side with `notice.Fill`, and stores it through the same outbox as the
  automatic notices, with the analyst recorded as the actor and a letter added where the template
  is a letter and the regime requires writing. Tenant keys cannot compose: machines do not write to
  customers.
- Attachments are uploaded first and claimed by the email that sends them, the two-step shape of
  comparable tools: a draft belongs to the dispute, a claim binds it to one notice for good, and
  drafts nobody claimed are swept after a day. Bytes live in the database (5 MB per file, three per
  email, PDF, PNG or JPEG) so the API stays the only stateful client; the dispatcher reads them
  across tenants through an owner-defined function and sends `multipart/mixed`; a letter lists them
  as enclosures. Machine credentials cannot upload.
- A resend is a new notice chained to the original (`resend_of`), with the original's words, the
  customer's current address and the analyst as author; only an email that was sent can be resent,
  because an email still in the outbox is the engine's to retry. A retry and a resend are
  therefore different records, as they should be for an auditor.
- The workbench gets a communications panel per dispute with two tabs, Create Email (template,
  form, live preview side by side) and Sent Emails (every notice, automatic and analyst-written,
  with the selected one shown as composed).

## Consequences

- Easier: a new email is a JSON file; preview and send cannot disagree on wording because they
  render the same paragraphs from the same values; every analyst email is an auditable notice row
  with its author; the two-tab shape is what analysts already know from comparable tools.
- Harder: the substitution language is deliberately tiny (no loops, no conditionals beyond
  presence), and both halves must stay in step, which the unit tests on each side pin; templates
  are one language and shared by every tenant; attachments in the database cap the sensible size
  and would move to object storage under real volume; a sent email cannot be recalled, only
  followed by another or resent.
