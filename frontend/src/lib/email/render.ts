import type { components } from '#/api/schema.gen'

export type EmailTemplate = components['schemas']['EmailTemplate']
export type TemplateField = EmailTemplate['fields'][number]
export type EmailLineSegment = { text: string; unfilledPlaceholder: boolean }
export type RenderedEmail = { subject: string; paragraphs: string[] }

// the browser half of the template language; the server half is notice.Fill and the two must agree
const placeholderPattern = /\{\{([A-Za-z][A-Za-z0-9]*)\}\}/g
const sectionPattern = /\{\{#([A-Za-z][A-Za-z0-9]*)\}\}(.*?)\{\{\/([A-Za-z][A-Za-z0-9]*)\}\}/g
const lonePlaceholderPattern = /^\{\{([A-Za-z][A-Za-z0-9]*)\}\}$/

export const customerFacingFieldValues = (
  fields: TemplateField[],
  analystInputs: Record<string, string>,
  today: Date,
): Record<string, string> => {
  const fieldValues: Record<string, string> = {}
  for (const field of fields) {
    const rawInput = (analystInputs[field.id] ?? '').trim() || (field.default ?? '')
    if (!rawInput) continue
    switch (field.type) {
      case 'SELECT':
        fieldValues[field.id] = field.options?.find((option) => option.key === rawInput)?.text ?? ''
        break
      case 'MULTISELECT':
        fieldValues[field.id] = rawInput
          .split(',')
          .map((optionKey) => field.options?.find((option) => option.key === optionKey.trim())?.text ?? '')
          .filter(Boolean)
          .join('\n')
        break
      case 'DATE': {
        const date = new Date(`${rawInput}T00:00:00Z`)
        fieldValues[field.id] = Number.isNaN(date.getTime()) ? rawInput : formatLongDate(date)
        break
      }
      case 'NUMBER':
      case 'TEXT':
      case 'TEXTAREA':
        fieldValues[field.id] = rawInput
    }
  }
  const daysToReply = Number(fieldValues.days)
  if (Number.isInteger(daysToReply) && daysToReply > 0) {
    const dueDate = new Date(today)
    dueDate.setUTCDate(dueDate.getUTCDate() + daysToReply)
    fieldValues.dueDate = formatLongDate(dueDate)
  }
  return fieldValues
}

const formatLongDate = (date: Date): string =>
  `${date.getUTCDate()} ${date.toLocaleString('en-GB', { month: 'long', timeZone: 'UTC' })} ${date.getUTCFullYear()}`

const listFieldIds = (fields: TemplateField[]) =>
  new Set(
    fields
      .filter((field) => field.type === 'MULTISELECT' || (field.type === 'TEXTAREA' && field.list))
      .map((field) => field.id),
  )

const fillEmailLine = (
  templateLine: string,
  fieldValues: Record<string, string>,
  listFields: Set<string>,
  keepUnfilledPlaceholders: boolean,
): string => {
  const withSectionsResolved = templateLine.replace(
    sectionPattern,
    (_match, fieldId: string, inner: string) => (fieldValues[fieldId] ? inner : ''),
  )
  const loneFieldId = lonePlaceholderPattern.exec(withSectionsResolved.trim())?.[1]
  const listValue = loneFieldId && listFields.has(loneFieldId) ? fieldValues[loneFieldId] : undefined
  if (listValue) {
    return listValue
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean)
      .map((item) => `- ${item.replace(/^- /, '')}`)
      .join('\n')
  }
  return withSectionsResolved
    .replace(
      placeholderPattern,
      (match, fieldId: string) => fieldValues[fieldId] ?? (keepUnfilledPlaceholders ? match : ''),
    )
    .trim()
}

export const renderEmailPreview = (
  template: EmailTemplate,
  analystInputs: Record<string, string>,
  today: Date,
): RenderedEmail => {
  const fieldValues = customerFacingFieldValues(template.fields, analystInputs, today)
  const listFields = listFieldIds(template.fields)
  return {
    subject: fillEmailLine(template.subject, fieldValues, listFields, true),
    paragraphs: template.paragraphs
      .map((paragraph) => fillEmailLine(paragraph, fieldValues, listFields, true))
      .filter((paragraph) => paragraph !== ''),
  }
}

export const emailLineSegments = (renderedLine: string): EmailLineSegment[] => {
  const segments: EmailLineSegment[] = []
  let consumedUpTo = 0
  for (const match of renderedLine.matchAll(placeholderPattern)) {
    if (match.index > consumedUpTo) {
      segments.push({
        text: renderedLine.slice(consumedUpTo, match.index),
        unfilledPlaceholder: false,
      })
    }
    segments.push({ text: match[1] ?? '', unfilledPlaceholder: true })
    consumedUpTo = match.index + match[0].length
  }
  if (consumedUpTo < renderedLine.length) {
    segments.push({ text: renderedLine.slice(consumedUpTo), unfilledPlaceholder: false })
  }
  return segments
}
