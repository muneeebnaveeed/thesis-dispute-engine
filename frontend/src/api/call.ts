import { classify, isRetryable, retryAfter, type Failure } from './failure'

type Outcome<T> = { data?: T | undefined; error?: unknown; response?: Response | undefined }

/**
 * Runs an openapi-fetch call and returns either data or a classified failure. Thrown fetch errors (network down,
 * aborted) become failures too. When `idempotent` is true the call is repeated once after a short retryable
 * failure: every mutation carries an Idempotency-Key, so the repeat cannot double-apply.
 */
export async function call<T>(
  run: () => Promise<Outcome<T>>,
  opts: { idempotent?: boolean; maxWaitSeconds?: number } = {},
): Promise<{ data: T; failure: null } | { data: null; failure: Failure }> {
  const once = async (): Promise<{ data: T; failure: null } | { data: null; failure: Failure }> => {
    let out: Outcome<T>
    try {
      out = await run()
    } catch (thrown) {
      return { data: null, failure: classify({ thrown }) ?? { kind: 'unreachable', message: 'unreachable' } }
    }
    const failure = classify(
      out.response ? { error: out.error, response: out.response } : { error: out.error },
    )
    if (failure) return { data: null, failure }
    // A 2xx without a body (204 on delete) is a success whose data is simply nothing.
    if (out.data === undefined && !(out.response?.ok ?? false))
      return {
        data: null,
        failure: { kind: 'unexpected', status: out.response?.status ?? 0, message: 'empty response' },
      }
    // openapi-fetch types a bodiless 2xx as undefined, which is T for exactly those routes.
    // oxlint-disable-next-line typescript/no-unsafe-type-assertion
    return { data: out.data as T, failure: null }
  }
  const first = await once()
  if (first.failure && opts.idempotent && isRetryable(first.failure)) {
    const wait = retryAfter(first.failure)
    if (wait <= (opts.maxWaitSeconds ?? 5)) {
      await new Promise((r) => setTimeout(r, wait * 1000))
      return once()
    }
  }
  return first
}
