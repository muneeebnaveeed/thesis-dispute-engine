import type { Api } from '#/api/client'
import type { Problem } from '#/api/problem'
import type { components } from '#/api/schema.gen'
import { ApplyEventRequest, CreateDisputeRequest } from '#/api/schemas.gen'
import { validate } from '#/api/validate'

type ApiDispute = components['schemas']['Dispute']
export type Json = string | number | boolean | null | Json[] | { [key: string]: Json }
// Start proves server-function results are serialisable; the contract's free-form payload needs an explicit JSON type.
export type Dispute = Omit<ApiDispute, 'events'> & {
  events: (Omit<ApiDispute['events'][number], 'payload'> & { payload: Json })[]
}
// A plain shape rather than a discriminated union: the same check does not narrow unions well.
export type Outcome<T> = { value: T | null; problem: Problem | null }

// The API only ever returns JSON; this walks the value so the type is earned, not asserted.
function asJson(v: unknown): Json {
  if (v === null || typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') return v
  if (Array.isArray(v)) return v.map(asJson)
  if (typeof v === 'object') return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, asJson(x)]))
  return null
}
const view = (d: ApiDispute): Dispute => ({
  ...d,
  events: d.events.map((e) => ({ ...e, payload: asJson(e.payload) })),
})
const ok = (value: ApiDispute): Outcome<Dispute> => ({ value: view(value), problem: null })
const failed = (problem: Problem): Outcome<Dispute> => ({ value: null, problem })

// Structural failures caught here are shaped like the server's own contract-violation problem so pages have one path.
function invalid(errors: Record<string, string>): Problem {
  return {
    type: 'urn:dispute-engine:error:contract-violation',
    title: 'The request is not valid',
    status: 400,
    code: 'contract-violation',
    retryable: false,
    requestId: 'local',
    errors: Object.entries(errors).map(([field, message]) => ({ field: `/${field}`, message })),
  }
}

export async function fetchDispute(api: Api, id: string): Promise<Outcome<Dispute>> {
  const { data, error } = await api.GET('/disputes/{disputeId}', { params: { path: { disputeId: id } } })
  return error ? failed(error) : ok(data)
}

export async function openDispute(api: Api, input: unknown): Promise<Outcome<Dispute>> {
  const v = validate(CreateDisputeRequest, input)
  if (!v.ok) return failed(invalid(v.errors))
  const { data, error } = await api.POST('/disputes', {
    body: v.value,
    headers: { 'Idempotency-Key': crypto.randomUUID() },
  })
  return error ? failed(error) : ok(data)
}

export async function applyDisputeEvent(
  api: Api,
  disputeId: string,
  body: unknown,
): Promise<Outcome<Dispute>> {
  const v = validate(ApplyEventRequest, body)
  if (!v.ok) return failed(invalid(v.errors))
  const { data, error } = await api.POST('/disputes/{disputeId}/events', {
    params: { path: { disputeId } },
    body: v.value,
    headers: { 'Idempotency-Key': crypto.randomUUID() },
  })
  return error ? failed(error) : ok(data)
}
