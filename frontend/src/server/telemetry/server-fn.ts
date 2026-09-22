import { SpanStatusCode } from '@opentelemetry/api'
import { isRedirect } from '@tanstack/react-router'
import { createMiddleware } from '@tanstack/react-start'

import { log } from './log'
import { SECONDS_BUCKETS, meter, tracer } from './sdk'

const ATTR_FN = 'workbench.server_fn'
const ATTR_OUTCOME = 'workbench.outcome'

const outcomeOf = (result: unknown): string => {
  if (typeof result !== 'object' || result === null || !('result' in result)) return 'ok'
  const returned = result.result
  if (typeof returned === 'object' && returned !== null && 'error' in returned) {
    const problem = returned.error
    if (
      typeof problem === 'object' &&
      problem !== null &&
      'code' in problem &&
      typeof problem.code === 'string'
    ) {
      return problem.code
    }
  }
  return 'ok'
}

const fnDuration = () =>
  meter().createHistogram('workbench.server_fn.duration', {
    unit: 's',
    description: 'Time a server function took, by function and outcome (ok or the problem code)',
    advice: { explicitBucketBoundaries: SECONDS_BUCKETS },
  })

// one span per server function call, inside the request span; the outcome is the API's problem code when there is
// one, so a refused write is visible on the trace without opening the body
export const traced = createMiddleware({ type: 'function' }).server(async ({ next, serverFnMeta }) => {
  const name = serverFnMeta.name
  return tracer().startActiveSpan(`serverFn ${name}`, { attributes: { [ATTR_FN]: name } }, async (span) => {
    const startedAt = performance.now()
    let outcome = 'ok'
    try {
      const handled = await next()
      outcome = outcomeOf(handled)
      span.setAttribute(ATTR_OUTCOME, outcome)
      if (outcome !== 'ok') log.warn('server function refused', { [ATTR_FN]: name, [ATTR_OUTCOME]: outcome })
      return handled
    } catch (thrown) {
      outcome = isRedirect(thrown) ? 'sign-in' : 'threw'
      span.setAttribute(ATTR_OUTCOME, outcome)
      if (!isRedirect(thrown)) {
        span.setStatus({
          code: SpanStatusCode.ERROR,
          message: thrown instanceof Error ? thrown.message : 'threw',
        })
        if (thrown instanceof Error) span.recordException(thrown)
        log.error('server function threw', {
          [ATTR_FN]: name,
          error: thrown instanceof Error ? thrown.message : String(thrown),
        })
      }
      throw thrown
    } finally {
      fnDuration().record((performance.now() - startedAt) / 1000, {
        [ATTR_FN]: name,
        [ATTR_OUTCOME]: outcome,
      })
      span.end()
    }
  })
})
