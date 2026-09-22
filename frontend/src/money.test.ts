import { formatMoney } from './money'

test('formats at the currency minor unit with grouping and the code', () => {
  expect(formatMoney('1899.00', 'EUR')).toBe('EUR 1,899.00')
  expect(formatMoney('75.4', 'USD')).toBe('USD 75.40')
  expect(formatMoney('12500', 'HUF')).toBe('HUF 12,500')
  expect(formatMoney('0.00', 'EUR')).toBe('EUR 0.00')
  expect(formatMoney('abc', 'EUR')).toBe('abc EUR')
})
