import { useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query'
import { useState } from 'react'

import { classify, type Failure } from '#/api/failure'
import type { Outcome } from '#/api/views'

/**
 * A write through a server function that answers with an outcome: the problem, if any, is classified the way
 * every page shows failures; on success the queries the change touches are invalidated. A thrown error (the
 * server function itself failing, or the network) is classified too, so nothing escapes as a blank.
 */
export const useServerMutation = <TVars, TData>(
  run: (vars: TVars) => Promise<Outcome<TData>>,
  opts: {
    invalidates?: (vars: TVars, data: TData) => QueryKey[]
    onSuccess?: (data: TData, vars: TVars) => void | Promise<void>
  } = {},
) => {
  const queryClient = useQueryClient()
  const [failure, setFailure] = useState<Failure | null>(null)
  const mutation = useMutation({
    mutationFn: async (vars: TVars): Promise<TData> => {
      setFailure(null)
      let out: Outcome<TData>
      try {
        out = await run(vars)
      } catch (thrown) {
        const f = classify({ thrown }) ?? {
          kind: 'unreachable' as const,
          message: 'the server could not be reached',
        }
        setFailure(f)
        throw f
      }
      if (out.problem || out.value === null) {
        const f = classify({ error: out.problem }) ?? {
          kind: 'unexpected' as const,
          status: 0,
          message: 'no data',
        }
        setFailure(f)
        throw f
      }
      return out.value
    },
    onSuccess: async (data, vars) => {
      await Promise.all(
        (opts.invalidates?.(vars, data) ?? []).map((queryKey) => queryClient.invalidateQueries({ queryKey })),
      )
      await opts.onSuccess?.(data, vars)
    },
    throwOnError: false,
  })
  return { ...mutation, failure, clearFailure: () => setFailure(null) }
}

/** Field messages for the current failure, with the `payload.` prefix the event payload adds stripped off. */
export const fieldsOf = (failure: Failure | null): Record<string, string> =>
  Object.fromEntries(
    Object.entries(failure?.kind === 'validation' ? failure.fields : {}).map(([k, v]) => [
      k.replace(/^payload\./, ''),
      v,
    ]),
  )
