import { open, seal } from './crypto'

test('seals and opens a payload, with a fresh nonce each time', async () => {
  const a = await seal({ hello: 'world' })
  const b = await seal({ hello: 'world' })
  expect(Buffer.from(a).equals(Buffer.from(b))).toBe(false)
  expect(await open(a)).toEqual({ hello: 'world' })
})

test('a tampered box does not open', async () => {
  const box = await seal({ n: 1 })
  box[box.length - 1] = (box[box.length - 1] ?? 0) ^ 1
  await expect(open(box)).rejects.toThrow(/operation|decrypt|failed/i)
})
