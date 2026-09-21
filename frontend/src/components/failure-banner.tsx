import { useEffect, useState } from 'react'

import { describe, reference, retryAfter, type Failure } from '#/api/failure'

/**
 * One banner for every kind of failure: what happened, what to do, a countdown when waiting helps, a retry button
 * when the caller can retry, the reference when support might need it. Field-level messages belong next to the
 * field (FieldError), not here.
 */
export function FailureBanner({ failure, onRetry }: { failure: Failure; onRetry?: () => void }) {
  const { title, hint } = describe(failure)
  const wait = retryAfter(failure)
  const left = useCountdown(wait)
  const ref = reference(failure)
  const tone = failure.kind === 'validation' || failure.kind === 'conflict' ? 'amber' : 'red'
  const box =
    tone === 'amber' ? 'border-amber-200 bg-amber-50 text-amber-900' : 'border-red-200 bg-red-50 text-red-900'
  return (
    <div role="alert" className={`rounded-md border p-4 text-sm ${box}`}>
      <p className="font-medium">{title}</p>
      <p className="mt-1">{hint}</p>
      {onRetry && wait > 0 && (
        <p className="mt-2">
          {left > 0 ? (
            <span>Retry available in {left}s.</span>
          ) : (
            <button type="button" onClick={onRetry} className="underline">
              Retry now
            </button>
          )}
        </p>
      )}
      {onRetry && wait === 0 && failure.kind === 'unreachable' && (
        <button type="button" onClick={onRetry} className="mt-2 underline">
          Retry
        </button>
      )}
      {ref && <p className="mt-2 text-xs opacity-80">Reference {ref}.</p>}
    </div>
  )
}

// Seconds remaining: a plain ticking counter that restarts whenever a new wait arrives. The tick lives in an
// effect (an external timer), the value in state; nothing impure runs during render.
function useCountdown(seconds: number): number {
  const [left, setLeft] = useState(seconds)
  useEffect(() => {
    setLeft(seconds) // eslint-disable-line react/set-state-in-effect -- restart on a new wait
    if (seconds <= 0) return undefined
    const t = setInterval(() => setLeft((s) => (s > 0 ? s - 1 : 0)), 1000)
    return () => clearInterval(t)
  }, [seconds])
  return left
}

/** Inline message under a form control; id pairs with the control's aria-describedby. */
export function FieldError({ id, message }: { id: string; message?: string | undefined }) {
  if (!message) return null
  return (
    <p id={id} className="mt-1 text-xs text-red-700">
      {message}
    </p>
  )
}
