import { useStore } from '@tanstack/react-form'
import { useState } from 'react'

import type { Failure } from '#/api/failure'
import type { components } from '#/api/schema.gen'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { FieldError } from '#/components/ui/field'
import { submitTo, submitting, useAppForm } from '#/forms/app-form'
import { emailLineSegments, renderEmailPreview } from '#/lib/email/render'

type TemplateSetting = components['schemas']['TemplateSetting']
type TemplateOverride = components['schemas']['TemplateOverride']
type TemplateWording = TemplateSetting['effective']

const FACT_PLACEHOLDERS = ['customer', 'bank', 'amount', 'merchant', 'dispute', 'today', 'dueDate']

const optionTextKey = (fieldId: string, optionKey: string) => `${fieldId}.${optionKey}`

const splitParagraphs = (text: string) =>
  text
    .split(/\n\s*\n/)
    .map((paragraph) => paragraph.trim())
    .filter(Boolean)

const sampleInputsFor = (wording: TemplateWording): Record<string, string> => {
  const sampleInputs: Record<string, string> = {}
  for (const field of wording.fields) {
    if (field.type === 'SELECT') sampleInputs[field.id] = field.options?.[0]?.key ?? ''
    else if (field.type === 'MULTISELECT')
      sampleInputs[field.id] = (field.options ?? [])
        .slice(0, 2)
        .map((option) => option.key)
        .join(',')
    else if (field.type === 'DATE') sampleInputs[field.id] = '2026-10-15'
    else if (field.type === 'NUMBER') sampleInputs[field.id] = field.default ?? '10'
    else sampleInputs[field.id] = `[${field.label}]`
  }
  return sampleInputs
}

