import { useState } from 'react'

import { FieldError } from '#/components/layout/failure-banner'
import type { Dispute } from '#/api/views'

type Questionnaire = NonNullable<Dispute['questionnaire']>
type Question = Questionnaire['questions'][number]

const input =
  'mt-1 block w-full rounded-md border px-2 py-1 text-sm focus:border-neutral-500 focus:outline-none'

/**
 * What the customer was asked and, once received, what they said. While unanswered and the dispute allows it,
 * the same panel is the form an analyst records the answers on; field errors from the API land under each question.
 */
export const QuestionnairePanel = ({
  questionnaire,
  canReceive,
  fields,
  busy,
  onReceive,
}: {
  questionnaire: Questionnaire
  canReceive: boolean
  fields: Record<string, string>
  busy: boolean
  onReceive: (answers: Record<string, string>) => void
}) => {
  const q = questionnaire
  const [draft, setDraft] = useState<Record<string, string>>({})
  const errorFor = (id: string) => fields[`answers.${id}`] ?? fields[id]

  if (q.receivedAt) {
    return (
      <div className="space-y-3 text-sm">
        <p className="text-neutral-600">
          Sent {q.sentAt.slice(0, 10)}, received {q.receivedAt.slice(0, 10)}.
        </p>
        <dl className="space-y-2" aria-label="Answers">
          {q.questions.map((question) => (
            <div key={question.id}>
              <dt className="text-neutral-500">{question.text}</dt>
              <dd className="font-medium">{q.answers?.[question.id] || 'not answered'}</dd>
            </div>
          ))}
        </dl>
        {q.inconsistencies.length > 0 && (
          <ul
            className="list-disc space-y-1 rounded-md border border-amber-200 bg-amber-50 p-3 pl-7 text-amber-900"
            aria-label="Inconsistencies"
          >
            {q.inconsistencies.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        )}
      </div>
    )
  }

  return (
    <form
      className="space-y-3 text-sm"
      onSubmit={(e) => {
        e.preventDefault()
        onReceive(Object.fromEntries(Object.entries(draft).filter(([, v]) => v.trim() !== '')))
      }}
    >
      <p className="text-neutral-600">
        Sent {q.sentAt.slice(0, 10)}; awaiting the customer. Record the answers as they come in.
      </p>
      {q.questions.map((question) => (
        <label key={question.id} className="block">
          {question.text}
          {question.required && <span className="text-neutral-400"> (required)</span>}
          <Answer
            question={question}
            value={draft[question.id] ?? ''}
            invalid={Boolean(errorFor(question.id))}
            disabled={!canReceive || busy}
            onChange={(v) => setDraft((d) => ({ ...d, [question.id]: v }))}
          />
          <FieldError id={`answer-${question.id}-error`} message={errorFor(question.id)} />
        </label>
      ))}
      {canReceive && (
        <button
          type="submit"
          disabled={busy}
          className="rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50"
        >
          Record answers
        </button>
      )}
    </form>
  )
}

const Answer = ({
  question,
  value,
  invalid,
  disabled,
  onChange,
}: {
  question: Question
  value: string
  invalid: boolean
  disabled: boolean
  onChange: (v: string) => void
}) => {
  const cls = `${input} ${invalid ? 'border-red-400' : 'border-neutral-300'}`
  const common = {
    value,
    disabled,
    'aria-invalid': invalid ? true : undefined,
    'aria-describedby': invalid ? `answer-${question.id}-error` : undefined,
    className: cls,
  }
  if (question.type === 'YES_NO')
    return (
      <select {...common} onChange={(e) => onChange(e.target.value)}>
        <option value="">choose</option>
        <option value="yes">yes</option>
        <option value="no">no</option>
      </select>
    )
  if (question.type === 'DATE')
    return <input {...common} type="date" onChange={(e) => onChange(e.target.value)} />
  if (question.type === 'AMOUNT')
    return (
      <input {...common} inputMode="decimal" placeholder="0.00" onChange={(e) => onChange(e.target.value)} />
    )
  return <textarea {...common} rows={2} onChange={(e) => onChange(e.target.value)} />
}
