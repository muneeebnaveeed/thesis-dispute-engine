import { useMemo, useState } from 'react'

import { FieldError } from '#/components/layout/failure-banner'
import { Button } from '#/components/ui/button'
import { inputVariants, invalidProps } from '#/components/ui/field'
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

export const EmailComposer = ({
  templates,
  facts,
  fields: fieldErrors,
  busy,
  onSend,
  attachmentDrafts = NO_ATTACHMENT_DRAFTS,
  onUpload,
  onRemoveAttachmentDraft,
}: {
  templates: EmailTemplate[]
  facts: EmailFacts
  fields: Record<string, string>
  busy: boolean
  onSend: (templateKind: EmailTemplate['kind'], analystInputs: Record<string, string>) => void
  attachmentDrafts?: AttachmentDraft[]
  onUpload?: (file: File) => void
  onRemoveAttachmentDraft?: (draftId: string) => void
}) => {
  const [selectedKind, setSelectedKind] = useState<EmailTemplate['kind']>(templates[0]?.kind ?? 'CUSTOM')
  const [analystInputs, setAnalystInputs] = useState<Record<string, string>>({})
  const selectedTemplate = templates.find((template) => template.kind === selectedKind) ?? templates[0]
  const today = useMemo(() => new Date(`${facts.today}T00:00:00Z`), [facts.today])
  const renderedEmail = useMemo(
    () => (selectedTemplate ? renderEmailPreview(selectedTemplate, analystInputs, today) : null),
    [selectedTemplate, analystInputs, today],
  )
  if (!selectedTemplate || !renderedEmail) {
    return <p className="text-sm text-neutral-600">No templates are available.</p>
  }
  const fieldErrorFor = (fieldId: string) => fieldErrors[`fields.${fieldId}`] ?? fieldErrors[fieldId]
  const setAnalystInput = (fieldId: string, value: string) =>
    setAnalystInputs((current) => ({ ...current, [fieldId]: value }))
  const filledInputs = () =>
    Object.fromEntries(Object.entries(analystInputs).filter(([, value]) => value.trim() !== ''))

  return (
    <div className="grid gap-8 lg:grid-cols-2">
      <form
        className="space-y-4 text-sm"
        onSubmit={(event) => {
          event.preventDefault()
          onSend(selectedTemplate.kind, filledInputs())
        }}
      >
        <label className="block">
          Email template
          <select
            value={selectedTemplate.kind}
            onChange={(event) => {
              const nextTemplate = templates.find((template) => template.kind === event.target.value)
              if (nextTemplate) setSelectedKind(nextTemplate.kind)
              setAnalystInputs({})
            }}
            className={inputVariants()}
          >
            {templates.map((template) => (
              <option key={template.kind} value={template.kind}>
                {template.label}
              </option>
            ))}
          </select>
          <span className="mt-1 block text-xs text-neutral-500">
            {selectedTemplate.description}
            {selectedTemplate.letter &&
              ' Also goes out as a letter where the regime requires written notices.'}
          </span>
        </label>
        {selectedTemplate.fields.map((field) => (
          <TemplateFieldInput
            key={field.id}
            field={field}
            value={analystInputs[field.id] ?? ''}
            error={fieldErrorFor(field.id)}
            onChange={(value) => setAnalystInput(field.id, value)}
          />
        ))}
        {onUpload && (
          <div>
            <label className="block">
              Attachments{' '}
              <span className="text-neutral-400">(PDF, PNG or JPEG, at most 5 MB each, up to 3)</span>
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
                    className="flex items-center gap-1 rounded-full bg-neutral-100 px-2 py-0.5 text-xs"
                  >
                    {draft.filename}{' '}
                    <span className="text-neutral-500">({Math.ceil(draft.size / 1024)} KB)</span>
                    {onRemoveAttachmentDraft && (
                      <button
                        type="button"
                        aria-label={`Remove ${draft.filename}`}
                        className="ml-1 text-neutral-500 hover:text-neutral-900"
                        onClick={() => onRemoveAttachmentDraft(draft.id)}
                      >
                        x
                      </button>
                    )}
                  </li>
                ))}
              </ul>
            )}
            <FieldError id="field-attachments-error" message={fieldErrors.attachments} />
          </div>
        )}
        <Button type="submit" disabled={busy}>
          {busy ? 'Sending...' : 'Send email'}
        </Button>
      </form>

      <section
        aria-label="Preview"
        className="rounded-md border border-neutral-200 bg-white p-6 font-serif text-[12pt] leading-relaxed"
      >
        <p className="mb-6 text-sm text-neutral-500">
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
          <p className="mt-6 text-sm text-neutral-600">
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
  field,
  value,
  error,
  onChange,
}: {
  field: TemplateField
  value: string
  error: string | undefined
  onChange: (value: string) => void
}) => {
  const inputClass = inputVariants({ invalid: Boolean(error) })
  const a11y = invalidProps(`field-${field.id}`, error)
  const fieldLabel = (
    <>
      {field.label}
      {field.required && <span className="text-neutral-400"> (required)</span>}
    </>
  )
  if (field.type === 'MULTISELECT') {
    const chosenOptionKeys = new Set(value.split(',').filter(Boolean))
    return (
      <fieldset aria-describedby={a11y['aria-describedby']}>
        <legend className="mb-1">{fieldLabel}</legend>
        <div className="space-y-1">
          {field.options?.map((option) => (
            <label key={option.key} className="flex items-start gap-2">
              <input
                type="checkbox"
                className="mt-1"
                checked={chosenOptionKeys.has(option.key)}
                onChange={(event) => {
                  const nextChosen = new Set(chosenOptionKeys)
                  if (event.target.checked) nextChosen.add(option.key)
                  else nextChosen.delete(option.key)
                  onChange([...nextChosen].join(','))
                }}
              />
              <span>
                {option.label}
                <span className="block text-xs text-neutral-500">{option.text}</span>
              </span>
            </label>
          ))}
        </div>
        <FieldError id={`field-${field.id}-error`} message={error} />
      </fieldset>
    )
  }
  return (
    <label className="block">
      {fieldLabel}
      {field.type === 'SELECT' && (
        <select
          {...a11y}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className={inputClass}
        >
          <option value="">choose</option>
          {field.options?.map((option) => (
            <option key={option.key} value={option.key}>
              {option.label}
            </option>
          ))}
        </select>
      )}
      {field.type === 'TEXTAREA' && (
        <textarea
          {...a11y}
          value={value}
          rows={3}
          onChange={(event) => onChange(event.target.value)}
          className={inputClass}
        />
      )}
      {field.type === 'TEXT' && (
        <input
          {...a11y}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className={inputClass}
        />
      )}
      {field.type === 'NUMBER' && (
        <input
          {...a11y}
          type="number"
          value={value}
          placeholder={field.default}
          min={field.min}
          max={field.max}
          onChange={(event) => onChange(event.target.value)}
          className={inputClass}
        />
      )}
      {field.type === 'DATE' && (
        <input
          {...a11y}
          type="date"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className={inputClass}
        />
      )}
      <FieldError id={`field-${field.id}-error`} message={error} />
    </label>
  )
}