export const TemplateEditor = ({
  setting,
  failure,
  busy,
  onSave,
  onRevert,
}: {
  setting: TemplateSetting
  failure: Failure | null
  busy: boolean
  onSave: (override: TemplateOverride) => Promise<unknown>
  onRevert: () => void
}) => {
  const { base, effective, override } = setting
  const [editing, setEditing] = useState(false)
  const initialOptionTexts: Record<string, string> = {}
  for (const field of effective.fields) {
    for (const option of field.options ?? [])
      initialOptionTexts[optionTextKey(field.id, option.key)] = option.text
  }
  const wordingForm = useAppForm({
    defaultValues: {
      label: effective.label,
      description: effective.description,
      subject: effective.subject,
      paragraphs: effective.paragraphs.join('\n\n'),
      optionTexts: initialOptionTexts,
    },
    onSubmit: ({ value, formApi }) => {
      const changedOptionTexts: Record<string, string> = {}
      for (const field of base.fields) {
        for (const option of field.options ?? []) {
          const text = value.optionTexts[optionTextKey(field.id, option.key)]
          if (text !== undefined && text !== option.text)
            changedOptionTexts[optionTextKey(field.id, option.key)] = text
        }
      }
      const paragraphs = splitParagraphs(value.paragraphs)
      return submitTo(formApi, () =>
        onSave({
          ...(value.label !== base.label ? { label: value.label } : {}),
          ...(value.description !== base.description ? { description: value.description } : {}),
          ...(value.subject !== base.subject ? { subject: value.subject } : {}),
          ...(paragraphs.join('\n') !== base.paragraphs.join('\n') ? { paragraphs } : {}),
          ...(Object.keys(changedOptionTexts).length ? { optionTexts: changedOptionTexts } : {}),
        }),
      )
    },
  })
  const draft = useStore(wordingForm.store, (state) => state.values)
  const draftWording: TemplateWording = {
    ...effective,
    label: draft.label,
    subject: draft.subject,
    paragraphs: splitParagraphs(draft.paragraphs),
    fields: effective.fields.map((field) => {
      const options = field.options?.map((option) => ({
        ...option,
        text: draft.optionTexts[optionTextKey(field.id, option.key)] ?? option.text,
      }))
      return options ? { ...field, options } : field
    }),
  }
  const renderedSample = renderEmailPreview(draftWording, sampleInputsFor(draftWording), new Date())
  const placeholderNames = [...base.fields.map((field) => field.id), ...FACT_PLACEHOLDERS]
  const refusal = failure?.kind === 'validation' ? failure.problem.detail : undefined

  return (
    <section className="rounded-md border border-neutral-200" aria-label={base.label}>
      <header className="flex items-center justify-between px-4 py-3">
        <div>
          <h2 className="font-medium">
            {effective.label}
            {override && (
              <Badge tone="warn" className="ml-2">
                customised
              </Badge>
            )}
          </h2>
          <p className="text-sm text-neutral-600">{effective.description}</p>
        </div>
        <Button variant="link" size="bare" onClick={() => setEditing((current) => !current)}>
          {editing ? 'Close' : 'Edit wording'}
        </Button>
      </header>
      {editing && (
        <div className="grid gap-6 border-t border-neutral-200 p-4 lg:grid-cols-2">
          <form className="space-y-3 text-sm" onSubmit={submitting(wordingForm)}>
            <wordingForm.AppField name="label">
              {(field) => <field.TextField label="Name analysts see" />}
            </wordingForm.AppField>
            <wordingForm.AppField name="description">
              {(field) => <field.TextField label="Description" />}
            </wordingForm.AppField>
            <wordingForm.AppField name="subject">
              {(field) => <field.TextField label="Subject" />}
            </wordingForm.AppField>
            <wordingForm.AppField name="paragraphs">
              {(field) => (
                <field.TextareaField
                  label="Paragraphs"
                  hint="(separate with a blank line)"
                  rows={8}
                  mono
                  inputClassName="text-xs"
                />
              )}
            </wordingForm.AppField>
            <p className="text-xs text-neutral-500">
              Placeholders: {placeholderNames.map((name) => `{{${name}}}`).join(' ')}. Wrap text in{' '}
              {'{{#field}}'}
              ...{'{{/field}}'} to show it only when the field is filled.
            </p>
            {base.fields.some((field) => field.options?.length) && (
              <fieldset className="space-y-2">
                <legend className="font-medium">What the customer reads for each option</legend>
                {base.fields.map((field) =>
                  field.options?.map((option) => (
                    <wordingForm.AppField
                      key={optionTextKey(field.id, option.key)}
                      name={`optionTexts.${optionTextKey(field.id, option.key)}`}
                    >
                      {(boundField) => (
                        <boundField.TextField
                          label={
                            <span className="text-neutral-600">
                              {field.label}: {option.label}
                            </span>
                          }
                        />
                      )}
                    </wordingForm.AppField>
                  )),
                )}
              </fieldset>
            )}
            <FieldError id={`template-${base.kind}-error`} message={refusal} />
            <div className="flex items-center gap-3">
              <wordingForm.AppForm>
                <wordingForm.SubmitButton busy={busy}>Save wording</wordingForm.SubmitButton>
              </wordingForm.AppForm>
              {override && (
                <Button variant="link" size="bare" disabled={busy} onClick={onRevert}>
                  Revert to standard
                </Button>
              )}
            </div>
          </form>
          <section
            aria-label={`Preview of ${base.label}`}
            className="rounded-md border border-neutral-200 bg-white p-6 font-serif text-[12pt] leading-relaxed"
          >
            <h3 className="mb-4 text-[14pt] font-semibold">{renderedSample.subject}</h3>
            <p className="mb-4">Dear Kovács Anna,</p>
            {renderedSample.paragraphs.map((paragraph, paragraphIndex) => (
              <p
                // eslint-disable-next-line react/no-array-index-key -- paragraphs are positional prose
                key={`${paragraphIndex}-${paragraph.length}`}
                className="mb-4 whitespace-pre-line"
              >
                {emailLineSegments(paragraph).map((segment, segmentIndex) =>
                  segment.unfilledPlaceholder ? (
                    <mark
                      // eslint-disable-next-line react/no-array-index-key -- segments are positional runs of one line
                      key={`${segmentIndex}-${segment.text}`}
                      className="rounded bg-red-100 px-1 font-sans text-xs text-red-900"
                    >
                      unknown: {segment.text}
                    </mark>
                  ) : (
                    segment.text
                  ),
                )}
              </p>
            ))}
          </section>
        </div>
      )}
    </section>
  )
}
