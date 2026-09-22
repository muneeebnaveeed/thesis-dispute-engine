import type { components } from '#/api/schema.gen'

export type EmailTemplate = components['schemas']['EmailTemplate']
export type TemplateField = EmailTemplate['fields'][number]

/**
 * The browser half of the template language (the server half is notice.Fill): {{id}} placeholders, {{#id}}...{{/id}}
 * sections that appear only when the field has a value, paragraphs that are only a list field rendered as bullet
 * lines, empty paragraphs dropped. Facts are already substituted by the server; dueDate is computed here from "days".
 */
const placeholder = /\{\{([A-Za-z][A-Za-z0-9]*)\}\}/g
const section = /\{\{#([A-Za-z][A-Za-z0-9]*)\}\}(.*?)\{\{\/([A-Za-z][A-Za-z0-9]*)\}\}/g

/** What the customer will read for each field, from what the analyst typed or chose. */
export function displayValues(
  fields: TemplateField[],
  inputs: Record<string, string>,
  today: Date,
): Record<string, string> {
  const out: Record<string, string> = {}
  for (const f of fields) {
    const raw = (inputs[f.id] ?? '').trim() || (f.default ?? '')
    if (!raw) continue
    switch (f.type) {
      case 'SELECT':
        out[f.id] = f.options?.find((o) => o.key === raw)?.text ?? ''
        break
      case 'MULTISELECT':
        out[f.id] = raw
          .split(',')
          .map((k) => f.options?.find((o) => o.key === k.trim())?.text ?? '')
          .filter(Boolean)
          .join('\n')
        break
      case 'DATE': {
        const d = new Date(`${raw}T00:00:00Z`)
        out[f.id] = Number.isNaN(d.getTime()) ? raw : longDate(d)
        break
      }
      case 'NUMBER':
      case 'TEXT':
      case 'TEXTAREA':
        out[f.id] = raw
    }
  }
  const days = Number(out.days)
  if (Number.isInteger(days) && days > 0) {
    const due = new Date(today)
    due.setUTCDate(due.getUTCDate() + days)
    out.dueDate = longDate(due)
  }
  return out
}

export function longDate(d: Date): string {
  return `${d.getUTCDate()} ${d.toLocaleString('en-GB', { month: 'long', timeZone: 'UTC' })} ${d.getUTCFullYear()}`
}

const listy = (fields: TemplateField[]) =>
  new Set(fields.filter((f) => f.type === 'MULTISELECT' || f.type === 'TEXTAREA').map((f) => f.id))

/** Renders one line with the display values; unfilled placeholders stay as {{id}} so the preview can mark them. */
export function substitute(
  line: string,
  values: Record<string, string>,
  lists: Set<string>,
  keepUnfilled: boolean,
): string {
  let s = line.replace(section, (_m, id: string, inner: string) => (values[id] ? inner : ''))
  const onlyId = /^\{\{([A-Za-z][A-Za-z0-9]*)\}\}$/.exec(s.trim())?.[1]
  const listValue = onlyId && lists.has(onlyId) ? values[onlyId] : undefined
  if (listValue) {
    return listValue
      .split('\n')
      .map((l: string) => l.trim())
      .filter(Boolean)
      .map((l) => `- ${l.replace(/^- /, '')}`)
      .join('\n')
  }
  s = s.replace(placeholder, (m, id: string) => values[id] ?? (keepUnfilled ? m : ''))
  return s.trim()
}

/** The whole message as the customer would read it; paragraphs that come out empty are dropped. */
export function preview(t: EmailTemplate, inputs: Record<string, string>, today: Date) {
  const values = displayValues(t.fields, inputs, today)
  const lists = listy(t.fields)
  return {
    subject: substitute(t.subject, values, lists, true),
    paragraphs: t.paragraphs.map((p) => substitute(p, values, lists, true)).filter((p) => p !== ''),
  }
}

/** Splits a preview line into text and unfilled {{placeholder}} runs so the UI can mark what is still missing. */
export function segments(line: string): { text: string; missing: boolean }[] {
  const out: { text: string; missing: boolean }[] = []
  let last = 0
  for (const m of line.matchAll(placeholder)) {
    if (m.index > last) out.push({ text: line.slice(last, m.index), missing: false })
    out.push({ text: m[1] ?? '', missing: true })
    last = m.index + m[0].length
  }
  if (last < line.length) out.push({ text: line.slice(last), missing: false })
  return out
}
