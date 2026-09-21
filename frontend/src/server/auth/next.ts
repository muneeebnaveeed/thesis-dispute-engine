/** Only same-origin paths may be a post-login destination; anything else falls back to home. */
export function safeNext(raw: unknown): string {
  return typeof raw === 'string' && raw.startsWith('/') && !raw.startsWith('//') && !raw.startsWith('/t/')
    ? raw
    : '/'
}
