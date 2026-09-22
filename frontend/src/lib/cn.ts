import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** Class names with conditionals resolved and Tailwind conflicts merged, last one wins. */
export const cn = (...inputs: ClassValue[]): string => twMerge(clsx(inputs))
