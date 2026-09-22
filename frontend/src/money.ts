/** Money as a person reads it: grouped, at the currency's minor unit, with the code rather than a symbol. */
export function formatMoney(amount: string, currency: string): string {
  const n = Number(amount)
  if (!Number.isFinite(n)) return `${amount} ${currency}`
  try {
    return new Intl.NumberFormat('en-GB', { style: 'currency', currency, currencyDisplay: 'code' })
      .format(n)
      .replace(/ /g, ' ')
  } catch {
    return `${amount} ${currency}`
  }
}
