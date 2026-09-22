import { cn } from './cn'

test('merges conditionals and resolves Tailwind conflicts in favour of the last class', () => {
  const hidden = false as boolean
  expect(cn('px-2 py-1', hidden && 'hidden', { 'text-red-900': true }, 'px-4')).toBe('py-1 text-red-900 px-4')
})
