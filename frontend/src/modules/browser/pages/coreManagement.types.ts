export interface CoreDisplayInfo {
  coreId: string
  coreName: string
  corePath: string
  isDefault: boolean
  pathValid: boolean
  pathMessage: string
  chromeVersion: string
  instanceCount: number
	provider: string
	platform: string
	architecture: string
	archiveSha256: string
	verificationStatus: string
	archiveSize: number
}

export interface CoreSettingsForm {
  userDataRoot: string
  defaultFingerprintArgs: string
  defaultLaunchArgs: string
  defaultStartUrls: string
  lightStartEnabled: boolean
  restoreLastSession: boolean
  startReadyTimeoutMs: number
  startStableWindowMs: number
}

export interface CoreEditForm {
  coreName: string
  corePath: string
}

export interface CoreDownloadForm {
  coreId?: string
	releaseTag?: string
	assetName?: string
	assetSize?: number
	platform?: string
	architecture?: string
  name: string
  url: string
  proxyMode: string
  proxyId: string
  mode?: 'download' | 'redownload'
}

export interface CoreDownloadProgress {
	taskId?: string
  phase: string
  progress: number
  message: string
	downloadedBytes?: number
	totalBytes?: number
	speedBytesPerSecond?: number
	errorDetail?: string
	canRetry?: boolean
}
