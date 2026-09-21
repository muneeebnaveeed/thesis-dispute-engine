import { fieldErrors, isRetryable, problemMessage, type Problem } from '#/api/problem'

/** Renders a problem+json the way the contract intends: message, retry hint, field errors. Never the raw payload. */
export function ProblemBanner({ problem }: { problem: Problem }) {
  const fields = Object.entries(fieldErrors(problem))
  return (
    <div role="alert" className="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-900">
      <p className="font-medium">{problemMessage(problem)}</p>
      {fields.length > 0 && (
        <ul className="mt-2 list-disc pl-5">
          {fields.map(([field, message]) => (
            <li key={field}>
              <span className="font-mono">{field}</span>: {message}
            </li>
          ))}
        </ul>
      )}
      <p className="mt-2 text-xs text-red-700">
        {isRetryable(problem) ? 'You can try again.' : 'Trying again will not help.'} Reference{' '}
        {problem.requestId}.
      </p>
    </div>
  )
}
