import { safeNext } from './next'

test('only same-origin paths survive as a post-login destination', () => {
  expect(safeNext('/disputes/abc')).toBe('/disputes/abc')
  expect(safeNext('/disputes/abc?x=1')).toBe('/disputes/abc?x=1')
  expect(safeNext('https://evil.example/')).toBe('/')
  expect(safeNext('//evil.example/')).toBe('/')
  expect(safeNext('/t/otp')).toBe('/')
  expect(safeNext(undefined)).toBe('/')
})
