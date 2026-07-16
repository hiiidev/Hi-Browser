import type { BrowserCore, BrowserCoreDownloadTask, BrowserCoreExtended, BrowserCoreInput, BrowserCoreRelease, BrowserCoreSettings, BrowserCoreValidateResult, FingerprintArgResult, FingerprintCapabilities } from '../types'
import { getBindings, getMockCores, setMockCores } from './runtime'

export async function fetchBrowserCores(): Promise<BrowserCore[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreList) {
    return (await bindings.BrowserCoreList()) || []
  }
  return getMockCores()
}

export async function saveBrowserCore(input: BrowserCoreInput): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreSave) {
    await bindings.BrowserCoreSave(input)
    return true
  }

  const nextCores = [...getMockCores()]
  const index = nextCores.findIndex((core) => core.coreId === input.coreId)
  if (index >= 0) {
    nextCores[index] = input
  } else {
    nextCores.push({ ...input, coreId: input.coreId || `core-${Date.now()}` })
  }
  setMockCores(nextCores)
  return true
}

export async function deleteBrowserCore(coreId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreDelete) {
    await bindings.BrowserCoreDelete(coreId)
    return true
  }
  setMockCores(getMockCores().filter((core) => core.coreId !== coreId))
  return true
}

export async function setDefaultBrowserCore(coreId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreSetDefault) {
    await bindings.BrowserCoreSetDefault(coreId)
    return true
  }
  setMockCores(getMockCores().map((core) => ({ ...core, isDefault: core.coreId === coreId })))
  return true
}

export async function validateBrowserCorePath(corePath: string): Promise<BrowserCoreValidateResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreValidate) {
    return (await bindings.BrowserCoreValidate(corePath)) || { valid: false, message: '验证失败' }
  }
  return { valid: true, message: '路径有效（模拟）' }
}
export async function verifyBrowserCore(coreId:string):Promise<BrowserCoreValidateResult>{const bindings:any=await getBindings();return bindings?.BrowserCoreVerify ? await bindings.BrowserCoreVerify(coreId) : {valid:false,message:'当前后端不支持完整性检查'}}

export async function fetchCoreExtendedInfo(): Promise<BrowserCoreExtended[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreExtendedInfo) {
    return (await bindings.BrowserCoreExtendedInfo()) || []
  }
  return []
}

export async function scanBrowserCores(): Promise<BrowserCore[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreScan) {
    return (await bindings.BrowserCoreScan()) || []
  }
  return getMockCores()
}

export async function importLocalBrowserCore(): Promise<BrowserCore | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreImportLocal) {
    return (await bindings.BrowserCoreImportLocal()) || null
  }
  return null
}

export async function BrowserCoreDownload(coreName: string, url: string, proxyConfig?: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreDownload) {
    await bindings.BrowserCoreDownload(coreName, url, proxyConfig || '')
    return true
  }
  return true
}

export async function fetchBrowserCoreReleases(): Promise<BrowserCoreRelease[]> { const bindings:any=await getBindings(); return bindings?.BrowserCoreAvailableReleases ? ((await bindings.BrowserCoreAvailableReleases()) || []) : [] }
export async function installBrowserCoreRelease(releaseTag:string,proxyConfig=''):Promise<string>{const bindings:any=await getBindings();if(!bindings?.BrowserCoreInstallRelease)throw new Error('当前后端不支持自动安装');return await bindings.BrowserCoreInstallRelease(releaseTag,proxyConfig)}
export async function cancelBrowserCoreDownload(taskId:string):Promise<void>{const bindings:any=await getBindings();if(bindings?.BrowserCoreCancelDownload)await bindings.BrowserCoreCancelDownload(taskId)}
export async function retryBrowserCoreDownload(taskId:string):Promise<string>{const bindings:any=await getBindings();if(!bindings?.BrowserCoreRetryDownload)throw new Error('当前后端不支持重试');return await bindings.BrowserCoreRetryDownload(taskId)}
export async function fetchBrowserCoreDownloadTask(taskId:string):Promise<BrowserCoreDownloadTask>{const bindings:any=await getBindings();if(!bindings?.BrowserCoreDownloadTask)throw new Error('当前后端不支持任务查询');return await bindings.BrowserCoreDownloadTask(taskId)}
export async function fetchBrowserCoreSettings():Promise<BrowserCoreSettings>{const bindings:any=await getBindings();return bindings?.GetBrowserCoreSettings ? await bindings.GetBrowserCoreSettings() : {provider:'fingerprint-chromium-static',channel:'stable',manifestUrl:'https://raw.githubusercontent.com/hiiidev/Hi-Browser/browser-core-index/browser-core-manifest.json',autoCheckUpdates:true,autoInstallWhenMissing:true,autoInstallRecommended:false,keepVersions:2,downloadProxyMode:'system',skippedVersion:'',lastUpdateCheckAt:''}}
export async function saveBrowserCoreSettings(settings:BrowserCoreSettings):Promise<void>{const bindings:any=await getBindings();if(bindings?.SaveBrowserCoreSettings)await bindings.SaveBrowserCoreSettings(settings)}

export async function fetchFingerprintCapabilities(coreId: string, targetPlatform: string): Promise<FingerprintCapabilities> {
	const bindings: any = await getBindings()
	if (bindings?.BrowserCoreFingerprintCapabilities) {
		return await bindings.BrowserCoreFingerprintCapabilities(coreId, targetPlatform)
	}
	const hostPlatform = /Mac/i.test(navigator.platform) ? 'macos' : /Win/i.test(navigator.platform) ? 'windows' : 'linux'
	return {
		provider: 'unknown',
		chromiumMajor: 0,
		hostPlatform,
		targetPlatform: targetPlatform || hostPlatform,
		supportedBrands: ['Chrome', 'Edge', 'Opera', 'Vivaldi', 'Firefox', 'Safari'],
		supportedParameters: [],
		deprecatedParameters: [],
		unsupportedParameters: [],
		warnings: [],
		gpuSpoofingMode: 'seed-driven-real-parameter-set',
		manualGpuConfig: false,
		webpageLanguage: true,
		applicationLocaleMode: 'chromium',
		intlLocaleMode: 'chromium',
		ttsVoicesMode: 'host',
		fontsMode: 'host',
	}
}

export async function normalizeFingerprintArgs(coreId: string, targetPlatform: string, args: string[]): Promise<FingerprintArgResult> {
	const bindings: any = await getBindings()
	if (bindings?.BrowserCoreNormalizeFingerprintArgs) {
		const result = await bindings.BrowserCoreNormalizeFingerprintArgs(coreId, targetPlatform, args)
		const normalizedArgs = Array.isArray(result?.args) ? result.args : [...args]
		return {
			args: normalizedArgs,
			warnings: Array.isArray(result?.warnings) ? result.warnings : [],
			adjusted: Array.isArray(result?.adjusted) ? result.adjusted : [],
			entries: Array.isArray(result?.entries) ? result.entries : normalizedArgs.map((arg: string) => ({ arg, source: 'user' })),
		}
	}
	return { args, warnings: [], adjusted: [], entries: args.map(arg => ({ arg, source: 'user' })) }
}

export async function redownloadBrowserCore(coreId: string, url: string, proxyConfig?: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreRedownload) {
    await bindings.BrowserCoreRedownload(coreId, url, proxyConfig || '')
    return true
  }
  return true
}

export async function openCorePath(corePath: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.OpenCorePath) {
    await bindings.OpenCorePath(corePath)
    return true
  }
  return false
}
