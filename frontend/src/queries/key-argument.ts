// a null identifying argument yields the key prefix (disputeQuery(null).queryKey is ['disputes']) and must never fetch
export const keyArgument = <T>(value: T | null, what: string): T => {
  if (value === null) throw new Error(`${what} is required to fetch; null is for the key prefix only`)
  return value
}
