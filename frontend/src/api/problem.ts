import type { components } from './schema.gen'

export type Problem = components['schemas']['Problem']
export type FieldError = components['schemas']['FieldError']

/** Whether the caller may retry the same or a re-evaluated request; the server decides, the UI only reads it. */
export function isRetryable(p: Problem): boolean {
  return p.retryable
}

/** Field errors keyed by JSON pointer without the leading slash, so `errors['transactionId']` reads naturally. */
export function fieldErrors(p: Problem): Record<string, string> {
  const out: Record<string, string> = {}
  for (const e of p.errors ?? []) out[e.field.replace(/^\//, '')] = e.message
  return out
}

/** The one line to show a person; internal problems already carry a reference instead of a cause. */
export function problemMessage(p: Problem): string {
  return p.detail ?? p.title
}
