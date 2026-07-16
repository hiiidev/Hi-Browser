// 指纹参数序列化/反序列化工具

/**
 * 获取系统当前时区
 * @returns IANA 时区标识符，如 "Asia/Shanghai"
 */
export function getSystemTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

export interface FingerprintConfig {
  // 指纹种子（核心）
  seed?: string            // --fingerprint=<seed>  控制所有随机噪声的根种子

  // 基础身份
  brand?: string           // --fingerprint-brand=
  platform?: string        // --fingerprint-platform=
  lang?: string            // --lang=
	acceptLang?: string      // --accept-lang=
  timezone?: string        // --timezone=

  // 屏幕与窗口
  resolution?: string      // --window-size=（预设值或 'custom'）
  customResolution?: string // 当 resolution === 'custom' 时使用
  colorDepth?: string      // --fingerprint-color-depth=

  // 硬件信息
  hardwareConcurrency?: string  // --fingerprint-hardware-concurrency=
  deviceMemory?: string         // --fingerprint-device-memory=

  // 渲染指纹
  canvasNoise?: boolean         // --fingerprint-canvas-noise=
  webglVendor?: string          // --fingerprint-webgl-vendor=
  webglRenderer?: string        // --fingerprint-webgl-renderer=
  audioNoise?: boolean          // --fingerprint-audio-noise=

  // 字体
  fonts?: string                // --fingerprint-fonts=

  // 网络与隐私
  webrtcPolicy?: string         // --webrtc-ip-handling-policy=
  doNotTrack?: boolean          // --fingerprint-do-not-track=

  // 媒体设备
  mediaDevices?: string         // --fingerprint-media-devices= (格式: "2,1,0" 摄像头,麦克风,扬声器)

  // 触摸
  touchPoints?: string          // --fingerprint-touch-points=

  unknownArgs?: string[]        // 无法识别的原始参数，原样保留
}

export const PRESET_RESOLUTIONS = ['1920,1080', '1440,900', '1366,768', '2560,1440', '1280,800', '1600,900']

// CLI 参数前缀 → FingerprintConfig 字段映射
export const KEY_MAP: Record<string, keyof FingerprintConfig> = {
  '--fingerprint': 'seed',
  '--fingerprint-brand': 'brand',
  '--fingerprint-platform': 'platform',
  '--lang': 'lang',
	'--accept-lang': 'acceptLang',
  '--timezone': 'timezone',
  '--window-size': 'resolution',
  '--fingerprint-color-depth': 'colorDepth',
  '--fingerprint-hardware-concurrency': 'hardwareConcurrency',
  '--fingerprint-device-memory': 'deviceMemory',
  '--fingerprint-canvas-noise': 'canvasNoise',
  '--fingerprint-webgl-vendor': 'webglVendor',
  '--fingerprint-webgl-renderer': 'webglRenderer',
  '--fingerprint-audio-noise': 'audioNoise',
  '--fingerprint-fonts': 'fonts',
  '--webrtc-ip-handling-policy': 'webrtcPolicy',
  '--fingerprint-do-not-track': 'doNotTrack',
  '--fingerprint-media-devices': 'mediaDevices',
  '--fingerprint-touch-points': 'touchPoints',
}

// FingerprintConfig → string[]
export function serialize(config: FingerprintConfig): string[] {
  const args: string[] = []
  if (config.seed) args.push(`--fingerprint=${config.seed}`)
  if (config.brand) args.push(`--fingerprint-brand=${config.brand}`)
	if (config.platform) args.push(`--fingerprint-platform=${config.platform === 'mac' ? 'macos' : config.platform}`)
	if (config.lang) {
		args.push(`--lang=${config.lang}`)
		const primary = config.lang.split('-')[0]
		args.push(`--accept-lang=${config.acceptLang || `${config.lang},${primary}`}`)
	}
  if (config.timezone) {
    // 如果是 system，替换为实际系统时区
    const tz = config.timezone === 'system' ? getSystemTimezone() : config.timezone
    args.push(`--timezone=${tz}`)
  }

  const res = config.resolution === 'custom' ? config.customResolution : config.resolution
  if (res) args.push(`--window-size=${res}`)

  if (config.colorDepth) args.push(`--fingerprint-color-depth=${config.colorDepth}`)
  if (config.hardwareConcurrency) args.push(`--fingerprint-hardware-concurrency=${config.hardwareConcurrency}`)
  if (config.deviceMemory) args.push(`--fingerprint-device-memory=${config.deviceMemory}`)

  if (config.canvasNoise !== undefined) args.push(`--fingerprint-canvas-noise=${config.canvasNoise}`)
  if (config.webglVendor) args.push(`--fingerprint-webgl-vendor=${config.webglVendor}`)
  if (config.webglRenderer) args.push(`--fingerprint-webgl-renderer=${config.webglRenderer}`)
  if (config.audioNoise !== undefined) args.push(`--fingerprint-audio-noise=${config.audioNoise}`)

  if (config.fonts) args.push(`--fingerprint-fonts=${config.fonts}`)

  if (config.webrtcPolicy) args.push(`--webrtc-ip-handling-policy=${config.webrtcPolicy}`)
  if (config.doNotTrack !== undefined) args.push(`--fingerprint-do-not-track=${config.doNotTrack}`)
  if (config.mediaDevices) args.push(`--fingerprint-media-devices=${config.mediaDevices}`)
  if (config.touchPoints) args.push(`--fingerprint-touch-points=${config.touchPoints}`)

	return dedupeArgs([...args, ...(config.unknownArgs ?? [])])
}

