import { Button, Input, Select, Switch } from '../../../../shared/components'

interface ProxyPoolToolbarProps {
  allCurrentPageSelected: boolean
  failedCount: number
  filterAvailableOnly: boolean
  filterGroup: string
  filterKeyword: string
  filterProtocol: string
  globalAutoRefreshEnabled: boolean
  globalRefreshIntervalM: string
  groups: string[]
  hasActiveFilters: boolean
  hasSelectablePageItems: boolean
  isTesting: boolean
  onClearFilters: () => void
  onDeleteFailed: () => void
  onFilterAvailableOnlyChange: (checked: boolean) => void
  onFilterGroupChange: (value: string) => void
  onFilterKeywordChange: (value: string) => void
  onFilterProtocolChange: (value: string) => void
  onGlobalAutoRefreshEnabledChange: (checked: boolean) => void
  onGlobalRefreshIntervalMChange: (value: string) => void
  onOpenBatchDelete: () => void
  onToggleAll: () => void
  onWarmupSelected: () => void
  protocolOptions: string[]
  selectedCount: number
  someCurrentPageSelected: boolean
  warmingAllBridges: boolean
}

const checkboxClassName = 'h-4 w-4 shrink-0 cursor-pointer rounded border-[var(--color-border-default)] accent-[var(--color-accent)]'

export function ProxyPoolToolbar({
  allCurrentPageSelected,
  failedCount,
  filterAvailableOnly,
  filterGroup,
  filterKeyword,
  filterProtocol,
  globalAutoRefreshEnabled,
  globalRefreshIntervalM,
  groups,
  hasActiveFilters,
  hasSelectablePageItems,
  isTesting,
  onClearFilters,
  onDeleteFailed,
  onFilterAvailableOnlyChange,
  onFilterGroupChange,
  onFilterKeywordChange,
  onFilterProtocolChange,
  onGlobalAutoRefreshEnabledChange,
  onGlobalRefreshIntervalMChange,
  onOpenBatchDelete,
  onToggleAll,
  onWarmupSelected,
  protocolOptions,
  selectedCount,
  someCurrentPageSelected,
  warmingAllBridges,
}: ProxyPoolToolbarProps) {
  return (
    <div className="space-y-3 border-b border-[var(--color-border-muted)] p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Input
          value={filterKeyword}
          onChange={event => onFilterKeywordChange(event.target.value)}
          placeholder="搜索名称或服务器..."
          aria-label="搜索代理"
          className="w-full sm:w-56"
        />
        <Select
          value={filterProtocol}
          onChange={event => onFilterProtocolChange(event.target.value)}
          options={protocolOptions.map(protocol => ({
            value: protocol,
            label: protocol === 'all' ? '全部协议' : protocol.toUpperCase(),
          }))}
          aria-label="按协议筛选"
          className="w-[132px]"
        />
        <Select
          value={filterGroup}
          onChange={event => onFilterGroupChange(event.target.value)}
          options={[
            { value: 'all', label: '全部分组' },
            ...groups.map(group => ({ value: group, label: group })),
          ]}
          aria-label="按分组筛选"
          className="w-[148px]"
        />
        <label className="flex min-h-9 shrink-0 cursor-pointer select-none items-center gap-2 rounded-lg px-2 text-sm text-[var(--color-text-secondary)]">
          <input
            type="checkbox"
            checked={filterAvailableOnly}
            onChange={event => onFilterAvailableOnlyChange(event.target.checked)}
            className={checkboxClassName}
          />
          <span className="whitespace-nowrap">只展示可用</span>
        </label>
        {hasActiveFilters && <Button size="sm" variant="ghost" onClick={onClearFilters}>清除筛选</Button>}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex min-h-9 flex-wrap items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-2.5 py-1.5">
            <span className="whitespace-nowrap text-xs text-[var(--color-text-muted)]">全局自动刷新</span>
            <Switch checked={globalAutoRefreshEnabled} onChange={onGlobalAutoRefreshEnabledChange} />
            <Input
              type="number"
              min={5}
              max={1440}
              value={globalRefreshIntervalM}
              onChange={event => onGlobalRefreshIntervalMChange(event.target.value)}
              className="w-20"
              disabled={!globalAutoRefreshEnabled}
              aria-label="自动刷新间隔分钟数"
            />
            <span className="text-xs text-[var(--color-text-muted)]">分钟</span>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {hasSelectablePageItems && (
            <label className="flex min-h-8 shrink-0 cursor-pointer select-none items-center gap-2 px-1 text-sm text-[var(--color-text-muted)]">
              <input
                type="checkbox"
                checked={allCurrentPageSelected}
                ref={element => {
                  if (element) element.indeterminate = someCurrentPageSelected && !allCurrentPageSelected
                }}
                onChange={onToggleAll}
                className={checkboxClassName}
              />
              <span className="whitespace-nowrap">全选本页</span>
            </label>
          )}
          {selectedCount > 0 && (
            <>
              <Button size="sm" variant="secondary" onClick={onWarmupSelected} loading={warmingAllBridges}>
                预热所选 ({selectedCount})
              </Button>
              <Button size="sm" variant="danger" onClick={onOpenBatchDelete}>
                删除所选 ({selectedCount})
              </Button>
            </>
          )}
          <span className="mx-1 hidden h-6 w-px bg-[var(--color-border-muted)] sm:block" aria-hidden="true" />
          <Button size="sm" variant="danger" onClick={onDeleteFailed} disabled={failedCount === 0 || isTesting}>
            删除测试失败 ({failedCount})
          </Button>
        </div>
      </div>
    </div>
  )
}
