import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link } from 'react-router-dom'
import { Copy, Download, Key, Loader2, MoreHorizontal, Play, Puzzle, Repeat2, RotateCcw, Settings, Square, Trash2, Wifi } from 'lucide-react'

import { Badge, Button, Card, Table } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'

import type { BrowserCore, BrowserProfile, BrowserProxy, ProxySpeedTestResult } from '../types'
import { browserProxyTestSpeed, testProxyConnectivity } from '../api'
import type { BrowserViewMode } from './BrowserListLayout'
import { KeywordInlineRow, LaunchCodeCell } from './BrowserListWidgets'
import { ProfileIconBadge } from './ProfileIconBadge'

type ProfileStatusVariant = 'default' | 'success' | 'error' | 'warning' | 'info'

interface ProfileStatus {
  variant: ProfileStatusVariant
  label: string
}

interface BrowserProfilesPanelProps {
  loading: boolean
  viewMode: BrowserViewMode
  profiles: BrowserProfile[]
  proxies: BrowserProxy[]
  selectedIds: Set<string>
  resolveProfileCore: (profile: BrowserProfile) => BrowserCore | null
  getProfileCoreLabel: (profile: BrowserProfile) => string
  getProfileStatus: (profile: BrowserProfile) => ProfileStatus
  isProfileStarting: (profileId: string) => boolean
  isProfileStopping: (profileId: string) => boolean
  isProfileBusy: (profileId: string) => boolean
  onToggleSelect: (profileId: string) => void
  onSelectAll: () => void
  onDeselectAll: () => void
  onRefreshProfiles: () => void
  onStart: (profileId: string) => void
  onStop: (profileId: string) => void
  onRestart: (profileId: string) => void
  onOpenKeywords: (profile: BrowserProfile) => void
  onOpenExtensions: (profile: BrowserProfile) => void
  onExport: (profile: BrowserProfile) => void
  onOpenCopy: (profile: BrowserProfile) => void
  onOpenProxyPicker: (profile: BrowserProfile) => void
  onDelete: (profileId: string) => void
}

const formatTime = (value?: string) => {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN')
}

function formatProxyLabel(profile: BrowserProfile, proxy?: BrowserProxy): string {
  if (proxy?.proxyName) {
    return proxy.proxyName
  }
  if (profile.proxyId) {
    return profile.proxyId
  }
  const customProxy = (profile.proxyConfig || '').trim()
  if (customProxy) {
    return `自定义: ${customProxy}`
  }
  return '-'
}

function ProxyLatency({ result }: { result?: ProxySpeedTestResult | null }) {
  if (!result) return null
  if (!result.ok) return <span className="text-xs text-red-500">失败</span>
  const color = result.latencyMs < 200 ? 'text-green-500' : result.latencyMs < 500 ? 'text-yellow-500' : 'text-red-500'
  return <span className={`text-xs font-medium ${color}`}>{result.latencyMs}ms</span>
}

