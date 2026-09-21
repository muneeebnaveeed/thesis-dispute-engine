import { api, expect, test } from './fixtures'

// In its own file, last by name on purpose: it blocks this machine's address for bearer traffic at the API for a
// minute, and on a dev machine the frontend server shares that address.
test('the API refuses a peer that keeps guessing credentials', async ({ request }) => {
  let last = 0
  for (let i = 0; i < 40; i++) {
    const res = await request.get(`${api}/disputes/00000000-0000-8000-8000-000000000000`, {
      headers: { Authorization: `Bearer tk_definitely_wrong_${i}` },
    })
    last = res.status()
    if (last === 429) break
  }
  expect(last).toBe(429)
})
