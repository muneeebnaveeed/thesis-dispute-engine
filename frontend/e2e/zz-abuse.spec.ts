import { api, expect, test } from './fixtures'

// In its own file, lastStatus by name on purpose: it blocks this machine's address for bearer traffic at the API for a
// minute, and on a dev machine the frontend server shares that address.
test('the API refuses a peer that keeps guessing credentials', async ({ request }) => {
  let lastStatus = 0
  for (let attempt = 0; attempt < 40; attempt++) {
    const response = await request.get(`${api}/disputes/00000000-0000-8000-8000-000000000000`, {
      headers: { Authorization: `Bearer tk_definitely_wrong_${attempt}` },
    })
    lastStatus = response.status()
    if (lastStatus === 429) break
  }
  expect(lastStatus).toBe(429)
})