function ProxyInlineActions({
  profile,
  proxy,
  isBusy,
  onOpenProxyPicker,
  maxWidthClass = 'max-w-[220px]',
}: {
  profile: BrowserProfile
  proxy?: BrowserProxy
  isBusy: boolean
  onOpenProxyPicker: (profile: BrowserProfile) => void
  maxWidthClass?: string
}) {
  const [testing, setTesting] = useState(false)
  const [speedResult, setSpeedResult] = useState<ProxySpeedTestResult | null>(null)
  const historyResult = proxy?.lastTestedAt
    ? {
        proxyId: proxy.proxyId,
        ok: proxy.lastTestOk ?? false,
        latencyMs: proxy.lastLatencyMs ?? -1,
        error: '',
      }
    : null
  const displayResult = speedResult || historyResult
  const canTest = !!profile.proxyId || !!profile.proxyConfig.trim()

  const handleTest = async () => {
    if (testing || !canTest) return
    setTesting(true)
    try {
      const result = profile.proxyId
        ? await browserProxyTestSpeed(profile.proxyId)
        : await testProxyConnectivity(profile.profileId, profile.proxyConfig)
      setSpeedResult(result)
    } catch (error: any) {
      setSpeedResult({
        proxyId: profile.proxyId || profile.profileId,
        ok: false,
        latencyMs: -1,
        error: error?.message || '测速失败',
      })
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className={`inline-flex ${maxWidthClass} items-center gap-1.5 text-xs`} title={formatProxyLabel(profile, proxy)}>
      <span className="min-w-0 truncate text-[var(--color-text-primary)]">{formatProxyLabel(profile, proxy)}</span>
      <button
        type="button"
        className="shrink-0 rounded p-0.5 text-[var(--color-text-muted)] transition-colors hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-40"
        title={isBusy ? '实例操作中，暂不可切换代理' : '切换代理'}
        disabled={isBusy}
        onClick={() => onOpenProxyPicker(profile)}
      >
        <Repeat2 className="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        className="shrink-0 rounded p-0.5 text-[var(--color-text-muted)] transition-colors hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-40"
        title={canTest ? '测速' : '无可测速代理'}
        disabled={testing || !canTest}
        onClick={handleTest}
      >
        {testing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Wifi className="h-3.5 w-3.5" />}
      </button>
      <ProxyLatency result={displayResult} />
    </div>
  )
}

function ProfileMoreActions({
  open,
  disabled,
  onToggle,
  onClose,
  onRestart,
  onOpenKeywords,
  onOpenExtensions,
  onExport,
}: {
  open: boolean
  disabled: boolean
  onToggle: () => void
  onClose: () => void
  onRestart: () => void
  onOpenKeywords: () => void
  onOpenExtensions: () => void
  onExport: () => void
}) {
  const triggerRef = useRef<HTMLDivElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const [menuPosition, setMenuPosition] = useState({ top: 0, left: 0 })

  useEffect(() => {
    if (!open) return
    const updateMenuPosition = () => {
      const rect = triggerRef.current?.getBoundingClientRect()
      if (!rect) return
      const menuWidth = 128
      const menuHeight = 168
      const gap = 8
      const left = Math.max(8, Math.min(rect.right - menuWidth, window.innerWidth - menuWidth - 8))
      const belowTop = rect.bottom + gap
      const top = belowTop + menuHeight > window.innerHeight
        ? Math.max(8, rect.top - menuHeight - gap)
        : belowTop
      setMenuPosition({ top, left })
    }
    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as Node
      if (!triggerRef.current?.contains(target) && !menuRef.current?.contains(target)) {
        onClose()
      }
    }
    updateMenuPosition()
    document.addEventListener('mousedown', handlePointerDown)
    window.addEventListener('resize', updateMenuPosition)
    window.addEventListener('scroll', updateMenuPosition, true)
    return () => {
      document.removeEventListener('mousedown', handlePointerDown)
      window.removeEventListener('resize', updateMenuPosition)
      window.removeEventListener('scroll', updateMenuPosition, true)
    }
  }, [open, onClose])

  const runAndClose = (handler: () => void) => {
    handler()
    onClose()
  }

  return (
    <>
    <div ref={triggerRef} className="inline-flex">
      <Button
        size="sm"
        variant="ghost"
        onClick={onToggle}
        title="更多"
        disabled={disabled}
        className="px-2"
      >
        <MoreHorizontal className="w-3.5 h-3.5" />
        更多
      </Button>
    </div>
      {open && createPortal(
        <div
          ref={menuRef}
          className="fixed z-[9999] w-32 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-1.5 shadow-xl"
          style={{ top: menuPosition.top, left: menuPosition.left }}
        >
          <button
            type="button"
            className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
            onClick={() => runAndClose(onRestart)}
          >
            <RotateCcw className="w-3.5 h-3.5" />
            重启
          </button>
          <button
            type="button"
            className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
            onClick={() => runAndClose(onOpenKeywords)}
          >
            <Key className="w-3.5 h-3.5" />
            关键字
          </button>
          <button
            type="button"
            className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
            onClick={() => runAndClose(onOpenExtensions)}
          >
            <Puzzle className="w-3.5 h-3.5" />
            插件
          </button>
          <button
            type="button"
            className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
            onClick={() => runAndClose(onExport)}
          >
            <Download className="w-3.5 h-3.5" />
            导出
          </button>
        </div>,
        document.body
      )}
    </>
  )
}

