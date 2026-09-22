import { createApi } from '#/api/client'
import { serverEnv } from '#/server/runtime/env'

export const createServerApi = (token: string) => createApi(serverEnv().apiUrl, token)
