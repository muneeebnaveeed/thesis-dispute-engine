import type { Problem } from '#/api/failure'
import type { components } from '#/api/schema.gen'

type ApiDispute = components['schemas']['Dispute']
type Json = string | number | boolean | null | Json[] | { [key: string]: Json }
export type Dispute = Omit<ApiDispute, 'events'> & {
  events: (Omit<ApiDispute['events'][number], 'payload'> & { payload: Json })[]
}
export type DisputeState = components['schemas']['DisputeState']
export type Outcome<T> = { value: T | null; problem: Problem | null }

// earn the Json type by walking the value instead of asserting it
const toJson = (value: unknown): Json => {
  if (
    value === null ||
    typeof value === 'string' ||
    typeof value === 'number' ||
    typeof value === 'boolean'
  ) {
    return value
  }
  if (Array.isArray(value)) return value.map(toJson)
  if (typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, nested]) => [key, toJson(nested)]))
  }
  return null
}

const disputeView = (apiDispute: ApiDispute): Dispute => ({
  ...apiDispute,
  events: apiDispute.events.map((event) => ({ ...event, payload: toJson(event.payload) })),
})

type ApiResult<T> = { data?: T; error?: Problem }

export const withDisputeView = async (
  result: Promise<ApiResult<ApiDispute>>,
): Promise<ApiResult<Dispute>> => {
  const { data, error } = await result
  if (data !== undefined) return { data: disputeView(data) }
  return error ? { error } : {}
}

export const toOutcome = <T>(result: ApiResult<T>): Outcome<T> =>
  result.error || result.data === undefined
    ? { value: null, problem: result.error ?? null }
    : { value: result.data, problem: null }

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
