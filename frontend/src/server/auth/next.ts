/** Only same-origin paths may be a post-login destination; anything else falls back to the tenant's home. */
export function safeNext(raw: unknown, slug: string): string {
  const home = `/${slug}`
  if (typeof raw !== 'string' || !raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/auth/'))
    return home
  // Stay inside this tenant's part of the app.
  return raw === home || raw.startsWith(`${home}/`) || raw.startsWith(`${home}?`) ? raw : home
}
