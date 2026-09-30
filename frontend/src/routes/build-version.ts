import { createFileRoute } from '@tanstack/react-router'

import { buildVersionResponse } from '#/server/runtime/build-version'

export const Route = createFileRoute('/build-version')({
  server: { handlers: { GET: () => buildVersionResponse() } },
})
