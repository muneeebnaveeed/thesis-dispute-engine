import type { components } from './schema.gen'

export type Problem = components['schemas']['Problem']
type ErrorCode = components['schemas']['ErrorCode']

export type Failure =
  | { kind: 'validation'; problem: Problem; fields: Record<string, string> }
  | { kind: 'conflict'; problem: Problem; allowedEvents: string[] }
  | { kind: 'not-found'; problem: Problem }
  | { kind: 'signed-out'; problem: Problem }
  | { kind: 'forbidden'; problem: Problem }
  | { kind: 'declined'; problem: Problem }
  | { kind: 'rate-limited'; problem: Problem; retryAfterSeconds: number }
  | { kind: 'unavailable'; problem: Problem; retryAfterSeconds: number }
  | { kind: 'internal'; problem: Problem }
  | { kind: 'unreachable'; message: string }
  | { kind: 'unexpected'; status: number; message: string }

export const isRetryable = (failure: Failure): boolean =>
  failure.kind === 'rate-limited' || failure.kind === 'unavailable' || failure.kind === 'unreachable'

export const retryAfter = (failure: Failure): number => {
  if (failure.kind === 'rate-limited' || failure.kind === 'unavailable') return failure.retryAfterSeconds
  return failure.kind === 'unreachable' ? 3 : 0
}

export const reference = (failure: Failure): string | null =>
  'problem' in failure && failure.problem.requestId !== 'local' ? failure.problem.requestId : null

// "/transactionId", "body.transactionId" and "payload/liability" all become dotted bare paths
export const fieldErrors = (problem: Problem): Record<string, string> => {
  const byField: Record<string, string> = {}
  for (const fieldError of problem.errors ?? []) {
    const fieldPath = fieldError.field.replace(/^\/|^(body|query|path|header)\./, '').replace(/\//g, '.')
    byField[fieldPath] ??= fieldError.message
  }
  return byField
}

// the code is the only thing the contract promises to keep stable; never branch on title or detail.
// `satisfies Record<ErrorCode, ...>` fails the build when the contract gains a code this table does not place.
const FAILURE_KIND_BY_CODE = {
  'contract-violation': 'validation',
  'malformed-request': 'validation',
  'no-regime': 'validation',
  'unknown-regime': 'validation',
  'unknown-reason': 'validation',
  'invalid-answers': 'validation',
  'invalid-fields': 'validation',
  'unknown-template': 'validation',
  'invalid-template-override': 'validation',
  'attachment-refused': 'validation',
  'attachment-unknown': 'validation',
  'invalid-liability': 'validation',
  'invalid-settlement': 'validation',
  'risk-hold': 'validation',
  'invalid-transition': 'conflict',
  'appeals-exhausted': 'conflict',
  'concurrent-update': 'conflict',
  'idempotency-key-reuse': 'conflict',
  'not-resendable': 'conflict',
  'not-found': 'not-found',
  unauthenticated: 'signed-out',
  forbidden: 'forbidden',
  'core-declined': 'declined',
  'rate-limited': 'rate-limited',
  unavailable: 'unavailable',
  internal: 'internal',
} as const satisfies Record<ErrorCode, Failure['kind']>

// ledger refusals name the payload fact they are about, so the form can point at the input
const PAYLOAD_FACT_BY_CODE: Partial<Record<ErrorCode, string>> = {
  'invalid-liability': 'liability',
  'invalid-settlement': 'settlement',
  'risk-hold': 'riskOverride',
}

const validationFields = (problem: Problem): Record<string, string> => {
  const fields = fieldErrors(problem)
  if (problem.code === 'attachment-refused' || problem.code === 'attachment-unknown') {
    return { attachments: problem.detail ?? problem.title, ...fields }
  }
  const payloadFact = PAYLOAD_FACT_BY_CODE[problem.code]
  if (payloadFact) {
    return { ...fields, [payloadFact]: fields[`payload.${payloadFact}`] ?? problem.detail ?? problem.title }
  }
  return fields
}

export const fromProblem = (problem: Problem): Failure => {
  // a code this build does not know (newer server) reads as undefined at runtime; internal keeps the reference visible
  const kind: (typeof FAILURE_KIND_BY_CODE)[ErrorCode] | undefined = FAILURE_KIND_BY_CODE[problem.code]
  switch (kind) {
    case 'validation':
      return { kind, problem, fields: validationFields(problem) }
    case 'conflict':
      return { kind, problem, allowedEvents: problem.allowedEvents ?? [] }
    case 'not-found':
    case 'signed-out':
    case 'forbidden':
    case 'declined':
      return { kind, problem }
    case 'rate-limited':
      return { kind, problem, retryAfterSeconds: problem.retryAfterSeconds ?? 60 }
    case 'unavailable':
      return { kind, problem, retryAfterSeconds: problem.retryAfterSeconds ?? 5 }
    case 'internal':
    default:
      return { kind: 'internal', problem }
  }
}

export const classify = (input: {
  error?: unknown
  response?: Response | undefined
  thrown?: unknown
}): Failure | null => {
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

const isProblem = (body: unknown): body is Problem =>
  !!body &&
  typeof body === 'object' &&
  typeof (body as { code?: unknown }).code === 'string' &&
  typeof (body as { status?: unknown }).status === 'number' &&
  typeof (body as { retryable?: unknown }).retryable === 'boolean'

export const localValidation = (fields: Record<string, string>): Failure => {
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

// never the raw payload, never internal detail beyond the reference
export const describe = (failure: Failure): { title: string; hint: string } => {
  switch (failure.kind) {
    case 'validation':
      return {
        title: failure.problem.detail ?? 'Some fields need attention.',
        hint: 'Fix the highlighted fields and try again.',
      }
    case 'conflict':
      return {
        title: failure.problem.detail ?? 'The dispute changed in the meantime.',
        hint: failure.allowedEvents.length
          ? `Allowed now: ${failure.allowedEvents.join(', ')}.`
          : 'Reload to see the current state.',
      }
    case 'not-found':
      return {
        title: failure.problem.detail ?? 'Not found.',
        hint: 'Check the identifier; it may belong to another organisation.',
      }
    case 'signed-out':
      return { title: 'Your session has ended.', hint: 'Sign in again to continue.' }
    case 'forbidden':
      return {
        title: failure.problem.detail ?? 'Not allowed.',
        hint: 'This needs a role your account does not have.',
      }
    case 'declined':
      return {
        title: failure.problem.detail ?? 'The banking core declined the posting.',
        hint: 'Nothing was recorded. Check the amount and the account with your core banking team, then try again.',
      }
    case 'rate-limited':
      return {
        title: 'Too many requests for your organisation right now.',
        hint: `Try again in ${failure.retryAfterSeconds} seconds.`,
      }
    case 'unavailable':
      return {
        title: 'The service is temporarily unavailable.',
        hint: `Try again in ${failure.retryAfterSeconds} seconds.`,
      }
    case 'internal':
      return {
        title: 'Something went wrong on our side.',
        hint: 'Quote the reference if you contact support.',
      }
    case 'unreachable':
      return { title: 'Cannot reach the service.', hint: 'Check your connection and try again.' }
    case 'unexpected':
      return { title: failure.message, hint: 'Reload the page; if it persists, contact support.' }
    default:
      return { title: 'Something went wrong.', hint: 'Reload the page.' }
  }
}
