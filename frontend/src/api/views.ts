import type { Problem } from '#/api/failure'
import type { components } from '#/api/schema.gen'

type ApiDispute = components['schemas']['Dispute']
type Json = string | number | boolean | null | Json[] | { [key: string]: Json }
export type Dispute = Omit<ApiDispute, 'events'> & {
  events: (Omit<ApiDispute['events'][number], 'payload'> & { payload: Json })[]
}
export type DisputeState = components['schemas']['DisputeState']
// what every server function returns: openapi-fetch's own result, the Response reduced to its status line on the
// way to the browser by response-adapter.ts
export type ApiResult<T> =
  | { data: T; error?: undefined; response: Response }
  | { data?: undefined; error: Problem; response: Response }

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

export const withDisputeView = async (
  result: Promise<ApiResult<ApiDispute>>,
): Promise<ApiResult<Dispute>> => {
  const settled = await result
  return settled.error ? settled : { data: disputeView(settled.data), response: settled.response }
}
