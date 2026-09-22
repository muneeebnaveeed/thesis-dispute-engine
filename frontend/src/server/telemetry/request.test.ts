import { routeOf } from './request'

test('span names stay bounded: ids and ordinals become placeholders, an RPC is one name', () => {
  expect(routeOf(new URL('http://x/otp/disputes/0199a2b4-7c3e-7a1f-9d2e-1b2c3d4e5f60'))).toBe(
    '/otp/disputes/:id',
  )
  expect(routeOf(new URL('http://x/otp/disputes/0199a2b4-7c3e-7a1f-9d2e-1b2c3d4e5f60/notices/12'))).toBe(
    '/otp/disputes/:id/notices/:n',
  )
  expect(routeOf(new URL('http://x/_serverFn/abc123?payload=1'))).toBe('/_serverFn/:fn')
  expect(routeOf(new URL('http://x/otp/keys'))).toBe('/otp/keys')
})
