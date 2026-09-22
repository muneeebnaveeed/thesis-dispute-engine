import { useState } from 'react'

import type { Dispute } from '#/api/views'
import { submitTo, submitting, useAppForm } from '#/forms/app-form'
import { useQuestionnaireSuggestion } from '#/queries/suggestions'

type Questionnaire = NonNullable<Dispute['questionnaire']>

const YES_NO = [
  { value: 'yes', label: 'yes' },
  { value: 'no', label: 'no' },
]

export const QuestionnairePanel = ({
  questionnaire,
  disputeId,
  canReceive,
  busy,
  onReceive,
}: {
  questionnaire: Questionnaire
  disputeId: string
  canReceive: boolean
  busy: boolean
  onReceive: (
    answers: Record<string, string>,
    suggestions: Record<string, { value: string; probability: number }>,
  ) => Promise<unknown>
}) => {
  // ids the model filled in, cleared per field as soon as the analyst touches it
  // what was proposed travels with the answers, so acceptance can be counted per question
  const [proposed, setProposed] = useState<Record<string, { value: string; probability: number }>>({})
  const suggestion = useQuestionnaireSuggestion()
  const answersForm = useAppForm({
    defaultValues: {
      answers: Object.fromEntries(questionnaire.questions.map((question) => [question.id, ''])),
    },
    onSubmit: ({ value, formApi }) =>
      submitTo(formApi, () =>
        onReceive(
          Object.fromEntries(Object.entries(value.answers).filter(([, answer]) => answer.trim() !== '')),
          proposed,
        ),
      ),
  })

  const readReply = async (reply: string) => {
    const trimmed = reply.trim()
    if (!trimmed) return
    const read = await suggestion.mutateAsync({ disputeId, reply: trimmed }).catch(() => null)
    if (!read) return
    const filled = Object.entries(read.answers)
    for (const [id, answer] of filled) answersForm.setFieldValue(`answers.${id}`, answer.value)
    setProposed(Object.fromEntries(filled))
  }

  if (questionnaire.receivedAt) {
    return (
      <div className="space-y-2 text-[11px]">
        <p className="text-muted-foreground">
          Sent {questionnaire.sentAt.slice(0, 10)}, received {questionnaire.receivedAt.slice(0, 10)}.
        </p>
        <dl className="space-y-2" aria-label="Answers">
          {questionnaire.questions.map((question) => (
            <div key={question.id}>
              <dt className="text-muted-foreground">{question.text}</dt>
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
    <form className="space-y-2 text-[11px]" onSubmit={submitting(answersForm)}>
      <p className="text-muted-foreground">
        Sent {questionnaire.sentAt.slice(0, 10)}; awaiting the customer. Record the answers as they come in.
      </p>
      {canReceive && (
        <label className="field">
          <span className="field-label">
            What the customer wrote back
            <span className="text-muted-foreground"> (optional, fills in the yes and no answers)</span>
          </span>
          <span className="field-control">
            <textarea
              rows={3}
              disabled={busy}
              className="mt-1 block w-full border border-input bg-card px-1 py-[2px] text-[11px] shadow-[inset_1px_1px_2px_rgb(0_0_0/0.12)] focus:border-ring focus:outline-none"
              placeholder="paste the reply, in any language"
              onBlur={(event) => void readReply(event.target.value)}
            />
          </span>
        </label>
      )}
      {questionnaire.questions.map((question) => {
        const label = (
          <>
            {question.text}
            {question.required && <span className="text-muted-foreground"> (required)</span>}
          </>
        )
        const disabled = !canReceive || busy
        return (
          <answersForm.AppField key={question.id} name={`answers.${question.id}`}>
            {(field) => {
              if (question.type === 'YES_NO')
                return (
                  <field.SelectField
                    label={
                      <>
                        {label}
                        {question.id in proposed && (
                          <span className="text-muted-foreground"> (suggested)</span>
                        )}
                      </>
                    }
                    options={YES_NO}
                    placeholder="choose"
                    disabled={disabled}
                    onChange={() => setProposed(({ [question.id]: _dropped, ...rest }) => rest)}
                  />
                )
              if (question.type === 'DATE')
                return <field.TextField label={label} type="date" disabled={disabled} />
              if (question.type === 'AMOUNT')
                return (
                  <field.TextField label={label} inputMode="decimal" placeholder="0.00" disabled={disabled} />
                )
              return <field.TextareaField label={label} rows={2} disabled={disabled} />
            }}
          </answersForm.AppField>
        )
      })}
      {canReceive && (
        <answersForm.AppForm>
          <answersForm.SubmitButton busy={busy}>Record answers</answersForm.SubmitButton>
        </answersForm.AppForm>
      )}
    </form>
  )
}
