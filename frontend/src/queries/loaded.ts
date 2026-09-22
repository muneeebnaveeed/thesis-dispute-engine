import { fromProblem, type Failure } from '#/api/failure'
import type { ApiResult } from '#/api/views'

export type Loaded<T> = { value: T; failure: null } | { value: null; failure: Failure }

// the select of every query: a problem on the wire becomes the failure pages render, once, here
export const classified = <T>(result: ApiResult<T>): Loaded<T> =>
  result.error ? { value: null, failure: fromProblem(result.error) } : { value: result.data, failure: null }
