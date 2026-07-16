import { useEffect, useState } from 'react'
import { ChevronDown, ChevronUp, RefreshCw, Wand2 } from 'lucide-react'
import { ConfirmModal, FormItem, Input, Select, Textarea } from '../../../shared/components'
import { normalizeFingerprintArgs } from '../api/cores'
import type { FingerprintArgResult } from '../types'
import {
  type FingerprintConfig,
  FINGERPRINT_PRESETS,
  PRESET_RESOLUTIONS,
  deserialize,
  getSystemTimezone,
	randomFingerprintSeed,
	serialize,
	applyFingerprintCapabilities,
	hasDisabledSpoofingFeature,
	setDisabledSpoofingFeature,
	withoutLegacyGpuArgs,
} from '../utils/fingerprintSerializer'

interface FingerprintPanelProps {
  value: string[]
  onChange: (args: string[]) => void
	coreId?: string
	chromiumMajor?: number
}

const BRAND_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'Chrome', label: 'Chrome' },
  { value: 'Edge', label: 'Edge' },
	{ value: 'Opera', label: 'Opera' },
	{ value: 'Vivaldi', label: 'Vivaldi' },
	{ value: 'Firefox', label: 'Firefox' },
	{ value: 'Safari', label: 'Safari' },
	{ value: 'custom', label: '自定义品牌...' },
]

const KNOWN_BRANDS = new Set(BRAND_OPTIONS.map(option => option.value).filter(value => value && value !== 'custom'))

const SOURCE_LABELS: Record<string, string> = {
	user: '用户配置',
	'compatibility-migration': '兼容性迁移',
	'core-capability-adjustment': '内核能力调整',
}

const PLATFORM_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'windows', label: 'Windows' },
	{ value: 'macos', label: 'macOS' },
  { value: 'linux', label: 'Linux' },
]

const LANG_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'zh-CN', label: '中文 (zh-CN)' },
  { value: 'zh-HK', label: '繁體中文香港 (zh-HK)' },
  { value: 'zh-TW', label: '繁體中文台灣 (zh-TW)' },
  { value: 'en-US', label: 'English US (en-US)' },
  { value: 'en-GB', label: 'English UK (en-GB)' },
  { value: 'en-CA', label: 'English Canada (en-CA)' },
  { value: 'en-AU', label: 'English Australia (en-AU)' },
  { value: 'en-SG', label: 'English Singapore (en-SG)' },
  { value: 'en-IN', label: 'English India (en-IN)' },
  { value: 'ja-JP', label: '日本語 (ja-JP)' },
  { value: 'ko-KR', label: '한국어 (ko-KR)' },
  { value: 'fr-FR', label: 'Français (fr-FR)' },
  { value: 'de-DE', label: 'Deutsch (de-DE)' },
  { value: 'nl-NL', label: 'Nederlands (nl-NL)' },
  { value: 'ru-RU', label: 'Русский (ru-RU)' },
  { value: 'pt-BR', label: 'Português Brasil (pt-BR)' },
]

