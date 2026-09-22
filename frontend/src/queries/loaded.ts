import { fromProblem, type Failure } from '#/api/failure'
import type { Outcome } from '#/api/views'

export type Loaded<T> = { value: T; failure: null } | { value: null; failure: Failure }

// the select of every query: a problem on the wire becomes the failure pages render, once, here
export const classified = <T>(outcome: Outcome<T>): Loaded<T> =>
  outcome.problem
    ? { value: null, failure: fromProblem(outcome.problem) }
    : { value: outcome.value, failure: null }
