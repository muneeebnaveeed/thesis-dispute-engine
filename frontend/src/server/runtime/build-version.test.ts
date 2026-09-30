import { expect, test } from 'vitest'

import { buildVersionResponse } from './build-version'

test('the build version is plain text that no cache may keep', async () => {
  const res = buildVersionResponse('v7')
  expect(res.status).toBe(200)
  expect(res.headers.get('cache-control')).toBe('no-store')
  expect(res.headers.get('content-type')).toMatch(/^text\/plain/)
  expect(await res.text()).toBe('v7')
})