const TIMEZONE_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'system', label: '跟随系统时区' },
  // 亚洲
  { value: 'Asia/Shanghai', label: 'Asia/Shanghai (UTC+8)' },
  { value: 'Asia/Tokyo', label: 'Asia/Tokyo (UTC+9)' },
  { value: 'Asia/Seoul', label: 'Asia/Seoul (UTC+9)' },
  { value: 'Asia/Singapore', label: 'Asia/Singapore (UTC+8)' },
  { value: 'Asia/Hong_Kong', label: 'Asia/Hong_Kong (UTC+8)' },
  { value: 'Asia/Taipei', label: 'Asia/Taipei (UTC+8)' },
  { value: 'Asia/Dubai', label: 'Asia/Dubai (UTC+4)' },
  { value: 'Asia/Kolkata', label: 'Asia/Kolkata (UTC+5:30)' },
  // 美洲
  { value: 'America/New_York', label: 'America/New_York (UTC-5)' },
  { value: 'America/Los_Angeles', label: 'America/Los_Angeles (UTC-8)' },
  { value: 'America/Chicago', label: 'America/Chicago (UTC-6)' },
  { value: 'America/Denver', label: 'America/Denver (UTC-7)' },
  { value: 'America/Toronto', label: 'America/Toronto (UTC-5)' },
  { value: 'America/Vancouver', label: 'America/Vancouver (UTC-8)' },
  { value: 'America/Phoenix', label: 'America/Phoenix (UTC-7)' },
  { value: 'America/Sao_Paulo', label: 'America/Sao_Paulo (UTC-3)' },
  // EMEA
  { value: 'Europe/London', label: 'Europe/London (UTC+0)' },
  { value: 'Europe/Paris', label: 'Europe/Paris (UTC+1)' },
  { value: 'Europe/Berlin', label: 'Europe/Berlin (UTC+1)' },
  { value: 'Europe/Moscow', label: 'Europe/Moscow (UTC+3)' },
  // 大洋洲
  { value: 'Australia/Sydney', label: 'Australia/Sydney (UTC+10)' },
  { value: 'Australia/Melbourne', label: 'Australia/Melbourne (UTC+10)' },
  { value: 'Australia/Perth', label: 'Australia/Perth (UTC+8)' },
  { value: 'Pacific/Auckland', label: 'Pacific/Auckland (UTC+12)' },
]

export function currentTimezoneOffsetLabel(timeZone: string, date = new Date()): string {
  try {
    const part = new Intl.DateTimeFormat('en-US', {
      timeZone,
      timeZoneName: 'longOffset',
    }).formatToParts(date).find(item => item.type === 'timeZoneName')?.value || ''
    const normalized = part.replace(/^GMT/, 'UTC')
    if (normalized === 'UTC') return 'UTC+0'
    const match = normalized.match(/^UTC([+-])(\d{2})(?::(\d{2}))?$/)
    if (!match) return normalized || 'UTC'
    const hours = String(Number(match[2]))
    return `UTC${match[1]}${hours}${match[3] && match[3] !== '00' ? `:${match[3]}` : ''}`
  } catch {
    return 'UTC'
  }
}

const RESOLUTION_OPTIONS = [
  { value: '', label: '不设置' },
  ...PRESET_RESOLUTIONS.map(r => ({ value: r, label: r })),
  { value: 'custom', label: '自定义...' },
]

const BOOL_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'true', label: '启用' },
  { value: 'false', label: '禁用' },
]

const HARDWARE_CONCURRENCY_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '2', label: '2 核' },
  { value: '4', label: '4 核' },
  { value: '6', label: '6 核' },
  { value: '8', label: '8 核' },
  { value: '10', label: '10 核' },
  { value: '12', label: '12 核' },
  { value: '16', label: '16 核' },
]

const DEVICE_MEMORY_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '2', label: '2 GB' },
  { value: '4', label: '4 GB' },
  { value: '8', label: '8 GB' },
  { value: '16', label: '16 GB' },
  { value: '32', label: '32 GB' },
]

const COLOR_DEPTH_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '24', label: '24 位（标准）' },
  { value: '30', label: '30 位（HDR）' },
  { value: '32', label: '32 位' },
]

const WEBRTC_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'disable_non_proxied_udp', label: '禁用非代理 UDP（推荐）' },
  { value: 'default_public_interface_only', label: '仅公网接口' },
  { value: 'default_public_and_private_interfaces', label: '公网+私网接口' },
]

const TOUCH_POINTS_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '0', label: '0（无触摸）' },
  { value: '1', label: '1 点触摸' },
  { value: '5', label: '5 点触摸' },
  { value: '10', label: '10 点触摸' },
]

const PRESET_OPTIONS = [
  { value: '', label: '选择预设...' },
  ...FINGERPRINT_PRESETS.map(p => ({ value: p.id, label: p.name })),
]

