import { useEffect, useState } from 'react'

import { describe, reference, retryAfter, type Failure } from '#/api/failure'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/cn'

export const FailureBanner = ({ failure, onRetry }: { failure: Failure; onRetry?: () => void }) => {
  const { title, hint } = describe(failure)
  const retryAfterSeconds = retryAfter(failure)
  const secondsUntilRetry = useCountdown(retryAfterSeconds)
  const supportReference = reference(failure)
  const personCanFixIt = failure.kind === 'validation' || failure.kind === 'conflict'
  return (
    <div
      role="alert"
      className={cn(
        'rounded-md border p-4 text-sm',
        personCanFixIt
          ? 'border-amber-200 bg-amber-50 text-amber-900'
          : 'border-red-200 bg-red-50 text-red-900',
      )}
    >
      <p className="font-medium">{title}</p>
      <p className="mt-1">{hint}</p>
      {onRetry && retryAfterSeconds > 0 && (
        <p className="mt-2">
          {secondsUntilRetry > 0 ? (
            <span>Retry available in {secondsUntilRetry}s.</span>
          ) : (
            <Button variant="link" size="bare" onClick={onRetry}>
              Retry now
            </Button>
          )}
        </p>
      )}
      {onRetry && retryAfterSeconds === 0 && failure.kind === 'unreachable' && (
        <Button variant="link" size="bare" onClick={onRetry} className="mt-2">
          Retry
        </Button>
      )}
      {supportReference && <p className="mt-2 text-xs opacity-80">Reference {supportReference}.</p>}
    </div>
  )
}

const useCountdown = (seconds: number): number => {
  const [secondsLeft, setSecondsLeft] = useState(seconds)
  useEffect(() => {
    setSecondsLeft(seconds) // eslint-disable-line react/set-state-in-effect -- restart on a new wait
    if (seconds <= 0) return undefined
    const tick = setInterval(() => setSecondsLeft((left) => (left > 0 ? left - 1 : 0)), 1000)
    return () => clearInterval(tick)
  }, [seconds])
  return secondsLeft
}

export const FieldError = ({ id, message }: { id: string; message?: string | undefined }) => {
  if (!message) return null
  return (
    <p id={id} className="mt-1 text-xs text-red-700">
      {message}
    </p>
  )
}
