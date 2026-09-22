import type { Problem } from '#/api/failure'
import type { components } from '#/api/schema.gen'

// The API's own types plus the two adjustments the frontend needs: a JSON type for the free-form event payload
// (Start proves route data is serialisable) and one outcome shape for success and failure alike.
type ApiDispute = components['schemas']['Dispute']
export type Json = string | number | boolean | null | Json[] | { [key: string]: Json }
export type Dispute = Omit<ApiDispute, 'events'> & {
  events: (Omit<ApiDispute['events'][number], 'payload'> & { payload: Json })[]
}
export type DisputePage = components['schemas']['DisputePage']
export type DisputeState = components['schemas']['DisputeState']
export type Outcome<T> = { value: T | null; problem: Problem | null }

// The API only ever returns JSON; this walks the value so the type is earned, not asserted.
const asJson = (v: unknown): Json => {
  if (v === null || typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') return v
  if (Array.isArray(v)) return v.map(asJson)
  if (typeof v === 'object') return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, asJson(x)]))
  return null
}

export const view = (d: ApiDispute): Dispute => ({
  ...d,
  events: d.events.map((e) => ({ ...e, payload: asJson(e.payload) })),
})

/** Turns an openapi-fetch result into an outcome, through a mapper when the value needs adjusting (the dispute view). */
type ToOutcome = {
  <T>(res: { data?: T; error?: Problem }): Outcome<T>
  <T, V>(res: { data?: T; error?: Problem }, map: (t: T) => V): Outcome<V>
}
export const outcome: ToOutcome = <T, V>(
  res: { data?: T; error?: Problem },
  map?: (t: T) => V,
): Outcome<T | V> => {
  if (res.error || res.data === undefined) return { value: null, problem: res.error ?? null }
  return { value: map ? map(res.data) : res.data, problem: null }
}

/** No session: the same problem the API returns for a missing credential. */
export const unauthenticated = <T>(): Outcome<T> => ({
  value: null,
  problem: {
    type: 'urn:dispute-engine:error:unauthenticated',
    title: 'Authentication required',
    status: 401,
    code: 'unauthenticated',
    retryable: false,
    requestId: 'local',
    detail: 'Sign in to continue.',
  },
})