export function FingerprintPanel({ value, onChange, coreId = '' }: FingerprintPanelProps) {
  const [config, setConfig] = useState<FingerprintConfig>(() => deserialize(value))
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [confirmSeedOpen, setConfirmSeedOpen] = useState(false)
	const [preview, setPreview] = useState<FingerprintArgResult | null>(null)

  useEffect(() => {
    setConfig(deserialize(value))
  }, [value.join('\n')])

	useEffect(() => {
		let cancelled = false
		normalizeFingerprintArgs(coreId, config.platform || '', value).then(result => {
			if (!cancelled) setPreview(result)
		}).catch(() => {
			if (!cancelled) setPreview(null)
		})
		return () => { cancelled = true }
	}, [coreId, config.platform, value.join('\n')])

  const update = (patch: Partial<FingerprintConfig>) => {
		const next = { ...config, ...patch }
    setConfig(next)
		onChange(applyFingerprintCapabilities(serialize(next)))
  }

  const handlePresetChange = (presetId: string) => {
    if (!presetId) return
    const preset = FINGERPRINT_PRESETS.find(p => p.id === presetId)
    if (!preset) return
    // 应用预设时自动生成新种子，保留未知参数
		const next: FingerprintConfig = {
      ...preset.config,
      seed: randomFingerprintSeed(),
			unknownArgs: withoutLegacyGpuArgs(config.unknownArgs),
    }
    setConfig(next)
		onChange(applyFingerprintCapabilities(serialize(next)))
  }

  const handleAdvancedChange = (text: string) => {
    const args = text.split('\n').map(s => s.trim()).filter(Boolean)
    const parsed = deserialize(args)
    setConfig(parsed)
		onChange(applyFingerprintCapabilities(serialize(parsed)))
  }

	const advancedText = (preview?.args || applyFingerprintCapabilities(serialize(config))).join('\n')
	const brandSelectValue = config.brand && !KNOWN_BRANDS.has(config.brand) ? 'custom' : (config.brand || '')
	const useRealGpu = hasDisabledSpoofingFeature(config.unknownArgs, 'gpu')
	const setGpuPolicy = (realGpu: boolean) => update({ unknownArgs: setDisabledSpoofingFeature(config.unknownArgs, 'gpu', realGpu) })

  return (
    <div className="space-y-4">
      {/* 指纹种子 */}
      <div className="p-3 rounded-lg bg-[var(--color-bg-hover)] border border-[var(--color-border)] space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wide">指纹种子（Fingerprint Seed）</span>
          <span className="text-xs text-[var(--color-text-muted)]">决定所有随机噪声的根值，不同种子 = 不同指纹</span>
        </div>
        <div className="flex items-center gap-2">
          <Input
            value={config.seed ?? ''}
            onChange={e => update({ seed: e.target.value || undefined })}
            placeholder="留空则由系统按 ProfileId 自动生成"
            className="flex-1 font-mono text-sm"
          />
          <button
            type="button"
            title="随机生成新种子"
            onClick={() => {
              if (config.seed) {
                setConfirmSeedOpen(true)
              } else {
                update({ seed: randomFingerprintSeed() })
              }
            }}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[var(--color-primary)] text-white hover:opacity-90 transition-opacity shrink-0"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            随机
          </button>
		</div>
	  </div>

      <ConfirmModal
        open={confirmSeedOpen}
        onClose={() => setConfirmSeedOpen(false)}
        onConfirm={() => update({ seed: randomFingerprintSeed() })}
        title="重新生成指纹种子"
        content="重新生成后，当前指纹将完全改变，浏览器的 Canvas、WebGL、Audio 等所有噪声特征都会随之变化。确定继续？"
        confirmText="确定重新生成"
        danger
      />

      {/* 预设选择 */}
      <div className="flex items-center gap-3 p-3 rounded-lg bg-[var(--color-bg-hover)] border border-[var(--color-border)]">
        <Wand2 className="w-4 h-4 text-[var(--color-text-muted)] shrink-0" />
        <div className="flex-1 min-w-0">
			<Select
            value=""
            onChange={e => handlePresetChange(e.target.value)}
            options={PRESET_OPTIONS}
          />
        </div>
        <span className="text-xs text-[var(--color-text-muted)] shrink-0">选择后覆盖当前配置</span>
      </div>

      {/* 基础身份 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">基础身份</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="浏览器品牌">
			<Select
				value={brandSelectValue}
				onChange={e => update({ brand: e.target.value === 'custom' ? 'Custom' : (e.target.value || undefined) })}
				options={BRAND_OPTIONS}
			/>
          </FormItem>
			{brandSelectValue === 'custom' && (
					<FormItem label="自定义品牌">
					<Input value={config.brand || ''} onChange={e => update({ brand: e.target.value || undefined })} placeholder="Custom" />
				</FormItem>
			)}
          <FormItem label="操作系统">
            <Select value={config.platform ?? ''} onChange={e => update({ platform: e.target.value || undefined })} options={PLATFORM_OPTIONS} />
          </FormItem>
		  <FormItem label="网页首选语言" hint="控制网页请求、navigator.language、--lang 和 --accept-lang">
            <Select value={config.lang ?? ''} onChange={e => update({ lang: e.target.value || undefined })} options={LANG_OPTIONS} />
          </FormItem>
          <FormItem label="时区">
            <Select value={config.timezone ?? ''} onChange={e => update({ timezone: e.target.value || undefined })} options={TIMEZONE_OPTIONS.map(opt => {
              if (opt.value === 'system') return { ...opt, label: `跟随系统时区 (当前: ${getSystemTimezone()})` }
              if (!opt.value) return opt
              return { ...opt, label: `${opt.value} (${currentTimezoneOffsetLabel(opt.value)})` }
            })} />
          </FormItem>
		</div>
      </div>

      {/* 屏幕与硬件 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">屏幕与硬件</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
			<FormItem label="启动窗口大小">
            <Select
              value={config.resolution ?? ''}
              onChange={e => update({ resolution: e.target.value || undefined, customResolution: undefined })}
              options={RESOLUTION_OPTIONS}
            />
          </FormItem>
          {config.resolution === 'custom' && (
			<FormItem label="自定义启动窗口大小">
              <Input value={config.customResolution ?? ''} onChange={e => update({ customResolution: e.target.value || undefined })} placeholder="1600,900" />
            </FormItem>
          )}
          <FormItem label="色深">
            <Select value={config.colorDepth ?? ''} onChange={e => update({ colorDepth: e.target.value || undefined })} options={COLOR_DEPTH_OPTIONS} />
          </FormItem>
          <FormItem label="CPU 核心数">
            <Select value={config.hardwareConcurrency ?? ''} onChange={e => update({ hardwareConcurrency: e.target.value || undefined })} options={HARDWARE_CONCURRENCY_OPTIONS} />
          </FormItem>
          <FormItem label="设备内存">
            <Select value={config.deviceMemory ?? ''} onChange={e => update({ deviceMemory: e.target.value || undefined })} options={DEVICE_MEMORY_OPTIONS} />
          </FormItem>
          <FormItem label="触摸点数">
            <Select value={config.touchPoints ?? ''} onChange={e => update({ touchPoints: e.target.value || undefined })} options={TOUCH_POINTS_OPTIONS} />
          </FormItem>
        </div>
      </div>

      {/* 渲染指纹 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">渲染指纹</p>
		<div className="space-y-4">
		  <FormItem label="GPU 指纹策略" hint="内核自动模拟会由指纹种子稳定选择一组真实 GPU 参数；使用真实 GPU 会关闭内核的 GPU 模拟。">
			<div className="grid grid-cols-2 gap-1 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-hover)] p-1" role="group" aria-label="GPU 指纹策略">
			  <button
				type="button"
				aria-pressed={!useRealGpu}
				onClick={() => setGpuPolicy(false)}
				className={`min-h-11 rounded-md px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)] ${!useRealGpu ? 'bg-[var(--color-bg-card)] text-[var(--color-text)] shadow-sm' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text)]'}`}
			  >
				内核自动模拟
			  </button>
			  <button
				type="button"
				aria-pressed={useRealGpu}
				onClick={() => setGpuPolicy(true)}
				className={`min-h-11 rounded-md px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)] ${useRealGpu ? 'bg-[var(--color-bg-card)] text-[var(--color-text)] shadow-sm' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text)]'}`}
			  >
				使用真实 GPU
			  </button>
			</div>
		  </FormItem>
	        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="Canvas 噪声">
            <Select
              value={config.canvasNoise === undefined ? '' : String(config.canvasNoise)}
              onChange={e => { const v = e.target.value; update({ canvasNoise: v === '' ? undefined : v === 'true' }) }}
              options={BOOL_OPTIONS}
            />
          </FormItem>
          <FormItem label="Audio 噪声">
            <Select
              value={config.audioNoise === undefined ? '' : String(config.audioNoise)}
              onChange={e => { const v = e.target.value; update({ audioNoise: v === '' ? undefined : v === 'true' }) }}
              options={BOOL_OPTIONS}
            />
          </FormItem>
        </div>
		</div>
      </div>

      {/* 网络与隐私 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">网络与隐私</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="WebRTC 策略">
            <Select value={config.webrtcPolicy ?? ''} onChange={e => update({ webrtcPolicy: e.target.value || undefined })} options={WEBRTC_OPTIONS} />
          </FormItem>
          <FormItem label="Do Not Track">
            <Select
              value={config.doNotTrack === undefined ? '' : String(config.doNotTrack)}
              onChange={e => { const v = e.target.value; update({ doNotTrack: v === '' ? undefined : v === 'true' }) }}
              options={BOOL_OPTIONS}
            />
          </FormItem>
          <FormItem label="媒体设备 (摄像头,麦克风,扬声器)">
            <Input
              value={config.mediaDevices ?? ''}
              onChange={e => update({ mediaDevices: e.target.value || undefined })}
              placeholder="2,1,1"
            />
          </FormItem>
        </div>
      </div>

      {/* 字体 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">字体</p>
		<FormItem label="字体列表">
          <Input
            value={config.fonts ?? ''}
            onChange={e => update({ fonts: e.target.value || undefined })}
            placeholder="Arial,Helvetica,Times New Roman（逗号分隔）"
          />
		</FormItem>
      </div>

      {/* 高级模式 */}
      <div className="border border-[var(--color-border)] rounded-lg overflow-hidden">
        <button
          type="button"
          className="w-full flex items-center justify-between px-4 py-2.5 text-sm text-[var(--color-text-muted)] hover:bg-[var(--color-bg-hover)] transition-colors"
          onClick={() => setAdvancedOpen(v => !v)}
        >
          <span>高级模式（原始参数）</span>
          {advancedOpen ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
        </button>
		{advancedOpen && (
          <div className="px-4 pb-4 pt-2 border-t border-[var(--color-border)]">
			<p className="text-xs text-[var(--color-text-muted)] mb-2">每行一个参数。旧 Profile 的 GPU/WebGL 参数会在这里原样保留；应用内置预设时才会清理。</p>
            <Textarea
              value={advancedText}
              onChange={e => handleAdvancedChange(e.target.value)}
              rows={6}
              placeholder="--fingerprint-brand=Chrome"
            />
          </div>
		)}
		</div>
		<div className="border border-[var(--color-border)] rounded-lg p-3">
			<p className="text-xs font-medium text-[var(--color-text-muted)] mb-2">最终启动参数</p>
			{preview?.warnings?.map(warning => <p key={warning} className="mb-2 text-xs text-[var(--color-warning)]">{warning}</p>)}
			{preview?.entries?.length ? (
				<div className="space-y-1.5">
					{preview.entries.map(entry => (
						<div key={entry.arg} className="flex items-start justify-between gap-3 text-xs">
							<code className="min-w-0 break-all text-[var(--color-text-secondary)]">{entry.arg}</code>
							<span className="shrink-0 text-[var(--color-text-muted)]">{SOURCE_LABELS[entry.source] || entry.source}</span>
						</div>
					))}
				</div>
			) : <pre className="text-xs leading-5 whitespace-pre-wrap break-all text-[var(--color-text-secondary)]">{advancedText || '未配置指纹参数'}</pre>}
		</div>
	</div>
  )
}
