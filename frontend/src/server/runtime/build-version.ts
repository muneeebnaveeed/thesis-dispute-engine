import { BUILD_VERSION } from '#/lib/stale-build'

// what an open tab compares its own build against; never cached, or a tab would compare against an old answer
export const buildVersionResponse = (version: string = BUILD_VERSION): Response =>
  new Response(version, {
    headers: { 'content-type': 'text/plain; charset=utf-8', 'cache-control': 'no-store' },
  })
