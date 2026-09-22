import { useMemo, useState } from 'react'

import { preview, segments, type EmailTemplate, type TemplateField } from '#/email/render'
import { FieldError } from './failure-banner'

const input =
  'mt-1 block w-full rounded-md border px-2 py-1 text-sm focus:border-neutral-500 focus:outline-none'

type Facts = { customer: string; bank: string; today: string }
export type Draft = { id: string; filename: string; size: number }
const NO_DRAFTS: Draft[] = []

/**
 * Create Email: pick a template, fill its fields, watch the message form on the right. The preview is the same
 * substitution the server performs on send, so what the analyst sees is what the customer gets.
 */
export function EmailComposer({
  templates,
  facts,
  fields: errors,
  busy,
  onSend,
  drafts = NO_DRAFTS,
  onUpload,
  onRemoveDraft,
}: {
  templates: EmailTemplate[]
  facts: Facts
  fields: Record<string, string>
  busy: boolean
  onSend: (template: EmailTemplate['kind'], inputs: Record<string, string>) => void
  /** Files already uploaded for this email; sent along with it. */
  drafts?: Draft[]
  onUpload?: (file: File) => void
  onRemoveDraft?: (id: string) => void
}) {
  const [kind, setKind] = useState<EmailTemplate['kind']>(templates[0]?.kind ?? 'CUSTOM')
  const [inputs, setInputs] = useState<Record<string, string>>({})
  const template = templates.find((t) => t.kind === kind) ?? templates[0]
  const today = useMemo(() => new Date(`${facts.today}T00:00:00Z`), [facts.today])
  const shown = useMemo(() => (template ? preview(template, inputs, today) : null), [template, inputs, today])
  if (!template || !shown) return <p className="text-sm text-neutral-600">No templates are available.</p>
  const errorFor = (id: string) => errors[`fields.${id}`] ?? errors[id]
  const set = (id: string, v: string) => setInputs((cur) => ({ ...cur, [id]: v }))

  return (
    <div className="grid gap-8 lg:grid-cols-2">
      <form
        className="space-y-4 text-sm"
        onSubmit={(e) => {
          e.preventDefault()
          onSend(template.kind, Object.fromEntries(Object.entries(inputs).filter(([, v]) => v.trim() !== '')))
        }}
      >
        <label className="block">
          Email template
          <select
            value={template.kind}
            onChange={(e) => {
              const next = templates.find((t) => t.kind === e.target.value)
              if (next) setKind(next.kind)
              setInputs({})
            }}
            className={`${input} border-neutral-300`}
          >
            {templates.map((t) => (
              <option key={t.kind} value={t.kind}>
                {t.label}
              </option>
            ))}
          </select>
          <span className="mt-1 block text-xs text-neutral-500">
            {template.description}
            {template.letter && ' Also goes out as a letter where the regime requires written notices.'}
          </span>
        </label>
        {template.fields.map((f) => (
          <FieldInput
            key={f.id}
            field={f}
            value={inputs[f.id] ?? ''}
            error={errorFor(f.id)}
            onChange={(v) => set(f.id, v)}
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
                disabled={busy || drafts.length >= 3}
                className="mt-1 block text-sm"
                onChange={(e) => {
                  const f = e.target.files?.[0]
                  if (f) onUpload(f)
                  e.target.value = ''
                }}
              />
            </label>
            {drafts.length > 0 && (
              <ul className="mt-2 flex flex-wrap gap-2" aria-label="Attached files">
                {drafts.map((d) => (
                  <li
                    key={d.id}
                    className="flex items-center gap-1 rounded-full bg-neutral-100 px-2 py-0.5 text-xs"
                  >
                    {d.filename} <span className="text-neutral-500">({Math.ceil(d.size / 1024)} KB)</span>
                    {onRemoveDraft && (
                      <button
                        type="button"
                        aria-label={`Remove ${d.filename}`}
                        className="ml-1 text-neutral-500 hover:text-neutral-900"
                        onClick={() => onRemoveDraft(d.id)}
                      >
                        x
                      </button>
                    )}
                  </li>
                ))}
              </ul>
            )}
            <FieldError id="field-attachments-error" message={errors.attachments} />
          </div>
        )}
        <button
          type="submit"
          disabled={busy}
          className="rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50"
        >
          {busy ? 'Sending...' : 'Send email'}
        </button>
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
          <Marked line={shown.subject} />
        </h3>
        <p className="mb-4">Dear {facts.customer},</p>
        {shown.paragraphs.map((p, i) => (
          // Paragraphs are positional prose; nothing else identifies them.
          // eslint-disable-next-line react/no-array-index-key
          <p key={i} className="mb-4 whitespace-pre-line">
            <Marked line={p} />
          </p>
        ))}
        <p className="whitespace-pre-line">
          Yours sincerely,{'\n'}
          {facts.bank} disputes team
        </p>
        {drafts.length > 0 && (
          <p className="mt-6 text-sm text-neutral-600">
            Attached: {drafts.map((d) => d.filename).join(', ')}
          </p>
        )}
      </section>
    </div>
  )
}

/** Unfilled placeholders are shown as a highlighted field name, so a missing input is visible in the message itself. */
function Marked({ line }: { line: string }) {
  return (
    <>
      {segments(line).map((s, i) =>
        s.missing ? (
          <mark
            // Segments are positional runs of one line; index plus text is their identity.
            // eslint-disable-next-line react/no-array-index-key
            key={`${i}-${s.text}`}
            className="rounded bg-amber-100 px-1 font-sans text-xs text-amber-900"
            data-missing={s.text}
          >
            {s.text}
          </mark>
        ) : (
          s.text
        ),
      )}
    </>
  )
}

function FieldInput({
  field: f,
  value,
  error,
  onChange,
}: {
  field: TemplateField
  value: string
  error: string | undefined
  onChange: (v: string) => void
}) {
  const cls = `${input} ${error ? 'border-red-400' : 'border-neutral-300'}`
  const common = {
    'aria-invalid': error ? true : undefined,
    'aria-describedby': error ? `field-${f.id}-error` : undefined,
  }
  const label = (
    <>
      {f.label}
      {f.required && <span className="text-neutral-400"> (required)</span>}
    </>
  )
  if (f.type === 'MULTISELECT') {
    const chosen = new Set(value.split(',').filter(Boolean))
    return (
      <fieldset aria-describedby={common['aria-describedby']}>
        <legend className="mb-1">{label}</legend>
        <div className="space-y-1">
          {f.options?.map((o) => (
            <label key={o.key} className="flex items-start gap-2">
              <input
                type="checkbox"
                className="mt-1"
                checked={chosen.has(o.key)}
                onChange={(e) => {
                  const next = new Set(chosen)
                  if (e.target.checked) next.add(o.key)
                  else next.delete(o.key)
                  onChange([...next].join(','))
                }}
              />
              <span>
                {o.label}
                <span className="block text-xs text-neutral-500">{o.text}</span>
              </span>
            </label>
          ))}
        </div>
        <FieldError id={`field-${f.id}-error`} message={error} />
      </fieldset>
    )
  }
  return (
    <label className="block">
      {label}
      {f.type === 'SELECT' && (
        <select {...common} value={value} onChange={(e) => onChange(e.target.value)} className={cls}>
          <option value="">choose</option>
          {f.options?.map((o) => (
            <option key={o.key} value={o.key}>
              {o.label}
            </option>
          ))}
        </select>
      )}
      {f.type === 'TEXTAREA' && (
        <textarea
          {...common}
          value={value}
          rows={3}
          onChange={(e) => onChange(e.target.value)}
          className={cls}
        />
      )}
      {f.type === 'TEXT' && (
        <input {...common} value={value} onChange={(e) => onChange(e.target.value)} className={cls} />
      )}
      {f.type === 'NUMBER' && (
        <input
          {...common}
          type="number"
          value={value}
          placeholder={f.default}
          min={f.min}
          max={f.max}
          onChange={(e) => onChange(e.target.value)}
          className={cls}
        />
      )}
      {f.type === 'DATE' && (
        <input
          {...common}
          type="date"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className={cls}
        />
      )}
      <FieldError id={`field-${f.id}-error`} message={error} />
    </label>
  )
}
