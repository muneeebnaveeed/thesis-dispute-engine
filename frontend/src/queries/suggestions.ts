import { suggestSearchFilters } from '#/server/functions/suggestions'
import { useServerMutation } from './use-server-mutation'

export const useSearchFilterSuggestion = () =>
  useServerMutation((query: string) => suggestSearchFilters({ data: { query } }))
