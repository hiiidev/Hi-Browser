import { ChevronLeft, ChevronRight, MoreHorizontal } from 'lucide-react'

import { Button, Select } from '../../../../shared/components'

interface ProxyPoolPaginationProps {
  currentPage: number
  end: number
  onPageChange: (page: number) => void
  onPageSizeChange: (pageSize: number) => void
  pageCount: number
  pageSize: number
  pageSizeOptions: number[]
  selectedCount: number
  start: number
  total: number
}

function getVisiblePages(currentPage: number, pageCount: number): Array<number | 'ellipsis'> {
  if (pageCount <= 7) return Array.from({ length: pageCount }, (_, index) => index + 1)

  const pages: Array<number | 'ellipsis'> = [1]
  const windowStart = Math.max(2, currentPage - 1)
  const windowEnd = Math.min(pageCount - 1, currentPage + 1)
  if (windowStart > 2) pages.push('ellipsis')
  for (let page = windowStart; page <= windowEnd; page += 1) pages.push(page)
  if (windowEnd < pageCount - 1) pages.push('ellipsis')
  pages.push(pageCount)
  return pages
}

export function ProxyPoolPagination({
  currentPage,
  end,
  onPageChange,
  onPageSizeChange,
  pageCount,
  pageSize,
  pageSizeOptions,
  selectedCount,
  start,
  total,
}: ProxyPoolPaginationProps) {
  const pages = getVisiblePages(currentPage, pageCount)

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-[var(--color-border-muted)] px-4 py-3">
      <div className="text-xs text-[var(--color-text-muted)] tabular-nums">
        当前 {start}-{end} / 共 {total} 条{selectedCount > 0 ? ` · 已选 ${selectedCount} 条` : ''}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs text-[var(--color-text-muted)]">每页</span>
        <Select
          value={String(pageSize)}
          onChange={event => onPageSizeChange(Number(event.target.value))}
          options={pageSizeOptions.map(value => ({ value: String(value), label: String(value) }))}
          aria-label="每页显示数量"
          className="w-[76px]"
        />
        <Button
          size="sm"
          variant="secondary"
          onClick={() => onPageChange(currentPage - 1)}
          disabled={currentPage <= 1}
          aria-label="上一页"
          className="px-2"
        >
          <ChevronLeft className="h-4 w-4" />
        </Button>
        <div className="flex items-center gap-1">
          {pages.map((page, index) => page === 'ellipsis' ? (
            <span key={`ellipsis-${index}`} className="flex h-8 w-7 items-center justify-center text-[var(--color-text-muted)]">
              <MoreHorizontal className="h-4 w-4" />
            </span>
          ) : (
            <Button
              key={page}
              size="sm"
              variant={page === currentPage ? 'primary' : 'ghost'}
              onClick={() => onPageChange(page)}
              aria-label={`第 ${page} 页`}
              aria-current={page === currentPage ? 'page' : undefined}
              className="min-w-8 px-2 tabular-nums"
            >
              {page}
            </Button>
          ))}
        </div>
        <Button
          size="sm"
          variant="secondary"
          onClick={() => onPageChange(currentPage + 1)}
          disabled={currentPage >= pageCount}
          aria-label="下一页"
          className="px-2"
        >
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>
    </div>
  )
}