export function dedupeArgs(args: string[]): string[] {
	const lastIndex = new Map<string, number>()
	args.forEach((arg, index) => lastIndex.set(arg.split('=', 1)[0], index))
	return args.filter((arg, index) => lastIndex.get(arg.split('=', 1)[0]) === index)
}

export function applyFingerprintCapabilities(args: string[]): string[] {
	return dedupeArgs(args.map(arg => {
		if (arg === '--fingerprint-platform=mac') return '--fingerprint-platform=macos'
		return arg
	}))
}

export function withoutDisabledSpoofingFeature(args: string[] | undefined, feature: string): string[] {
	return setDisabledSpoofingFeature(args, feature, false)
}

export function hasDisabledSpoofingFeature(args: string[] | undefined, feature: string): boolean {
	const normalizedFeature = feature.trim().toLowerCase()
	return (args ?? []).some(arg => arg.startsWith('--disable-spoofing=') && arg
		.slice('--disable-spoofing='.length)
		.split(',')
		.some(item => item.trim().toLowerCase() === normalizedFeature))
}

export function setDisabledSpoofingFeature(args: string[] | undefined, feature: string, disabled: boolean): string[] {
	const normalizedFeature = feature.trim().toLowerCase()
	const otherArgs: string[] = []
	const tokens: string[] = []
	const seen = new Set<string>()

	for (const arg of args ?? []) {
		if (!arg.startsWith('--disable-spoofing=')) {
			otherArgs.push(arg)
			continue
		}
		for (const rawToken of arg.slice('--disable-spoofing='.length).split(',')) {
			const token = rawToken.trim()
			const normalizedToken = token.toLowerCase()
			if (!token || normalizedToken === normalizedFeature || seen.has(normalizedToken)) continue
			seen.add(normalizedToken)
			tokens.push(token)
		}
	}

	if (disabled && normalizedFeature && !seen.has(normalizedFeature)) tokens.push(feature.trim())
	return tokens.length > 0 ? [...otherArgs, `--disable-spoofing=${tokens.join(',')}`] : otherArgs
}

const LEGACY_GPU_ARG_KEYS = new Set([
	'--fingerprint-gpu-vendor',
	'--fingerprint-gpu-renderer',
	'--fingerprint-webgl-vendor',
	'--fingerprint-webgl-renderer',
	'--disable-gpu-fingerprint',
])

export function withoutLegacyGpuArgs(args: string[] | undefined): string[] {
	const filtered = (args ?? []).filter(arg => !LEGACY_GPU_ARG_KEYS.has(arg.split('=', 1)[0].toLowerCase()))
	return withoutDisabledSpoofingFeature(filtered, 'gpu')
}

// string[] → FingerprintConfig
export function deserialize(args: string[]): FingerprintConfig {
  const config: FingerprintConfig = { unknownArgs: [] }

  for (const arg of args) {
    const eqIdx = arg.indexOf('=')
    if (eqIdx === -1) {
      config.unknownArgs!.push(arg)
      continue
    }
    const key = arg.slice(0, eqIdx)
    const val = arg.slice(eqIdx + 1)
    const field = KEY_MAP[key]

    if (!field) {
      config.unknownArgs!.push(arg)
      continue
    }

    if (field === 'canvasNoise' || field === 'audioNoise' || field === 'doNotTrack') {
      (config as Record<string, unknown>)[field] = val === 'true'
    } else if (field === 'resolution') {
      if (PRESET_RESOLUTIONS.includes(val)) {
        config.resolution = val
      } else {
        config.resolution = 'custom'
        config.customResolution = val
      }
    } else {
      (config as Record<string, unknown>)[field] = val
    }
  }

  return config
}

