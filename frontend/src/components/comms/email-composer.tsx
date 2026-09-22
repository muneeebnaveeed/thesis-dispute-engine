import { useStore } from '@tanstack/react-form'
import { useMemo } from 'react'

import { FieldError } from '#/components/ui/field'
import { submitTo, submitting, useAppForm, type BoundFieldComponents } from '#/forms/app-form'
import {
  emailLineSegments,
  renderEmailPreview,
  type EmailTemplate,
  type TemplateField,
} from '#/lib/email/render'

type EmailFacts = { customer: string; bank: string; today: string }
export type AttachmentDraft = { id: string; filename: string; size: number }
const NO_ATTACHMENT_DRAFTS: AttachmentDraft[] = []
const MAX_ATTACHMENTS = 3

const blankInputs = (template: EmailTemplate | undefined): Record<string, string> =>
  Object.fromEntries((template?.fields ?? []).map((field) => [field.id, '']))

const filledOnly = (inputs: Record<string, string>) =>
  Object.fromEntries(Object.entries(inputs).filter(([, value]) => value.trim() !== ''))

export const EmailComposer = ({
  templates,
  facts,
  attachmentError,
  busy,
  onSend,
  attachmentDrafts = NO_ATTACHMENT_DRAFTS,
  onUpload,
  onRemoveAttachmentDraft,
}: {
  templates: EmailTemplate[]
  facts: EmailFacts
  attachmentError?: string | undefined
  busy: boolean
  onSend: (templateKind: EmailTemplate['kind'], analystInputs: Record<string, string>) => Promise<unknown>
  attachmentDrafts?: AttachmentDraft[]
  onUpload?: (file: File) => void
  onRemoveAttachmentDraft?: (draftId: string) => void
}) => {
  const composer = useAppForm({
    defaultValues: {
      template: templates[0]?.kind ?? ('CUSTOM' as EmailTemplate['kind']),
      fields: blankInputs(templates[0]),
    },
    onSubmit: ({ value, formApi }) =>
      submitTo(formApi, () => onSend(value.template, filledOnly(value.fields))),
  })
  const { template: selectedKind, fields: analystInputs } = useStore(composer.store, (state) => state.values)
  const selectedTemplate = templates.find((template) => template.kind === selectedKind) ?? templates[0]
  const today = useMemo(() => new Date(`${facts.today}T00:00:00Z`), [facts.today])
  const renderedEmail = useMemo(
    () => (selectedTemplate ? renderEmailPreview(selectedTemplate, analystInputs, today) : null),
    [selectedTemplate, analystInputs, today],
  )
  if (!selectedTemplate || !renderedEmail) {
    return <p className="text-sm text-muted-foreground">No templates are available.</p>
  }

  return (
    <div className="grid gap-8 lg:grid-cols-2">
      <form className="space-y-4 text-sm" onSubmit={submitting(composer)}>
        <composer.AppField
          name="template"
          listeners={{
            // a new template means a new form: its fields are different, the old answers mean nothing
            onChange: ({ value }) =>
              composer.setFieldValue(
                'fields',
                blankInputs(templates.find((template) => template.kind === value)),
              ),
          }}
        >
          {(field) => (
            <field.SelectField
              label="Email template"
              options={templates.map((template) => ({ value: template.kind, label: template.label }))}
              hint={
                <span className="mt-1 block text-xs text-muted-foreground">
                  {selectedTemplate.description}
                  {selectedTemplate.letter &&
                    ' Also goes out as a letter where the regime requires written notices.'}
                </span>
              }
            />
          )}
        </composer.AppField>
        {selectedTemplate.fields.map((templateField) => (
          <composer.AppField key={templateField.id} name={`fields.${templateField.id}`}>
            {(field) => <TemplateFieldInput templateField={templateField} field={field} />}
          </composer.AppField>
        ))}
        {onUpload && (
          <div>
            <label className="block">
              Attachments{' '}
              <span className="text-muted-foreground">(PDF, PNG or JPEG, at most 5 MB each, up to 3)</span>
              <input
                type="file"
                accept="application/pdf,image/png,image/jpeg"
                disabled={busy || attachmentDrafts.length >= MAX_ATTACHMENTS}
                className="mt-1 block text-sm"
                onChange={(event) => {
                  const chosenFile = event.target.files?.[0]
                  if (chosenFile) onUpload(chosenFile)
                  event.target.value = ''
                }}
              />
            </label>
            {attachmentDrafts.length > 0 && (
              <ul className="mt-2 flex flex-wrap gap-2" aria-label="Attached files">
                {attachmentDrafts.map((draft) => (
                  <li
                    key={draft.id}
                    className="flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs"
                  >
                    {draft.filename}{' '}
                    <span className="text-muted-foreground">({Math.ceil(draft.size / 1024)} KB)</span>
                    {onRemoveAttachmentDraft && (
                      <button
                        type="button"
                        aria-label={`Remove ${draft.filename}`}
                        className="ml-1 text-muted-foreground hover:text-foreground"
                        onClick={() => onRemoveAttachmentDraft(draft.id)}
                      >
                        x
                      </button>
                    )}
                  </li>
                ))}
              </ul>
            )}
            <FieldError id="attachments-error" message={attachmentError} />
          </div>
        )}
        <composer.AppForm>
          <composer.SubmitButton busy={busy}>{busy ? 'Sending...' : 'Send email'}</composer.SubmitButton>
        </composer.AppForm>
      </form>

      <section
        aria-label="Preview"
        className="rounded-md border border-border bg-white p-6 font-serif text-[12pt] leading-relaxed"
      >
        <p className="mb-6 text-sm text-muted-foreground">
          {facts.bank}
          <br />
          {today.toLocaleDateString('en-GB', {
            day: 'numeric',
            month: 'long',
            year: 'numeric',
            timeZone: 'UTC',
          })}
        </p>
        <h3 className="mb-4 text-[14pt] font-semibold">
          <EmailLineWithUnfilledMarked renderedLine={renderedEmail.subject} />
        </h3>
        <p className="mb-4">Dear {facts.customer},</p>
        {renderedEmail.paragraphs.map((paragraph, paragraphIndex) => (
          // eslint-disable-next-line react/no-array-index-key -- paragraphs are positional prose
          <p key={paragraphIndex} className="mb-4 whitespace-pre-line">
            <EmailLineWithUnfilledMarked renderedLine={paragraph} />
          </p>
        ))}
        <p className="whitespace-pre-line">
          Yours sincerely,{'\n'}
          {facts.bank} disputes team
        </p>
        {attachmentDrafts.length > 0 && (
          <p className="mt-6 text-sm text-muted-foreground">
            Attached: {attachmentDrafts.map((draft) => draft.filename).join(', ')}
          </p>
        )}
      </section>
    </div>
  )
}

