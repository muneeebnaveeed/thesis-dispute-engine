import type { ApiResult } from '#/api/views'
import { authenticatedGet, authenticatedPost } from '#/server/runtime/fn'

type Branding = { name: string; logoDataUrl: string | null }
type Avatar = { dataUrl: string | null }

// images ride to the browser inline: they are at most 256 kB, and a bare URL would not carry the session
const asDataUrl = async (blob: Blob, response: Response): Promise<string> =>
  `data:${response.headers.get('content-type') ?? 'image/png'};base64,${Buffer.from(await blob.arrayBuffer()).toString('base64')}`

// the browser sends a FormData, which no TypeBox schema describes
const imageUpload = (input: unknown): File => {
  if (!(input instanceof FormData)) throw new Error('expected a form')
  const file = input.get('file')
  if (!(file instanceof File)) throw new Error('expected a file')
  return file
}

export const getBranding = authenticatedGet.handler(
  async ({ context: { api } }): Promise<ApiResult<Branding>> => {
    const tenant = await api.GET('/tenant')
    if (tenant.error) return tenant
    if (!tenant.data.hasLogo) {
      return { data: { name: tenant.data.name, logoDataUrl: null }, response: tenant.response }
    }
    const logo = await api.GET('/tenant/logo', { parseAs: 'blob' })
    return {
      data: {
        name: tenant.data.name,
        logoDataUrl: logo.error ? null : await asDataUrl(logo.data, logo.response),
      },
      response: tenant.response,
    }
  },
)

export const getMyAvatar = authenticatedGet.handler(
  async ({ context: { api } }): Promise<ApiResult<Avatar>> => {
    const avatar = await api.GET('/me/avatar', { parseAs: 'blob' })
    if (avatar.error) {
      // an analyst who has never uploaded one is not a failure, it is the initials
      if (avatar.response.status === 404) return { data: { dataUrl: null }, response: avatar.response }
      return avatar
    }
    return { data: { dataUrl: await asDataUrl(avatar.data, avatar.response) }, response: avatar.response }
  },
)

export const putTenantLogo = authenticatedPost
  .validator(imageUpload)
  .handler(({ data: file, context: { api } }) => {
    const multipart = new FormData()
    multipart.append('file', file, file.name)
    return api.PUT('/tenant/logo', { body: { file: file.name }, bodySerializer: () => multipart })
  })

export const putMyAvatar = authenticatedPost
  .validator(imageUpload)
  .handler(({ data: file, context: { api } }) => {
    const multipart = new FormData()
    multipart.append('file', file, file.name)
    return api.PUT('/me/avatar', { body: { file: file.name }, bodySerializer: () => multipart })
  })