// 生成随机指纹种子（32位正整数）
export function randomFingerprintSeed(): string {
  return String(Math.floor(Math.random() * 2147483647) + 1)
}

// ─── 预设指纹配置 ────────────────────────────────────────────────────────────

export interface FingerprintPreset {
  id: string
  name: string
  description: string
  config: Partial<FingerprintConfig>
}

const COMMON_WINDOWS_FONTS = 'Arial,Segoe UI,Microsoft YaHei,Calibri,Times New Roman'
const COMMON_MAC_FONTS = 'Arial,Helvetica,PingFang SC,Hiragino Sans,Times New Roman'
const COMMON_LINUX_FONTS = 'Arial,Noto Sans,DejaVu Sans,Liberation Sans,Times New Roman'

const BUILTIN_FINGERPRINT_PRESETS: FingerprintPreset[] = [
	{
		id: 'win-chrome-cn-office', name: 'Windows / 中文办公', description: '中文办公环境，标准全高清窗口与常见硬件配置',
		config: { brand: 'Chrome', platform: 'windows', lang: 'zh-CN', timezone: 'Asia/Shanghai', resolution: '1920,1080', colorDepth: '24', hardwareConcurrency: '8', deviceMemory: '8', canvasNoise: true, audioNoise: true, fonts: COMMON_WINDOWS_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: false, touchPoints: '0' },
	},
	{
		id: 'win-chrome-high-performance', name: 'Windows / 高性能', description: '高性能桌面环境，2K 窗口与较高 CPU、内存参数',
		config: { brand: 'Chrome', platform: 'windows', lang: 'en-US', timezone: 'America/Los_Angeles', resolution: '2560,1440', colorDepth: '24', hardwareConcurrency: '16', deviceMemory: '16', canvasNoise: true, audioNoise: true, fonts: COMMON_WINDOWS_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: false, touchPoints: '0' },
	},
	{
		id: 'win-edge-enterprise', name: 'Windows / Edge 企业', description: '企业办公环境，Edge 品牌与标准桌面参数',
		config: { brand: 'Edge', platform: 'windows', lang: 'zh-CN', timezone: 'Asia/Shanghai', resolution: '1366,768', colorDepth: '24', hardwareConcurrency: '4', deviceMemory: '4', canvasNoise: true, audioNoise: false, fonts: COMMON_WINDOWS_FONTS, webrtcPolicy: 'default_public_interface_only', doNotTrack: false, touchPoints: '0' },
	},
	{
		id: 'win-chrome-us-daily', name: 'Windows / 美国日常', description: '美国日常使用环境，美西时区与常见桌面参数',
		config: { brand: 'Chrome', platform: 'windows', lang: 'en-US', timezone: 'America/Los_Angeles', resolution: '1920,1080', colorDepth: '24', hardwareConcurrency: '8', deviceMemory: '8', canvasNoise: true, audioNoise: true, fonts: COMMON_WINDOWS_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: false, touchPoints: '0' },
	},
	{
		id: 'win-chrome-uk-office', name: 'Windows / 英国办公', description: '英国办公环境，英语与伦敦时区',
		config: { brand: 'Chrome', platform: 'windows', lang: 'en-GB', timezone: 'Europe/London', resolution: '1920,1080', colorDepth: '24', hardwareConcurrency: '8', deviceMemory: '8', canvasNoise: true, audioNoise: true, fonts: COMMON_WINDOWS_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: false, touchPoints: '0' },
	},
	{
		id: 'win-chrome-jp-office', name: 'Windows / 日本办公', description: '日本办公环境，日语与东京时区',
		config: { brand: 'Chrome', platform: 'windows', lang: 'ja-JP', timezone: 'Asia/Tokyo', resolution: '1920,1080', colorDepth: '24', hardwareConcurrency: '8', deviceMemory: '8', canvasNoise: true, audioNoise: true, fonts: 'Arial,Segoe UI,Yu Gothic,Meiryo,Times New Roman', webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: false, touchPoints: '0' },
	},
	{
		id: 'win-chrome-in-office', name: 'Windows / 印度办公', description: '印度办公环境，英语与加尔各答时区',
		config: { brand: 'Chrome', platform: 'windows', lang: 'en-IN', timezone: 'Asia/Kolkata', resolution: '1920,1080', colorDepth: '24', hardwareConcurrency: '8', deviceMemory: '8', canvasNoise: true, audioNoise: true, fonts: COMMON_WINDOWS_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: false, touchPoints: '0' },
	},
	{
		id: 'mac-chrome-cn-creative', name: 'macOS / 中文创意工作', description: '中文创意工作环境，Retina 比例窗口与较高色深',
		config: { brand: 'Chrome', platform: 'macos', lang: 'zh-CN', timezone: 'Asia/Shanghai', resolution: '2560,1440', colorDepth: '30', hardwareConcurrency: '10', deviceMemory: '16', canvasNoise: true, audioNoise: true, fonts: COMMON_MAC_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: true, touchPoints: '0' },
	},
	{
		id: 'mac-chrome-jp-daily', name: 'macOS / 日本日常', description: '日本日常使用环境，日语与东京时区',
		config: { brand: 'Chrome', platform: 'macos', lang: 'ja-JP', timezone: 'Asia/Tokyo', resolution: '1440,900', colorDepth: '24', hardwareConcurrency: '8', deviceMemory: '8', canvasNoise: true, audioNoise: true, fonts: 'Arial,Helvetica,Hiragino Sans,Yu Gothic,Times New Roman', webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: true, touchPoints: '0' },
	},
	{
		id: 'mac-chrome-us-education', name: 'macOS / 美国教育', description: '美国教育场景，英语与美东时区',
		config: { brand: 'Chrome', platform: 'macos', lang: 'en-US', timezone: 'America/New_York', resolution: '1440,900', colorDepth: '24', hardwareConcurrency: '8', deviceMemory: '8', canvasNoise: true, audioNoise: true, fonts: COMMON_MAC_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: false, touchPoints: '0' },
	},
	{
		id: 'mac-chrome-uk-office', name: 'macOS / 英国办公', description: '英国办公环境，英语与伦敦时区',
		config: { brand: 'Chrome', platform: 'macos', lang: 'en-GB', timezone: 'Europe/London', resolution: '1440,900', colorDepth: '24', hardwareConcurrency: '8', deviceMemory: '8', canvasNoise: true, audioNoise: true, fonts: COMMON_MAC_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: false, touchPoints: '0' },
	},
	{
		id: 'linux-chrome-cn-development', name: 'Linux / 中文开发', description: '中文开发环境，常见桌面分辨率与开发字体',
		config: { brand: 'Chrome', platform: 'linux', lang: 'zh-CN', timezone: 'Asia/Shanghai', resolution: '1920,1080', colorDepth: '24', hardwareConcurrency: '8', deviceMemory: '8', canvasNoise: true, audioNoise: true, fonts: COMMON_LINUX_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: true, touchPoints: '0' },
	},
	{
		id: 'linux-chrome-us-development', name: 'Linux / 美国开发', description: '美国开发环境，英语与美西时区',
		config: { brand: 'Chrome', platform: 'linux', lang: 'en-US', timezone: 'America/Los_Angeles', resolution: '1920,1080', colorDepth: '24', hardwareConcurrency: '12', deviceMemory: '16', canvasNoise: true, audioNoise: true, fonts: COMMON_LINUX_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: true, touchPoints: '0' },
	},
	{
		id: 'linux-chrome-de-workstation', name: 'Linux / 德国工作站', description: '德国工作站环境，德语与柏林时区',
		config: { brand: 'Chrome', platform: 'linux', lang: 'de-DE', timezone: 'Europe/Berlin', resolution: '2560,1440', colorDepth: '24', hardwareConcurrency: '16', deviceMemory: '16', canvasNoise: true, audioNoise: true, fonts: COMMON_LINUX_FONTS, webrtcPolicy: 'disable_non_proxied_udp', doNotTrack: true, touchPoints: '0' },
	},
]

export function isGpuNeutralPreset(preset: FingerprintPreset): boolean {
	const { config } = preset
	if (config.webglVendor || config.webglRenderer) return false
	if (hasDisabledSpoofingFeature(config.unknownArgs, 'gpu')) return false
	return !(config.unknownArgs ?? []).some(arg => LEGACY_GPU_ARG_KEYS.has(arg.split('=', 1)[0].toLowerCase()))
}

function isCompletePreset(preset: FingerprintPreset): boolean {
	return Boolean(preset.id && preset.name && preset.config.platform && preset.config.lang && preset.config.timezone)
}

const seenPresetIds = new Set<string>()
export const FINGERPRINT_PRESETS: FingerprintPreset[] = BUILTIN_FINGERPRINT_PRESETS.filter(preset => {
	if (!isCompletePreset(preset) || !isGpuNeutralPreset(preset) || seenPresetIds.has(preset.id)) return false
	seenPresetIds.add(preset.id)
	return true
})

export function applyLocaleToFingerprintArgs(args: string[], lang: string, timezone: string): string[] {
  const nextConfig = deserialize(args || [])
  if (lang) nextConfig.lang = lang
  if (timezone) nextConfig.timezone = timezone
  return serialize(nextConfig)
}
