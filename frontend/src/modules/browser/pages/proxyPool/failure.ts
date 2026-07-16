import type { BrowserProxy } from '../../types'

import { BUILTIN_PROXY_IDS, type ProxyDisplayInfo } from './helpers'

const FAILED_LATENCY_VALUES = new Set([-2, -3, -4])

export function isFailedProxy(
  proxy: BrowserProxy,
  latencyMap: Record<string, number>,
  sessionTestedIds: ReadonlySet<string>,
): boolean {
  if (BUILTIN_PROXY_IDS.has(proxy.proxyId)) return false

  if (sessionTestedIds.has(proxy.proxyId)) {
    return FAILED_LATENCY_VALUES.has(latencyMap[proxy.proxyId])
  }

  return Boolean(proxy.lastTestedAt) && proxy.lastTestOk === false
}

export function getFailedProxyIds(
  filteredList: ProxyDisplayInfo[],
  proxies: BrowserProxy[],
  latencyMap: Record<string, number>,
  sessionTestedIds: ReadonlySet<string>,
): string[] {
  const filteredIds = new Set(filteredList.map(proxy => proxy.proxyId))
  return proxies
    .filter(proxy => filteredIds.has(proxy.proxyId) && isFailedProxy(proxy, latencyMap, sessionTestedIds))
    .map(proxy => proxy.proxyId)
}
