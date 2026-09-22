import { useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query'
import { isRedirect, useRouter } from '@tanstack/react-router'
import { useState } from 'react'

import { classify, fromProblem, type Failure } from '#/api/failure'
import type { Outcome } from '#/api/views'

export const useServerMutation = <TVariables, TData>(
  run: (variables: TVariables) => Promise<Outcome<TData>>,
  options: {
    invalidates?: (variables: TVariables, data: TData) => QueryKey[]
    onSuccess?: (data: TData, variables: TVariables) => void | Promise<void>
  } = {},
) => {
  const queryClient = useQueryClient()
  const router = useRouter()
  const [failure, setFailure] = useState<Failure | null>(null)
  const mutation = useMutation({
    mutationFn: async (variables: TVariables): Promise<TData> => {
      setFailure(null)
      let outcome: Outcome<TData>
      try {
        outcome = await run(variables)
      } catch (thrown) {
        // the session ended under this tab: the middleware sends us back through the front door
        if (isRedirect(thrown)) {
          await router.navigate(router.resolveRedirect(thrown).options)
          throw thrown
        }
        const unreachable = classify({ thrown }) ?? {
          kind: 'unreachable' as const,
          message: 'the server could not be reached',
        }
        setFailure(unreachable)
        throw unreachable
      }
      if (outcome.problem) {
        const refused = fromProblem(outcome.problem)
        setFailure(refused)
        throw refused
      }
      return outcome.value
    },
    onSuccess: async (data, variables) => {
      await Promise.all(
        (options.invalidates?.(variables, data) ?? []).map((queryKey) =>
          queryClient.invalidateQueries({ queryKey }),
        ),
      )
      await options.onSuccess?.(data, variables)
    },
    throwOnError: false,
  })
  return { ...mutation, failure, clearFailure: () => setFailure(null) }
}

// the event payload nests its fields under `payload.`; the form knows them by bare name
export const fieldErrorsOf = (failure: Failure | null): Record<string, string> =>
  Object.fromEntries(
    Object.entries(failure?.kind === 'validation' ? failure.fields : {}).map(([fieldPath, message]) => [
      fieldPath.replace(/^payload\./, ''),
      message,
    ]),
  )
