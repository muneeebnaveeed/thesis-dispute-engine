import type { components } from './schema.gen'

export type Problem = components['schemas']['Problem']
export type ErrorCode = components['schemas']['ErrorCode']

/**
 * Everything a call can fail with, in one shape the UI can branch on. Kinds group the contract's codes by what a
 * person should do next; the code itself stays available for anything finer.
 */
export type Failure =
  | { kind: 'validation'; problem: Problem; fields: Record<string, string> }
  | { kind: 'conflict'; problem: Problem; allowedEvents: string[] }
  | { kind: 'not-found'; problem: Problem }
  | { kind: 'signed-out'; problem: Problem }
  | { kind: 'forbidden'; problem: Problem }
  | { kind: 'rate-limited'; problem: Problem; retryAfterSeconds: number }
  | { kind: 'unavailable'; problem: Problem; retryAfterSeconds: number }
  | { kind: 'internal'; problem: Problem }
  | { kind: 'unreachable'; message: string }
  | { kind: 'unexpected'; status: number; message: string }

/** True for kinds where the same request may succeed later without the person changing anything. */
export function isRetryable(f: Failure): boolean {
  return f.kind === 'rate-limited' || f.kind === 'unavailable' || f.kind === 'unreachable'
}

/** Seconds to wait before a retry makes sense; 0 when a retry is pointless or may happen at once. */
export function retryAfter(f: Failure): number {
  if (f.kind === 'rate-limited' || f.kind === 'unavailable') return f.retryAfterSeconds
  return f.kind === 'unreachable' ? 3 : 0
}

/** The problem's request reference when there is one; the thing to quote to support. */
export function reference(f: Failure): string | null {
  return 'problem' in f && f.problem.requestId !== 'local' ? f.problem.requestId : null
}

/** Field errors keyed by field name: "/transactionId" and "body.transactionId" both become "transactionId". */
export function fieldErrors(p: Problem): Record<string, string> {
  const out: Record<string, string> = {}
  for (const e of p.errors ?? []) {
    const key = e.field.replace(/^\/|^(body|query|path|header)\./, '').replace(/\//g, '.')
    out[key] ??= e.message
  }
  return out
}

/** Classifies a problem+json body by its code, which is the only thing the contract promises to keep stable. */
export function fromProblem(p: Problem): Failure {
  switch (p.code) {
    case 'contract-violation':
    case 'malformed-request':
      return { kind: 'validation', problem: p, fields: fieldErrors(p) }
    case 'invalid-transition':
    case 'appeals-exhausted':
    case 'concurrent-update':
    case 'idempotency-key-reuse':
      return { kind: 'conflict', problem: p, allowedEvents: p.allowedEvents ?? [] }
    case 'not-found':
      return { kind: 'not-found', problem: p }
    case 'unauthenticated':
      return { kind: 'signed-out', problem: p }
    case 'forbidden':
      return { kind: 'forbidden', problem: p }
    case 'rate-limited':
      return { kind: 'rate-limited', problem: p, retryAfterSeconds: p.retryAfterSeconds ?? 60 }
    case 'unavailable':
      return { kind: 'unavailable', problem: p, retryAfterSeconds: p.retryAfterSeconds ?? 5 }
    case 'no-regime':
    case 'unknown-regime':
      return { kind: 'validation', problem: p, fields: fieldErrors(p) }
    case 'internal':
      return { kind: 'internal', problem: p }
    default:
      // A code this build does not know: newer server. Treat as internal so the reference still shows.
      return { kind: 'internal', problem: p }
  }
}

/** Classifies whatever openapi-fetch handed back: a typed problem, a non-problem response, or a thrown fetch error. */
export function classify(input: {
  error?: unknown
  response?: Response | undefined
  thrown?: unknown
}): Failure | null {
  if (input.thrown !== undefined) {
    const message = input.thrown instanceof Error ? input.thrown.message : 'the service could not be reached'
    return { kind: 'unreachable', message }
  }
  if (input.error === undefined || input.error === null) return null
  if (isProblem(input.error)) return fromProblem(input.error)
  const status = input.response?.status ?? 0
  return {
    kind: 'unexpected',
    status,
    message: status ? `unexpected response (${status})` : 'unexpected response',
  }
}

export function isProblem(v: unknown): v is Problem {
  return (
    !!v &&
    typeof v === 'object' &&
    typeof (v as { code?: unknown }).code === 'string' &&
    typeof (v as { status?: unknown }).status === 'number' &&
    typeof (v as { retryable?: unknown }).retryable === 'boolean'
  )
}

/** A locally produced validation failure, shaped like the server's so pages have one path. */
export function localValidation(fields: Record<string, string>): Failure {
  const problem: Problem = {
    type: 'urn:dispute-engine:error:contract-violation',
    title: 'The request is not valid',
    status: 400,
    code: 'contract-violation',
    retryable: false,
    requestId: 'local',
    detail: 'Some fields need attention.',
    errors: Object.entries(fields).map(([field, message]) => ({ field: `/${field}`, message })),
  }
  return { kind: 'validation', problem, fields }
}

/** What to tell a person, per kind. Never the raw payload; never internal detail beyond the reference. */
export function describe(f: Failure): { title: string; hint: string } {
  switch (f.kind) {
    case 'validation':
      return {
        title: f.problem.detail ?? 'Some fields need attention.',
        hint: 'Fix the highlighted fields and try again.',
      }
    case 'conflict':
      return {
        title: f.problem.detail ?? 'The dispute changed in the meantime.',
        hint: f.allowedEvents.length
          ? `Allowed now: ${f.allowedEvents.join(', ')}.`
          : 'Reload to see the current state.',
      }
    case 'not-found':
      return {
        title: f.problem.detail ?? 'Not found.',
        hint: 'Check the identifier; it may belong to another organisation.',
      }
    case 'signed-out':
      return { title: 'Your session has ended.', hint: 'Sign in again to continue.' }
    case 'forbidden':
      return {
        title: f.problem.detail ?? 'Not allowed.',
        hint: 'This needs a role your account does not have.',
      }
    case 'rate-limited':
      return {
        title: 'Too many requests for your organisation right now.',
        hint: `Try again in ${f.retryAfterSeconds} seconds.`,
      }
    case 'unavailable':
      return {
        title: 'The service is temporarily unavailable.',
        hint: `Try again in ${f.retryAfterSeconds} seconds.`,
      }
    case 'internal':
      return {
        title: 'Something went wrong on our side.',
        hint: 'Quote the reference if you contact support.',
      }
    case 'unreachable':
      return { title: 'Cannot reach the service.', hint: 'Check your connection and try again.' }
    case 'unexpected':
      return { title: f.message, hint: 'Reload the page; if it persists, contact support.' }
    default:
      return { title: 'Something went wrong.', hint: 'Reload the page.' }
  }
}
