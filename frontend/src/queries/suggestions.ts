import { suggestDisputeReason, suggestSearchFilters } from '#/server/functions/suggestions'
import { useServerMutation } from './use-server-mutation'

export const useDisputeReasonSuggestion = () =>
  useServerMutation((description: string) => suggestDisputeReason({ data: { description } }))

export const useSearchFilterSuggestion = () =>
  useServerMutation((query: string) => suggestSearchFilters({ data: { query } }))
