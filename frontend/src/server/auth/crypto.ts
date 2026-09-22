// AES-256-GCM under SESSION_SECRET: the API stores ciphertext it cannot read (ADR 0011)
import { serverEnv } from '#/server/runtime/env'

const IV_BYTES = 12

let sealingKeyPromise: Promise<CryptoKey> | undefined
const sealingKey = (): Promise<CryptoKey> => {
  sealingKeyPromise ??= (async () => {
    const secret = Buffer.from(serverEnv().sessionSecret, 'base64')
    if (secret.length !== 32) throw new Error('SESSION_SECRET must be 32 bytes, base64 encoded')
    return crypto.subtle.importKey('raw', secret, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt'])
  })()
  return sealingKeyPromise
}

export const seal = async (value: unknown): Promise<Uint8Array> => {
  const iv = crypto.getRandomValues(new Uint8Array(IV_BYTES))
  const plaintext = new TextEncoder().encode(JSON.stringify(value))
  const ciphertext = new Uint8Array(
    await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, await sealingKey(), plaintext),
  )
  const sealed = new Uint8Array(iv.length + ciphertext.length)
  sealed.set(iv)
  sealed.set(ciphertext, iv.length)
  return sealed
}

// a sealed box proves origin, not schema; callers validate the shape
export const open = async (sealed: Uint8Array): Promise<unknown> => {
  const iv = sealed.slice(0, IV_BYTES)
  const ciphertext = sealed.slice(IV_BYTES)
  const plaintext = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, await sealingKey(), ciphertext)
  return JSON.parse(new TextDecoder().decode(plaintext))
}
