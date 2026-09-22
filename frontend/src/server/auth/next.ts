export const safeNext = (raw: unknown, slug: string): string => {
  const home = `/${slug}`
  if (typeof raw !== 'string' || !raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/auth/'))
    return home
  return raw === home || raw.startsWith(`${home}/`) || raw.startsWith(`${home}?`) ? raw : home
}
