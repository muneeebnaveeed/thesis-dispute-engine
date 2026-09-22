import type { ReactNode } from 'react'

import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'

// The strip of actions above a grid, and the strip of paging controls below it: the period put both on every list.
export const GridToolbar = ({ children, className }: { children: ReactNode; className?: string }) => (
  <div
    className={cn(
      'flex flex-wrap items-center gap-1 border-b border-border bg-[image:var(--toolbar)] px-1 py-[3px]',
      className,
    )}
  >
    {children}
  </div>
)

export const ToolbarSeparator = () => <span className="mx-1 h-4 w-px bg-border" />

export const ToolbarFill = () => <span className="flex-1" />

const PAGE_SIZES = [25, 50, 100]

export const GridPager = ({
  page,
  shown,
  pageSize,
  hasNext,
  onFirst,
  onPrevious,
  onNext,
  onPageSize,
  noun,
}: {
  page: number
  shown: number
  pageSize: number
  hasNext: boolean
  onFirst: () => void
  onPrevious: () => void
  onNext: () => void
  onPageSize: (size: number) => void
  noun: string
}) => {
  const first = shown === 0 ? 0 : (page - 1) * pageSize + 1
  return (
    <div className="flex flex-wrap items-center gap-1.5 border-t border-border bg-[image:var(--toolbar)] px-1.5 py-1 text-[11px]">
      <Button variant="secondary" size="sm" disabled={page === 1} onClick={onFirst} aria-label="First page">
        &laquo;
      </Button>
      <Button
        variant="secondary"
        size="sm"
        disabled={page === 1}
        onClick={onPrevious}
        aria-label="Previous page"
      >
        &lsaquo;
      </Button>
      <span>Page {page}</span>
      <Button variant="secondary" size="sm" disabled={!hasNext} onClick={onNext} aria-label="Next page">
        &rsaquo;
      </Button>
      <span className="mx-1 h-4 w-px bg-border" />
      <label className="flex items-center gap-1">
        Rows
        <select
          className="border border-input bg-card px-1 py-px text-[11px] shadow-[inset_1px_1px_2px_rgb(0_0_0/0.12)]"
          value={pageSize}
          onChange={(event) => onPageSize(Number(event.target.value))}
        >
          {PAGE_SIZES.map((size) => (
            <option key={size} value={size}>
              {size}
            </option>
          ))}
        </select>
      </label>
      <span className="flex-1" />
      <span className="text-muted-foreground">
        {shown === 0 ? `No ${noun} to display` : `Displaying ${noun} ${first} - ${first + shown - 1}`}
      </span>
    </div>
  )
}