const EmailLineWithUnfilledMarked = ({ renderedLine }: { renderedLine: string }) => (
  <>
    {emailLineSegments(renderedLine).map((segment, segmentIndex) =>
      segment.unfilledPlaceholder ? (
        <mark
          // eslint-disable-next-line react/no-array-index-key -- segments are positional runs of one line
          key={`${segmentIndex}-${segment.text}`}
          className="rounded bg-amber-100 px-1 font-sans text-xs text-amber-900"
          data-missing={segment.text}
        >
          {segment.text}
        </mark>
      ) : (
        segment.text
      ),
    )}
  </>
)

const TemplateFieldInput = ({
  templateField,
  field,
}: {
  templateField: TemplateField
  field: BoundFieldComponents
}) => {
  const label = (
    <>
      {templateField.label}
      {templateField.required && <span className="text-muted-foreground"> (required)</span>}
    </>
  )
  if (templateField.type === 'MULTISELECT')
    return (
      <field.CheckboxGroupField
        label={label}
        options={(templateField.options ?? []).map((option) => ({
          key: option.key,
          label: option.label,
          description: option.text,
        }))}
      />
    )
  if (templateField.type === 'SELECT')
    return (
      <field.SelectField
        label={label}
        placeholder="choose"
        options={(templateField.options ?? []).map((option) => ({ value: option.key, label: option.label }))}
      />
    )
  if (templateField.type === 'TEXTAREA') return <field.TextareaField label={label} rows={3} />
  if (templateField.type === 'NUMBER')
    return (
      <field.TextField
        label={label}
        type="number"
        placeholder={templateField.default}
        min={templateField.min}
        max={templateField.max}
      />
    )
  if (templateField.type === 'DATE') return <field.TextField label={label} type="date" />
  return <field.TextField label={label} />
}
