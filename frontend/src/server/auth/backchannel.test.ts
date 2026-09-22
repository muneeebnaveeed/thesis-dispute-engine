import { SignJWT, exportJWK, generateKeyPair, createLocalJWKSet } from 'jose'

import { verifyLogoutToken } from './backchannel'

const issuer = 'http://localhost:8180/realms/otp'
const event = { 'http://schemas.openid.net/event/backchannel-logout': {} }

const fakeRealm = async () => {
  const { publicKey, privateKey } = await generateKeyPair('RS256')
  const jwk = { ...(await exportJWK(publicKey)), kid: 'k1', alg: 'RS256', use: 'sig' }
  const jwks = createLocalJWKSet({ keys: [jwk] })
  const sign = (claims: Record<string, unknown>, overrides: { iss?: string; aud?: string } = {}) =>
    new SignJWT(claims)
      .setProtectedHeader({ alg: 'RS256', kid: 'k1' })
      .setIssuer(overrides.iss ?? issuer)
      .setAudience(overrides.aud ?? 'frontend')
      .setIssuedAt()
      .setJti(crypto.randomUUID())
      .sign(privateKey)
  return { jwks, sign }
}

test('accepts a logout token for a realm session id', async () => {
  const realm = await fakeRealm()
  const keys = { jwks: () => Promise.resolve(realm.jwks) }
  expect(await verifyLogoutToken(await realm.sign({ events: event, sid: 's-1', sub: 'u-1' }), keys)).toEqual({
    sid: 's-1',
  })
  expect(await verifyLogoutToken(await realm.sign({ events: event, sub: 'u-1' }), keys)).toEqual({
    issuer,
    subject: 'u-1',
  })
})

test('rejects tokens that are not logout events, carry a nonce, have the wrong audience or issuer, or are unsigned by the realm', async () => {
  const realm = await fakeRealm()
  const otherRealm = await fakeRealm()
  const keys = { jwks: () => Promise.resolve(realm.jwks) }
  expect(await verifyLogoutToken(await realm.sign({ sid: 's-1' }), keys)).toBeNull()
  expect(
    await verifyLogoutToken(await realm.sign({ events: event, sid: 's-1', nonce: 'n' }), keys),
  ).toBeNull()
  expect(
    await verifyLogoutToken(await realm.sign({ events: event, sid: 's-1' }, { aud: 'someone-else' }), keys),
  ).toBeNull()
  expect(
    await verifyLogoutToken(
      await realm.sign({ events: event, sid: 's-1' }, { iss: 'http://evil/realms/otp' }),
      keys,
    ),
  ).toBeNull()
  expect(await verifyLogoutToken(await otherRealm.sign({ events: event, sid: 's-1' }), keys)).toBeNull()
  expect(await verifyLogoutToken('not-a-jwt', keys)).toBeNull()
})