function BrowserProfileCard({
  profile,
  proxy,
  isSelected,
  status,
  coreLabel,
  isStarting,
  isStopping,
  isBusy,
  onToggleSelect,
  onRefreshProfiles,
  onStart,
  onStop,
  onRestart,
  onOpenKeywords,
  onOpenExtensions,
  onOpenCopy,
  onOpenProxyPicker,
  onDelete,
}: {
  profile: BrowserProfile
  proxy: BrowserProxy | undefined
  isSelected: boolean
  status: ProfileStatus
  coreLabel: string
  isStarting: boolean
  isStopping: boolean
  isBusy: boolean
  onToggleSelect: (profileId: string) => void
  onRefreshProfiles: () => void
  onStart: (profileId: string) => void
  onStop: (profileId: string) => void
  onRestart: (profileId: string) => void
  onOpenKeywords: (profile: BrowserProfile) => void
  onOpenExtensions: (profile: BrowserProfile) => void
  onOpenCopy: (profile: BrowserProfile) => void
  onOpenProxyPicker: (profile: BrowserProfile) => void
  onDelete: (profileId: string) => void
}) {
  return (
    <div
      className={`flex min-h-[286px] flex-col overflow-hidden rounded-2xl border bg-[var(--color-bg-surface)] p-4 shadow-[0_4px_16px_rgba(15,23,42,0.06)] transition-all duration-200
        ${isSelected ? 'border-[var(--color-accent)] ring-1 ring-[var(--color-accent)]/20' : 'border-[var(--color-border-default)] hover:border-[var(--color-accent)]'}
      `}
    >
	  <div className="flex flex-col gap-3 border-b border-[var(--color-border-muted)]/60 pb-3 shrink-0">
        <div className="flex justify-between items-start gap-2">
		  <div className="flex min-w-0 items-center gap-2">
            <input
              type="checkbox"
              className="w-4 h-4 rounded cursor-pointer accent-[var(--color-accent)] mt-0.5 shrink-0"
              checked={isSelected}
              onChange={() => onToggleSelect(profile.profileId)}
            />
			<ProfileIconBadge badge={profile.iconBadge} color={profile.iconBadgeColor} size="sm" />
			<Link className="min-w-0 truncate text-sm font-semibold text-[var(--color-text-primary)] transition-colors hover:text-[var(--color-accent)]" to={`/browser/detail/${profile.profileId}`} title={profile.profileName}>
              {profile.profileName}
            </Link>
            {profile.tags && profile.tags.length > 0 && (
              <div className="flex gap-1 ml-1">
                {profile.tags.map(tag => <Badge variant="default" key={tag}>{tag}</Badge>)}
              </div>
            )}
          </div>

          <Badge variant={status.variant} dot dotClassName="w-2 h-2 shrink-0">
            {status.label}
          </Badge>
        </div>

		<div className="flex items-center gap-1.5 overflow-x-auto pb-0.5">
          {profile.running ? (
            <Button size="sm" variant="secondary" onClick={() => onStop(profile.profileId)} title={isStopping ? '停止中' : '停止'} loading={isStopping}>
              {!isStopping && <Square className="w-4 h-4 mr-1.5" />}
              {isStopping ? '停止中' : '停止'}
            </Button>
          ) : (
            <Button size="sm" onClick={() => onStart(profile.profileId)} title={isStarting ? '启动中' : '启动'} loading={isStarting}>
              {!isStarting && <Play className="w-4 h-4 fill-current mr-1.5" />}
              {isStarting ? '启动中' : '启动'}
            </Button>
          )}
		  <span className="mx-1 h-4 w-px shrink-0 bg-[var(--color-border-muted)]"></span>
		  <Button size="sm" variant="ghost" onClick={() => onRestart(profile.profileId)} title="重启" aria-label="重启" className="px-2.5" disabled={isBusy}><RotateCcw className="w-4 h-4" /></Button>
		  <Button size="sm" variant="ghost" onClick={() => onOpenKeywords(profile)} title="关键字管理" aria-label="关键字管理" className="px-2.5" disabled={isBusy}><Key className="w-4 h-4" /></Button>
		  <Button size="sm" variant="ghost" onClick={() => onOpenExtensions(profile)} title="插件配置" aria-label="插件配置" className="px-2.5" disabled={isBusy}><Puzzle className="w-4 h-4" /></Button>
		  <Link to={`/browser/edit/${profile.profileId}`}><Button size="sm" variant="ghost" title="配置" aria-label="配置" className="px-2.5" disabled={isBusy}><Settings className="w-4 h-4" /></Button></Link>
		  <Button size="sm" variant="ghost" onClick={() => onOpenCopy(profile)} title="克隆" aria-label="克隆" className="px-2.5" disabled={isBusy}><Copy className="w-4 h-4" /></Button>
		  <Button size="sm" variant="ghost" onClick={() => onDelete(profile.profileId)} title="删除" aria-label="删除" className="ml-auto px-2.5 text-red-500 hover:bg-red-50 hover:text-red-600" disabled={isBusy}><Trash2 className="w-4 h-4" /></Button>
        </div>
      </div>

	  <div className="grid grid-cols-2 gap-x-5 gap-y-3 py-3 shrink-0">
		<div className="min-w-0 rounded-lg bg-[var(--color-bg-secondary)] px-3 py-2">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">内核版本</span>
		  <span className="mt-1 block line-clamp-2 break-words text-xs leading-5 text-[var(--color-text-primary)]" title={coreLabel}>{coreLabel}</span>
        </div>
		<div className="min-w-0 rounded-lg bg-[var(--color-bg-secondary)] px-3 py-2">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">代理配置</span>
		  <div className="mt-1"><ProxyInlineActions
            profile={profile}
            proxy={proxy}
            isBusy={isBusy}
            onOpenProxyPicker={onOpenProxyPicker}
            maxWidthClass="max-w-full"
          /></div>
        </div>
		<div className="min-w-0 px-1">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">快捷配置码</span>
          <div className="mt-0.5"><LaunchCodeCell profileId={profile.profileId} code={profile.launchCode || ''} onRefresh={onRefreshProfiles} /></div>
        </div>
		<div className="min-w-0 px-1">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">上次更新时间</span>
		  <span className="mt-0.5 block whitespace-nowrap text-xs text-[var(--color-text-primary)]">{formatTime(profile.updatedAt)}</span>
        </div>
      </div>

      <div className="border-t border-[var(--color-border-muted)]/50 pt-2 flex items-start gap-2 flex-1 min-h-0">
        <span className="text-xs font-medium text-[var(--color-text-primary)] shrink-0 pt-0.5">系统关键字</span>
        <div className="flex-1 min-h-0 overflow-y-auto pr-1">
          <KeywordInlineRow keywords={profile.keywords || []} />
        </div>
      </div>
    </div>
  )
}

