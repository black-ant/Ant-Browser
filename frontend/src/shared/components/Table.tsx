import { ReactNode } from 'react'
import clsx from 'clsx'
import { ArrowUp, ArrowDown } from 'lucide-react'

export type SortOrder = 'asc' | 'desc' | undefined

export interface SorterResult {
  column: string
  order: SortOrder
}

export interface TableColumn<T> {
  key: string
  title: ReactNode
  width?: string | number
  align?: 'left' | 'center' | 'right'
  render?: (value: any, record: T, index: number) => ReactNode
  sortable?: boolean // 是否可排序
}

interface TableProps<T> {
  columns: TableColumn<T>[]
  data: T[]
  rowKey: string | ((record: T) => string)
  loading?: boolean
  emptyText?: string
  onRowClick?: (record: T) => void
  className?: string
  maxHeight?: string  // 表格最大高度，默认 'calc(100vh - 320px)'
  stickyHeader?: boolean  // 是否固定表头，默认 true
  onSort?: (sorterResult: SorterResult) => void // 排序变化回调
  sortColumn?: string // 当前排序的列
  sortOrder?: SortOrder // 当前排序方式
}

export function Table<T extends Record<string, any>>({
  columns,
  data,
  rowKey,
  loading = false,
  emptyText = '暂无数据',
  onRowClick,
  className,
  maxHeight = 'calc(100vh - 320px)',
  stickyHeader = true,
  onSort,
  sortColumn,
  sortOrder,
}: TableProps<T>) {
  const getRowKey = (record: T, index: number): string => {
    if (typeof rowKey === 'function') {
      return rowKey(record)
    }
    return record[rowKey] ?? index.toString()
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]" style={{ maxHeight }}>
        <div className="flex flex-col items-center gap-3">
          <div className="w-12 h-12 rounded-full bg-[var(--color-bg-muted)] flex items-center justify-center">
            <div className="w-6 h-6 border-3 border-[var(--color-accent)] border-t-transparent rounded-full animate-spin" />
          </div>
          <span className="text-sm font-medium text-[var(--color-text-secondary)]">加载中...</span>
        </div>
      </div>
    )
  }

  const handleSortClick = (column: TableColumn<T>) => {
    if (!column.sortable || !onSort) return;

    let newOrder: SortOrder;
    if (sortColumn !== column.key) {
      newOrder = 'asc';
    } else {
      newOrder = sortOrder === 'asc' ? 'desc' : sortOrder === 'desc' ? undefined : 'asc';
    }

    onSort({ column: column.key, order: newOrder });
  };

  // 渲染排序图标
  const renderSortIcon = (column: TableColumn<T>) => {
    if (!column.sortable) return null;

    if (sortColumn === column.key) {
      if (sortOrder === 'asc') {
        return <ArrowUp className="w-3.5 h-3.5 ml-1 text-[var(--color-accent)]" />;
      }
      if (sortOrder === 'desc') {
        return <ArrowDown className="w-3.5 h-3.5 ml-1 text-[var(--color-accent)]" />;
      }
    }

    return (
      <span className="text-[var(--color-text-muted)] ml-1 opacity-40 group-hover:opacity-70">
        <ArrowUp className="w-3 h-3" />
      </span>
    );
  };

  return (
    <div
      className={clsx('overflow-auto rounded-xl border border-[var(--color-border-default)]', className)}
      style={{ maxHeight }}
    >
      <table className="min-w-full">
        <thead className={clsx(stickyHeader && 'sticky top-0 z-10')}>
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                className={clsx(
                  'px-4 py-3.5 text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wider',
                  'bg-gradient-to-r from-[var(--color-bg-muted)] to-[var(--color-bg-surface)]',
                  'border-b-2 border-[var(--color-border-default)]',
                  col.align === 'center' && 'text-center',
                  col.align === 'right' && 'text-right',
                  !col.align && 'text-left',
                  col.sortable && 'cursor-pointer group hover:text-[var(--color-text-primary)] transition-colors duration-200'
                )}
                style={{ width: col.width }}
                onClick={() => col.sortable && handleSortClick(col)}
              >
                <span className="flex items-center">
                  {col.title}
                  {renderSortIcon(col)}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-[var(--color-border-muted)] bg-[var(--color-bg-surface)]">
          {data.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className="px-4 py-16 text-center">
                <div className="flex flex-col items-center gap-3">
                  <div className="w-16 h-16 rounded-2xl bg-[var(--color-bg-muted)] flex items-center justify-center">
                    <svg className="w-8 h-8 text-[var(--color-text-muted)]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                    </svg>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-[var(--color-text-secondary)]">{emptyText}</p>
                    <p className="text-xs text-[var(--color-text-muted)] mt-1">暂时没有可显示的数据</p>
                  </div>
                </div>
              </td>
            </tr>
          ) : (
            data.map((record, index) => (
              <tr
                key={getRowKey(record, index)}
                className={clsx(
                  'hover:bg-[var(--color-accent)]/5 transition-all duration-200',
                  onRowClick && 'cursor-pointer active:bg-[var(--color-accent)]/10'
                )}
                onClick={() => onRowClick?.(record)}
              >
                {columns.map((col) => (
                  <td
                    key={col.key}
                    className={clsx(
                      'px-4 py-4 text-sm text-[var(--color-text-secondary)]',
                      col.align === 'center' && 'text-center',
                      col.align === 'right' && 'text-right'
                    )}
                  >
                    {col.render
                      ? col.render(record[col.key], record, index)
                      : record[col.key]}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
