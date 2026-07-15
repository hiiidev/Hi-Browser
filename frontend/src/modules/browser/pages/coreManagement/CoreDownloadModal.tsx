import type { Dispatch, SetStateAction } from 'react'
import { Check, Download, Globe2, Network, Route, ShieldCheck } from 'lucide-react'
import { Button, FormItem, Input, Modal, toast } from '../../../../shared/components'
import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime'
import type { BrowserProxy } from '../../types'
import type { CoreDownloadForm, CoreDownloadProgress } from '../coreManagement.types'

interface CoreDownloadModalProps {
  open: boolean
  form: CoreDownloadForm
  progress: CoreDownloadProgress | null
  proxies: BrowserProxy[]
  setForm: Dispatch<SetStateAction<CoreDownloadForm>>
  setProgress: Dispatch<SetStateAction<CoreDownloadProgress | null>>
  onClose: () => void
  onStart: () => void
  onCancelTask?: () => void
  onRetry?: () => void
}

const proxyModes = [
  { value: 'system', label: '系统代理', detail: '跟随当前系统网络设置', icon: Globe2 },
  { value: 'direct', label: '直连', detail: '不使用任何代理', icon: Route },
  { value: 'custom', label: '应用代理', detail: '使用代理池中的指定节点', icon: Network },
  { value: 'gh-proxy', label: 'GitHub 加速', detail: 'https://gh-proxy.com/', icon: ShieldCheck },
] as const

function formatBytes(value?: number) {
  if (!value) return '-'
  return `${(value / 1048576).toFixed(1)} MB`
}

