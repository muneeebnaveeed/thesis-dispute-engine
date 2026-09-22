import { useState } from 'react'

import type { Dispute } from '#/api/views'
import { FieldError } from '#/components/layout/failure-banner'
import { Button } from '#/components/ui/button'
import { inputVariants, invalidProps } from '#/components/ui/field'

type Questionnaire = NonNullable<Dispute['questionnaire']>
type Question = Questionnaire['questions'][number]

export const QuestionnairePanel = ({
  questionnaire,
  canReceive,
  fields: fieldErrors,
  busy,
  onReceive,
}: {
  questionnaire: Questionnaire
  canReceive: boolean
  fields: Record<string, string>
  busy: boolean
  onReceive: (answers: Record<string, string>) => void
}) => {
  const [draftAnswers, setDraftAnswers] = useState<Record<string, string>>({})
  const answerErrorFor = (questionId: string) =>
    fieldErrors[`answers.${questionId}`] ?? fieldErrors[questionId]

  if (questionnaire.receivedAt) {
    return (
      <div className="space-y-3 text-sm">
        <p className="text-neutral-600">
          Sent {questionnaire.sentAt.slice(0, 10)}, received {questionnaire.receivedAt.slice(0, 10)}.
        </p>
        <dl className="space-y-2" aria-label="Answers">
          {questionnaire.questions.map((question) => (
            <div key={question.id}>
              <dt className="text-neutral-500">{question.text}</dt>
              <dd className="font-medium">{questionnaire.answers?.[question.id] || 'not answered'}</dd>
            </div>
          ))}
        </dl>
        {questionnaire.inconsistencies.length > 0 && (
          <ul
            className="list-disc space-y-1 rounded-md border border-amber-200 bg-amber-50 p-3 pl-7 text-amber-900"
            aria-label="Inconsistencies"
          >
            {questionnaire.inconsistencies.map((inconsistency) => (
              <li key={inconsistency}>{inconsistency}</li>
            ))}
          </ul>
        )}
      </div>
    )
  }

  return (
    <form
      className="space-y-3 text-sm"
      onSubmit={(event) => {
        event.preventDefault()
        onReceive(
          Object.fromEntries(Object.entries(draftAnswers).filter(([, answer]) => answer.trim() !== '')),
        )
      }}
    >
      <p className="text-neutral-600">
        Sent {questionnaire.sentAt.slice(0, 10)}; awaiting the customer. Record the answers as they come in.
      </p>
      {questionnaire.questions.map((question) => (
        <label key={question.id} className="block">
          {question.text}
          {question.required && <span className="text-neutral-400"> (required)</span>}
          <AnswerInput
            question={question}
            value={draftAnswers[question.id] ?? ''}
            error={answerErrorFor(question.id)}
            disabled={!canReceive || busy}
            onChange={(answer) => setDraftAnswers((current) => ({ ...current, [question.id]: answer }))}
          />
          <FieldError id={`answer-${question.id}-error`} message={answerErrorFor(question.id)} />
        </label>
      ))}
      {canReceive && (
        <Button type="submit" disabled={busy}>
          Record answers
        </Button>
      )}
    </form>
  )
}

const AnswerInput = ({
  question,
  value,
  error,
  disabled,
  onChange,
}: {
  question: Question
  value: string
  error: string | undefined
  disabled: boolean
  onChange: (answer: string) => void
}) => {
  const shared = {
    value,
    disabled,
    className: inputVariants({ invalid: Boolean(error) }),
    ...invalidProps(`answer-${question.id}`, error),
  }
  if (question.type === 'YES_NO')
    return (
      <select {...shared} onChange={(event) => onChange(event.target.value)}>
        <option value="">choose</option>
        <option value="yes">yes</option>
        <option value="no">no</option>
      </select>
    )
  if (question.type === 'DATE')
    return <input {...shared} type="date" onChange={(event) => onChange(event.target.value)} />
  if (question.type === 'AMOUNT')
    return (
      <input
        {...shared}
        inputMode="decimal"
        placeholder="0.00"
        onChange={(event) => onChange(event.target.value)}
      />
    )
  return <textarea {...shared} rows={2} onChange={(event) => onChange(event.target.value)} />
}
