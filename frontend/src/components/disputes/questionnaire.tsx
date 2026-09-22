import type { Dispute } from '#/api/views'
import { submitTo, submitting, useAppForm } from '#/forms/app-form'

type Questionnaire = NonNullable<Dispute['questionnaire']>

const YES_NO = [
  { value: 'yes', label: 'yes' },
  { value: 'no', label: 'no' },
]

export const QuestionnairePanel = ({
  questionnaire,
  canReceive,
  busy,
  onReceive,
}: {
  questionnaire: Questionnaire
  canReceive: boolean
  busy: boolean
  onReceive: (answers: Record<string, string>) => Promise<unknown>
}) => {
  const answersForm = useAppForm({
    defaultValues: {
      answers: Object.fromEntries(questionnaire.questions.map((question) => [question.id, ''])),
    },
    onSubmit: ({ value, formApi }) =>
      submitTo(formApi, () =>
        onReceive(
          Object.fromEntries(Object.entries(value.answers).filter(([, answer]) => answer.trim() !== '')),
        ),
      ),
  })

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
                    label={label}
                    options={YES_NO}
                    placeholder="choose"
                    disabled={disabled}
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
