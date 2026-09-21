import { safeNext } from './next'

test("only this tenant's same-origin paths survive as a post-login destination", () => {
  expect(safeNext('/otp/disputes/abc', 'otp')).toBe('/otp/disputes/abc')
  expect(safeNext('/otp?x=1', 'otp')).toBe('/otp?x=1')
  expect(safeNext('/otp', 'otp')).toBe('/otp')
  expect(safeNext('/erste/disputes/abc', 'otp')).toBe('/otp')
  expect(safeNext('/otpx/disputes', 'otp')).toBe('/otp')
  expect(safeNext('https://evil.example/', 'otp')).toBe('/otp')
  expect(safeNext('//evil.example/', 'otp')).toBe('/otp')
  expect(safeNext('/auth/callback', 'otp')).toBe('/otp')
  expect(safeNext(undefined, 'otp')).toBe('/otp')
})
