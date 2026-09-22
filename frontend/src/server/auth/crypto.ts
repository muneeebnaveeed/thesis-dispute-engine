// AES-256-GCM over the session payload with a key only the frontend server holds (SESSION_SECRET, base64, 32 bytes).
// The API stores the result without being able to read it (ADR 0011).
import { serverEnv } from '#/server/runtime/env'

let keyPromise: Promise<CryptoKey> | undefined
const key = (): Promise<CryptoKey> => {
  keyPromise ??= (async () => {
    const raw = Buffer.from(serverEnv().sessionSecret, 'base64')
    if (raw.length !== 32) throw new Error('SESSION_SECRET must be 32 bytes, base64 encoded')
    return crypto.subtle.importKey('raw', raw, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt'])
  })()
  return keyPromise
}

export const seal = async (value: unknown): Promise<Uint8Array> => {
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const plain = new TextEncoder().encode(JSON.stringify(value))
  const box = new Uint8Array(await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, await key(), plain))
  const out = new Uint8Array(iv.length + box.length)
  out.set(iv)
  out.set(box, iv.length)
  return out
}

/** Returns the parsed payload; callers validate the shape, since a sealed box proves origin, not schema. */
export const open = async (sealed: Uint8Array): Promise<unknown> => {
  const iv = sealed.slice(0, 12)
  const box = sealed.slice(12)
  const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, await key(), box)
  return JSON.parse(new TextDecoder().decode(plain))
}
