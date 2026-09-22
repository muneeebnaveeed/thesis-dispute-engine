export const formatMoney = (amount: string, currency: string): string => {
  const numeric = Number(amount)
  if (!Number.isFinite(numeric)) return `${amount} ${currency}`
  try {
    return new Intl.NumberFormat('en-GB', { style: 'currency', currency, currencyDisplay: 'code' })
      .format(numeric)
      .replace(/ /g, ' ')
  } catch {
    return `${amount} ${currency}`
  }
}
