import { useState } from 'react'

import type { components } from '#/api/schema.gen'
import { FieldError } from '#/components/layout/failure-banner'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { inputVariants } from '#/components/ui/field'
import { cn } from '#/lib/cn'
import { emailLineSegments, renderEmailPreview } from '#/lib/email/render'

type TemplateSetting = components['schemas']['TemplateSetting']
type TemplateOverride = components['schemas']['TemplateOverride']
type TemplateWording = TemplateSetting['effective']

const FACT_PLACEHOLDERS = ['customer', 'bank', 'amount', 'merchant', 'dispute', 'today', 'dueDate']

const optionTextKey = (fieldId: string, optionKey: string) => `${fieldId}.${optionKey}`

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
  fields: fieldErrors,
  busy,
  onSave,
  onRevert,
}: {
  setting: TemplateSetting
  fields: Record<string, string>
  busy: boolean
  onSave: (override: TemplateOverride) => void
  onRevert: () => void
}) => {
  const { base, effective, override } = setting
  const [editing, setEditing] = useState(false)
  const [label, setLabel] = useState(effective.label)
  const [description, setDescription] = useState(effective.description)
  const [subject, setSubject] = useState(effective.subject)
  const [paragraphsText, setParagraphsText] = useState(effective.paragraphs.join('\n\n'))
  const [optionTexts, setOptionTexts] = useState<Record<string, string>>(() => {
    const initial: Record<string, string> = {}
    for (const field of effective.fields) {
      for (const option of field.options ?? []) initial[optionTextKey(field.id, option.key)] = option.text
    }
    return initial
  })

  const draftWording: TemplateWording = {
    ...effective,
    label,
    subject,
    paragraphs: paragraphsText
      .split(/\n\s*\n/)
      .map((paragraph) => paragraph.trim())
      .filter(Boolean),
    fields: effective.fields.map((field) => {
      const options = field.options?.map((option) => ({
        ...option,
        text: optionTexts[optionTextKey(field.id, option.key)] ?? option.text,
      }))
      return options ? { ...field, options } : field
    }),
  }
  const renderedSample = renderEmailPreview(draftWording, sampleInputsFor(draftWording), new Date())
  const placeholderNames = [...base.fields.map((field) => field.id), ...FACT_PLACEHOLDERS]

  const saveChangedWording = () => {
    const changedOptionTexts: Record<string, string> = {}
    for (const field of base.fields) {
      for (const option of field.options ?? []) {
        const text = optionTexts[optionTextKey(field.id, option.key)]
        if (text !== undefined && text !== option.text)
          changedOptionTexts[optionTextKey(field.id, option.key)] = text
      }
    }
    const paragraphsChanged = draftWording.paragraphs.join('\n') !== base.paragraphs.join('\n')
    onSave({
      ...(label !== base.label ? { label } : {}),
      ...(description !== base.description ? { description } : {}),
      ...(subject !== base.subject ? { subject } : {}),
      ...(paragraphsChanged ? { paragraphs: draftWording.paragraphs } : {}),
      ...(Object.keys(changedOptionTexts).length ? { optionTexts: changedOptionTexts } : {}),
    })
  }

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
          <form
            className="space-y-3 text-sm"
            onSubmit={(event) => {
              event.preventDefault()
              saveChangedWording()
            }}
          >
            <label className="block">
              Name analysts see
              <input
                value={label}
                onChange={(event) => setLabel(event.target.value)}
                className={inputVariants()}
              />
            </label>
            <label className="block">
              Description
              <input
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                className={inputVariants()}
              />
            </label>
            <label className="block">
              Subject
              <input
                value={subject}
                onChange={(event) => setSubject(event.target.value)}
                className={inputVariants()}
              />
            </label>
            <label className="block">
              Paragraphs <span className="text-neutral-400">(separate with a blank line)</span>
              <textarea
                value={paragraphsText}
                rows={8}
                onChange={(event) => setParagraphsText(event.target.value)}
                className={cn(inputVariants({ mono: true }), 'text-xs')}
              />
            </label>
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
                    <label key={optionTextKey(field.id, option.key)} className="block">
                      <span className="text-neutral-600">
                        {field.label}: {option.label}
                      </span>
                      <input
                        value={optionTexts[optionTextKey(field.id, option.key)] ?? option.text}
                        onChange={(event) =>
                          setOptionTexts((current) => ({
                            ...current,
                            [optionTextKey(field.id, option.key)]: event.target.value,
                          }))
                        }
                        className={inputVariants()}
                      />
                    </label>
                  )),
                )}
              </fieldset>
            )}
            <FieldError
              id={`template-${base.kind}-error`}
              message={fieldErrors.body ?? Object.values(fieldErrors)[0]}
            />
            <div className="flex items-center gap-3">
              <Button type="submit" disabled={busy}>
                Save wording
              </Button>
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