export function BrowserProfilesPanel({
  loading,
  viewMode,
  profiles,
  proxies,
  selectedIds,
  resolveProfileCore,
  getProfileCoreLabel,
  getProfileStatus,
  isProfileStarting,
  isProfileStopping,
  isProfileBusy,
  onToggleSelect,
  onSelectAll,
  onDeselectAll,
  onRefreshProfiles,
  onStart,
  onStop,
  onRestart,
  onOpenKeywords,
  onOpenExtensions,
  onExport,
  onOpenCopy,
  onOpenProxyPicker,
  onDelete,
}: BrowserProfilesPanelProps) {
  const allSelected = profiles.length > 0 && selectedIds.size === profiles.length
  const partiallySelected = selectedIds.size > 0 && selectedIds.size < profiles.length
  const [openMoreProfileId, setOpenMoreProfileId] = useState<string | null>(null)

  const columns: TableColumn<BrowserProfile>[] = [
    {
      key: 'selection',
      title: (
        <input
          type="checkbox"
          className="w-4 h-4 rounded cursor-pointer accent-[var(--color-accent)]"
          checked={allSelected}
          ref={(input) => {
            if (input) {
              input.indeterminate = partiallySelected
            }
          }}
          onChange={(event) => {
            if (event.target.checked) {
              onSelectAll()
            } else {
              onDeselectAll()
            }
          }}
        />
      ),
      width: 40,
      render: (_, record) => (
        <input
          type="checkbox"
          className="w-4 h-4 rounded cursor-pointer accent-[var(--color-accent)]"
          checked={selectedIds.has(record.profileId)}
          onChange={() => onToggleSelect(record.profileId)}
        />
      ),
    },
    {
      key: 'profileName',
      title: '实例名称',
		  width: 240,
      render: (value, record) => (
        <div className="flex min-w-[260px] items-center gap-3">
          <ProfileIconBadge badge={record.iconBadge} color={record.iconBadgeColor} size="sm" />
          <div className="min-w-0 flex-1">
            <Link className="block truncate whitespace-nowrap text-[var(--color-accent)] text-sm font-medium hover:underline" to={`/browser/detail/${record.profileId}`} title={String(value || '')}>
              {value}
            </Link>
            {record.tags && record.tags.length > 0 && (
              <div className="mt-1 flex gap-1 flex-wrap">
                {record.tags.map(tag => <Badge variant="default" key={tag}>{tag}</Badge>)}
              </div>
            )}
          </div>
        </div>
      ),
    },
    {
      key: 'running',
      title: '状态',
		  width: 96,
      render: (_, record) => {
        const status = getProfileStatus(record)
		return <Badge variant={status.variant} dot className="whitespace-nowrap">{status.label}</Badge>
      },
    },
	  {
		key: 'coreId',
		title: '核心',
		width: 230,
		render: (_, record) => {
		  const label = getProfileCoreLabel(record)
		  return <span className="line-clamp-2 break-words text-xs leading-5" title={label}>{label}</span>
		},
	  },
    {
		key: 'proxyId',
		title: '代理',
		width: 250,
      render: (value, record) => {
        const proxy = proxies.find(item => item.proxyId === value)
        const isBusy = isProfileBusy(record.profileId)
		return <ProxyInlineActions profile={record} proxy={proxy} isBusy={isBusy} onOpenProxyPicker={onOpenProxyPicker} maxWidthClass="max-w-[230px]" />
      },
    },
    {
		key: 'launchCode',
		title: '快捷打开码',
		width: 180,
      render: (value, record) => <LaunchCodeCell profileId={record.profileId} code={value || ''} onRefresh={onRefreshProfiles} />,
    },
    {
      key: 'keywords',
      title: '关键字',
		width: 180,
      render: (value) => <KeywordInlineRow keywords={value || []} />,
    },
    {
		key: 'updatedAt',
		title: '上次更新',
		width: 160,
		render: (value) => <span className="whitespace-nowrap text-xs">{formatTime(value)}</span>,
    },
    {
      key: 'actions',
      title: '操作',
		width: 260,
      align: 'right',
      render: (_, record) => {
        const isStarting = isProfileStarting(record.profileId)
        const isStopping = isProfileStopping(record.profileId)
        const isBusy = isProfileBusy(record.profileId)
        const isMoreOpen = openMoreProfileId === record.profileId

        return (
          <div className="flex justify-end gap-1.5 whitespace-nowrap">
            {record.running ? (
              <Button size="sm" variant="secondary" onClick={() => onStop(record.profileId)} title="停止" loading={isStopping}>
                {!isStopping && <Square className="w-3.5 h-3.5" />}
              </Button>
            ) : (
              <Button size="sm" onClick={() => onStart(record.profileId)} title="启动" loading={isStarting}>
                {!isStarting && <Play className="w-3.5 h-3.5 fill-current" />}
              </Button>
            )}
            <Link to={`/browser/edit/${record.profileId}`}><Button size="sm" variant="ghost" title="配置" disabled={isBusy}><Settings className="w-3.5 h-3.5" /></Button></Link>
            <Button size="sm" variant="ghost" onClick={() => onOpenCopy(record)} title="克隆" disabled={isBusy}><Copy className="w-3.5 h-3.5" /></Button>
            <ProfileMoreActions
              open={isMoreOpen}
              disabled={isBusy}
              onToggle={() => setOpenMoreProfileId(isMoreOpen ? null : record.profileId)}
              onClose={() => setOpenMoreProfileId(null)}
              onRestart={() => onRestart(record.profileId)}
              onOpenKeywords={() => onOpenKeywords(record)}
              onOpenExtensions={() => onOpenExtensions(record)}
              onExport={() => onExport(record)}
            />
            <Button size="sm" variant="ghost" onClick={() => onDelete(record.profileId)} title="删除" disabled={isBusy}><Trash2 className="w-3.5 h-3.5 text-red-500" /></Button>
          </div>
        )
      },
    },
  ]

  return (
    <Card padding="none">
	  <div className={viewMode === 'card' ? 'overflow-auto' : undefined} style={viewMode === 'card' ? { maxHeight: 'calc(100vh - 320px)' } : undefined}>
        {loading ? (
          <div className="py-16 flex items-center justify-center text-sm text-[var(--color-text-muted)]">加载中...</div>
        ) : profiles.length === 0 ? (
          <div className="py-16 flex items-center justify-center text-sm text-[var(--color-text-muted)]">暂无数据</div>
        ) : viewMode === 'table' ? (
			  <Table
                columns={columns}
                data={profiles}
                rowKey="profileId"
				tableClassName="min-w-[1640px] table-fixed"
              />
        ) : (
		  <div className="grid min-h-[420px] grid-cols-[repeat(auto-fill,minmax(min(100%,520px),1fr))] items-start gap-4 p-4">
            {profiles.map((profile) => (
			  <div key={profile.profileId} className="min-w-0">
                <BrowserProfileCard
                  profile={profile}
                  proxy={proxies.find(item => item.proxyId === profile.proxyId)}
                  isSelected={selectedIds.has(profile.profileId)}
                  status={getProfileStatus(profile)}
                  coreLabel={resolveProfileCore(profile)?.coreName || getProfileCoreLabel(profile)}
                  isStarting={isProfileStarting(profile.profileId)}
                  isStopping={isProfileStopping(profile.profileId)}
                  isBusy={isProfileBusy(profile.profileId)}
                  onToggleSelect={onToggleSelect}
                  onRefreshProfiles={onRefreshProfiles}
                  onStart={onStart}
                  onStop={onStop}
                  onRestart={onRestart}
                  onOpenKeywords={onOpenKeywords}
                  onOpenExtensions={onOpenExtensions}
                  onOpenCopy={onOpenCopy}
                  onOpenProxyPicker={onOpenProxyPicker}
                  onDelete={onDelete}
                />
              </div>
            ))}
          </div>
        )}
      </div>
    </Card>
  )
}
