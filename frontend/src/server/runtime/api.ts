import { createApi } from '#/api/client'
import { serverEnv } from '#/server/runtime/env'

/** The app's server-side client: bound to this deployment's API URL, authenticated as the given analyst token. */
export const createServerApi = (token: string) => createApi(serverEnv().apiUrl, token)