export function CoreDownloadModal({
  open,
  form,
  progress,
  proxies,
  setForm,
  setProgress,
  onClose,
  onStart,
  onCancelTask,
  onRetry,
}: CoreDownloadModalProps) {
  const terminal = progress?.phase === 'error'
    || progress?.phase === 'failed'
    || progress?.phase === 'done'
    || progress?.phase === 'completed'
    || progress?.phase === 'cancelled'
  const downloading = progress !== null && !terminal
  const isRedownload = form.mode === 'redownload'
  const managedRelease = Boolean(form.releaseTag)

  const handleClose = () => {
    if (progress && !terminal) {
      toast.warning('正在下载中，请先取消任务')
      return
    }
    onClose()
    setProgress(null)
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={isRedownload ? '重新下载内核' : '下载浏览器内核'}
      width="600px"
      footer={(
        <>
          <Button variant="secondary" onClick={downloading && onCancelTask ? onCancelTask : handleClose}>
            {downloading ? '取消任务' : '关闭'}
          </Button>
          {progress?.canRetry && onRetry ? (
            <Button onClick={onRetry}>重试</Button>
          ) : (
            <Button onClick={onStart} loading={downloading} disabled={Boolean(progress) && !terminal}>
              <Download className="h-4 w-4" />
              {isRedownload ? '开始重新下载' : '开始下载'}
            </Button>
          )}
        </>
      )}
    >
      <div className="space-y-5">
        {managedRelease ? (
          <section className="border-b border-[var(--color-border)] pb-4">
            <div className="flex items-start justify-between gap-5">
              <div className="min-w-0">
                <div className="text-sm font-semibold text-[var(--color-text-primary)]">{form.name}</div>
                <div className="mt-1 truncate text-xs text-[var(--color-text-muted)]" title={form.assetName}>
                  {form.assetName}
                </div>
              </div>
              <div className="shrink-0 text-right text-xs text-[var(--color-text-muted)]">
                <div>{form.platform}/{form.architecture}</div>
                <div className="mt-1 font-medium text-[var(--color-text-secondary)]">{formatBytes(form.assetSize)}</div>
              </div>
            </div>
          </section>
        ) : (
          <div className="space-y-4">
            <FormItem label="内核名称" required>
              <Input
                value={form.name}
                onChange={event => setForm(previous => ({ ...previous, name: event.target.value }))}
                placeholder="例如: chrome-139"
                disabled={progress !== null || isRedownload}
              />
              {!isRedownload && <p className="mt-1.5 text-xs text-[var(--color-text-muted)]">同时作为内核数据目录名称。</p>}
            </FormItem>

            {isRedownload && (
              <div className="border-l-2 border-[var(--color-warning)] bg-[var(--color-warning)]/10 px-3 py-2 text-xs leading-5 text-[var(--color-text-secondary)]">
                新包验证成功后才会替换当前内核。请先停止正在使用该内核的实例。
              </div>
            )}

            <FormItem label="下载地址" required>
              <Input
                value={form.url}
                onChange={event => setForm(previous => ({ ...previous, url: event.target.value }))}
                placeholder="https://github.com/.../chrome-macos.zip"
                disabled={progress !== null}
              />
              <div className="mt-2 flex items-center justify-between gap-4 text-xs text-[var(--color-text-muted)]">
                <span>推荐内核：fingerprint-chromium</span>
                <button
                  type="button"
                  onClick={() => BrowserOpenURL('https://github.com/adryfish/fingerprint-chromium/releases')}
                  className="shrink-0 font-medium text-[var(--color-accent)] hover:underline"
                >
                  查看 Releases
                </button>
              </div>
            </FormItem>
          </div>
        )}

        <section>
          <div className="mb-2 text-sm font-medium text-[var(--color-text-primary)]">下载方式</div>
          <div className="grid grid-cols-2 gap-2">
            {proxyModes.map((option) => {
              const Icon = option.icon
              const selected = form.proxyMode === option.value
              const disabled = progress !== null || (option.value === 'custom' && proxies.length === 0)
              return (
                <button
                  key={option.value}
                  type="button"
                  disabled={disabled}
                  onClick={() => setForm(previous => ({
                    ...previous,
                    proxyMode: option.value,
                    proxyId: option.value === 'custom' ? previous.proxyId || proxies[0]?.proxyId || '' : '',
                  }))}
                  className={`relative flex min-h-[70px] items-start gap-3 rounded-md border px-3 py-3 text-left transition-colors ${
                    selected
                      ? 'border-[var(--color-accent)] bg-[var(--color-accent)]/10'
                      : 'border-[var(--color-border-default)] bg-[var(--color-bg-primary)] hover:border-[var(--color-border-strong)]'
                  } disabled:cursor-not-allowed disabled:opacity-50`}
                >
                  <Icon className={`mt-0.5 h-4 w-4 shrink-0 ${selected ? 'text-[var(--color-accent)]' : 'text-[var(--color-text-muted)]'}`} />
                  <span className="min-w-0">
                    <span className="block text-sm font-medium text-[var(--color-text-primary)]">{option.label}</span>
                    <span className="mt-0.5 block break-all text-xs leading-4 text-[var(--color-text-muted)]">{option.detail}</span>
                  </span>
                  {selected && <Check className="absolute right-2 top-2 h-3.5 w-3.5 text-[var(--color-accent)]" />}
                </button>
              )
            })}
          </div>
        </section>

        {form.proxyMode === 'custom' && (
          <FormItem label="代理池节点" required>
            <select
              value={form.proxyId}
              onChange={event => setForm(previous => ({ ...previous, proxyId: event.target.value }))}
              className="h-10 w-full rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-primary)] px-3 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
              disabled={progress !== null}
            >
              {proxies.map(proxy => <option key={proxy.proxyId} value={proxy.proxyId}>{proxy.proxyName}</option>)}
            </select>
            <p className="mt-1.5 text-xs text-[var(--color-text-muted)]">使用当前连接栈启动该节点，不会在 Xray 组合栈和 Mihomo 之间自动切换。</p>
          </FormItem>
        )}

        {form.proxyMode === 'gh-proxy' && (
          <div className="flex gap-2 border-l-2 border-[var(--color-accent)] bg-[var(--color-bg-muted)] px-3 py-2 text-xs leading-5 text-[var(--color-text-secondary)]">
            <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-[var(--color-accent)]" />
            <span>下载地址会临时转为 <strong>https://gh-proxy.com/</strong> 前缀。这是第三方服务，请自行确认可用性与信任风险；下载后仍执行项目现有的 SHA-256 校验。</span>
          </div>
        )}

        {progress && (
          <section className="border-t border-[var(--color-border)] pt-4">
            <div className="mb-2 flex items-center justify-between gap-3 text-sm">
              <span className="min-w-0 truncate font-medium text-[var(--color-text-primary)]">{progress.message}</span>
              <span className="shrink-0 tabular-nums text-[var(--color-text-muted)]">{progress.progress}%</span>
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--color-bg-muted)]">
              <div className="h-full rounded-full bg-[var(--color-accent)] transition-[width] duration-300" style={{ width: `${Math.max(0, Math.min(100, progress.progress))}%` }} />
            </div>
            {Boolean(progress.totalBytes) && (
              <div className="mt-2 text-xs tabular-nums text-[var(--color-text-muted)]">
                {(Number(progress.downloadedBytes || 0) / 1048576).toFixed(1)} / {(Number(progress.totalBytes || 0) / 1048576).toFixed(1)} MB
                {' · '}{(Number(progress.speedBytesPerSecond || 0) / 1048576).toFixed(1)} MB/s
              </div>
            )}
            {progress.errorDetail && <pre className="mt-3 max-h-28 overflow-auto whitespace-pre-wrap text-xs text-[var(--color-danger)]">{progress.errorDetail}</pre>}
          </section>
        )}
      </div>
    </Modal>
  )
}
