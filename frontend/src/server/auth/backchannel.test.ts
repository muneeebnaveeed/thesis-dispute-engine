import { SignJWT, exportJWK, generateKeyPair, createLocalJWKSet } from 'jose'

import { verifyLogoutToken } from './backchannel'

const issuer = 'http://localhost:8180/realms/otp'
const event = { 'http://schemas.openid.net/event/backchannel-logout': {} }

const realm = async () => {
  const { publicKey, privateKey } = await generateKeyPair('RS256')
  const jwk = { ...(await exportJWK(publicKey)), kid: 'k1', alg: 'RS256', use: 'sig' }
  const jwks = createLocalJWKSet({ keys: [jwk] })
  const sign = (claims: Record<string, unknown>, opts: { iss?: string; aud?: string } = {}) =>
    new SignJWT(claims)
      .setProtectedHeader({ alg: 'RS256', kid: 'k1' })
      .setIssuer(opts.iss ?? issuer)
      .setAudience(opts.aud ?? 'frontend')
      .setIssuedAt()
      .setJti(crypto.randomUUID())
      .sign(privateKey)
  return { jwks, sign }
}

test('accepts a logout token for a realm session id', async () => {
  const r = await realm()
  const deps = { jwks: () => Promise.resolve(r.jwks) }
  expect(await verifyLogoutToken(await r.sign({ events: event, sid: 's-1', sub: 'u-1' }), deps)).toEqual({
    sid: 's-1',
  })
  expect(await verifyLogoutToken(await r.sign({ events: event, sub: 'u-1' }), deps)).toEqual({
    issuer,
    subject: 'u-1',
  })
})

test('rejects tokens that are not logout events, carry a nonce, have the wrong audience or issuer, or are unsigned by the realm', async () => {
  const r = await realm()
  const other = await realm()
  const deps = { jwks: () => Promise.resolve(r.jwks) }
  expect(await verifyLogoutToken(await r.sign({ sid: 's-1' }), deps)).toBeNull()
  expect(await verifyLogoutToken(await r.sign({ events: event, sid: 's-1', nonce: 'n' }), deps)).toBeNull()
  expect(
    await verifyLogoutToken(await r.sign({ events: event, sid: 's-1' }, { aud: 'someone-else' }), deps),
  ).toBeNull()
  expect(
    await verifyLogoutToken(
      await r.sign({ events: event, sid: 's-1' }, { iss: 'http://evil/realms/otp' }),
      deps,
    ),
  ).toBeNull()
  expect(await verifyLogoutToken(await other.sign({ events: event, sid: 's-1' }), deps)).toBeNull()
  expect(await verifyLogoutToken('not-a-jwt', deps)).toBeNull()
})
