import { useState } from 'react'

import type { components } from '#/api/schema.gen'
import { preview, segments } from '#/lib/email/render'
import { FieldError } from '#/components/layout/failure-banner'

type TemplateSetting = components['schemas']['TemplateSetting']
type TemplateOverride = components['schemas']['TemplateOverride']

const input =
  'mt-1 block w-full rounded-md border border-neutral-300 px-2 py-1 text-sm focus:border-neutral-500 focus:outline-none'

/** One kind: the tenant's words on the left, the customer's view on the right with sample values in the fields. */
export const TemplateEditor = ({
  setting,
  fields: errors,
  busy,
  onSave,
  onRevert,
}: {
  setting: TemplateSetting
  fields: Record<string, string>
  busy: boolean
  onSave: (o: TemplateOverride) => void
  onRevert: () => void
}) => {
  const { base, effective, override } = setting
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState(effective.label)
  const [description, setDescription] = useState(effective.description)
  const [subject, setSubject] = useState(effective.subject)
  const [paragraphs, setParagraphs] = useState(effective.paragraphs.join('\n\n'))
  const [optionTexts, setOptionTexts] = useState<Record<string, string>>(() => {
    const out: Record<string, string> = {}
    for (const f of effective.fields) for (const o of f.options ?? []) out[`${f.id}.${o.key}`] = o.text
    return out
  })

  // The preview shows the template with every field filled by a sample, so a rewording is judged as a letter.
  const draft: TemplateSetting['effective'] = {
    ...effective,
    label,
    subject,
    paragraphs: paragraphs
      .split(/\n\s*\n/)
      .map((p) => p.trim())
      .filter(Boolean),
    fields: effective.fields.map((f) => {
      const options = f.options?.map((o) => ({ ...o, text: optionTexts[`${f.id}.${o.key}`] ?? o.text }))
      return options ? { ...f, options } : f
    }),
  }
  const sample: Record<string, string> = {}
  for (const f of draft.fields) {
    if (f.type === 'SELECT') sample[f.id] = f.options?.[0]?.key ?? ''
    else if (f.type === 'MULTISELECT')
      sample[f.id] = (f.options ?? [])
        .slice(0, 2)
        .map((o) => o.key)
        .join(',')
    else if (f.type === 'DATE') sample[f.id] = '2026-10-15'
    else if (f.type === 'NUMBER') sample[f.id] = f.default ?? '10'
    else sample[f.id] = `[${f.label}]`
  }
  const shown = preview(draft, sample, new Date())
  const placeholders = [
    ...base.fields.map((f) => f.id),
    'customer',
    'bank',
    'amount',
    'merchant',
    'dispute',
    'today',
    'dueDate',
  ]

  const submit = () => {
    const changed: Record<string, string> = {}
    for (const f of base.fields)
      for (const o of f.options ?? []) {
        const v = optionTexts[`${f.id}.${o.key}`]
        if (v !== undefined && v !== o.text) changed[`${f.id}.${o.key}`] = v
      }
    onSave({
      ...(label !== base.label ? { label } : {}),
      ...(description !== base.description ? { description } : {}),
      ...(subject !== base.subject ? { subject } : {}),
      ...(draft.paragraphs.join('\n') !== base.paragraphs.join('\n') ? { paragraphs: draft.paragraphs } : {}),
      ...(Object.keys(changed).length ? { optionTexts: changed } : {}),
    })
  }

  return (
    <section className="rounded-md border border-neutral-200" aria-label={base.label}>
      <header className="flex items-center justify-between px-4 py-3">
        <div>
          <h2 className="font-medium">
            {effective.label}
            {override && (
              <span className="ml-2 rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-900">
                customised
              </span>
            )}
          </h2>
          <p className="text-sm text-neutral-600">{effective.description}</p>
        </div>
        <button type="button" className="text-sm underline" onClick={() => setOpen((v) => !v)}>
          {open ? 'Close' : 'Edit wording'}
        </button>
      </header>
      {open && (
        <div className="grid gap-6 border-t border-neutral-200 p-4 lg:grid-cols-2">
          <form
            className="space-y-3 text-sm"
            onSubmit={(e) => {
              e.preventDefault()
              submit()
            }}
          >
            <label className="block">
              Name analysts see
              <input value={label} onChange={(e) => setLabel(e.target.value)} className={input} />
            </label>
            <label className="block">
              Description
              <input value={description} onChange={(e) => setDescription(e.target.value)} className={input} />
            </label>
            <label className="block">
              Subject
              <input value={subject} onChange={(e) => setSubject(e.target.value)} className={input} />
            </label>
            <label className="block">
              Paragraphs <span className="text-neutral-400">(separate with a blank line)</span>
              <textarea
                value={paragraphs}
                rows={8}
                onChange={(e) => setParagraphs(e.target.value)}
                className={`${input} font-mono text-xs`}
              />
            </label>
            <p className="text-xs text-neutral-500">
              Placeholders: {placeholders.map((p) => `{{${p}}}`).join(' ')}. Wrap text in {'{{#field}}'}...
              {'{{/field}}'} to show it only when the field is filled.
            </p>
            {base.fields.some((f) => f.options?.length) && (
              <fieldset className="space-y-2">
                <legend className="font-medium">What the customer reads for each option</legend>
                {base.fields.map((f) =>
                  f.options?.map((o) => (
                    <label key={`${f.id}.${o.key}`} className="block">
                      <span className="text-neutral-600">
                        {f.label}: {o.label}
                      </span>
                      <input
                        value={optionTexts[`${f.id}.${o.key}`] ?? o.text}
                        onChange={(e) =>
                          setOptionTexts((cur) => ({ ...cur, [`${f.id}.${o.key}`]: e.target.value }))
                        }
                        className={input}
                      />
                    </label>
                  )),
                )}
              </fieldset>
            )}
            <FieldError
              id={`template-${base.kind}-error`}
              message={errors.body ?? Object.values(errors)[0]}
            />
            <div className="flex gap-3">
              <button
                type="submit"
                disabled={busy}
                className="rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50"
              >
                Save wording
              </button>
              {override && (
                <button
                  type="button"
                  disabled={busy}
                  className="text-sm underline disabled:opacity-50"
                  onClick={onRevert}
                >
                  Revert to standard
                </button>
              )}
            </div>
          </form>
          <section
            aria-label={`Preview of ${base.label}`}
            className="rounded-md border border-neutral-200 bg-white p-6 font-serif text-[12pt] leading-relaxed"
          >
            <h3 className="mb-4 text-[14pt] font-semibold">{shown.subject}</h3>
            <p className="mb-4">Dear Kovács Anna,</p>
            {shown.paragraphs.map((p, i) => (
              <p
                // Paragraphs are positional prose.
                // eslint-disable-next-line react/no-array-index-key
                key={`${i}-${p.length}`}
                className="mb-4 whitespace-pre-line"
              >
                {segments(p).map((s, j) =>
                  s.missing ? (
                    <mark
                      // Positional run of one line.
                      // eslint-disable-next-line react/no-array-index-key
                      key={`${j}-${s.text}`}
                      className="rounded bg-red-100 px-1 font-sans text-xs text-red-900"
                    >
                      unknown: {s.text}
                    </mark>
                  ) : (
                    s.text
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
