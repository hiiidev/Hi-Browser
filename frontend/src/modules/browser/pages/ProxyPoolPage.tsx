import { useCallback, useEffect, useMemo, useState } from 'react'
import { ConfirmModal, toast } from '../../../shared/components'
import type { SortOrder } from '../../../shared/components/Table'
import { uiConfig } from '../../../config/ui.config'
import type { BrowserProxy, ProxyIPHealthResult } from '../types'
import { fetchBrowserProxies, fetchBrowserProxyGroups, saveBrowserProxies } from '../api'
import {
  buildChainImportCandidate,
  createInitialChainImportForm,
  ensureBuiltinProxies,
  toChainImportForm,
  toDisplayList,
  type ChainImportForm,
  type ProxyDisplayInfo,
} from './proxyPool/helpers'
import {
  ProxyPoolEditModal,
  ProxyPoolIPHealthDetailModal,
  ProxyPoolImportModal,
  ProxyPoolPreviewModal,
  type ProxyEditFormValue,
} from './proxyPool/ProxyPoolModals'
import { ProxyPoolHeader } from './proxyPool/ProxyPoolHeader'
import { ProxyPoolTableCard } from './proxyPool/ProxyPoolTableCard'
import { ProxyPoolCheckSettingsModal } from './proxyPool/ProxyPoolCheckSettingsModal'
import { ProxyCoreDownloadModal } from './proxyPool/ProxyCoreDownloadModal'
import { useProxySourceRefresh } from './proxyPool/useProxySourceRefresh'
import { useProxyImportFlow } from './proxyPool/useProxyImportFlow'
import { useProxyChecks } from './proxyPool/useProxyChecks'
import { useProxySelection } from './proxyPool/useProxySelection'
import { useProxyCheckSettingsModal } from './proxyPool/useProxyCheckSettingsModal'
import { useProxyGlobalRefreshConfig } from './proxyPool/useProxyGlobalRefreshConfig'
import { useProxyDeleteFlow } from './proxyPool/useProxyDeleteFlow'
import { useProxyCoreDownload } from './proxyPool/useProxyCoreDownload'
import { useProxyPoolFilter } from './proxyPool/useProxyPoolFilter'
import { getFailedProxyIds } from './proxyPool/failure'

export function ProxyPoolPage() {
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [displayList, setDisplayList] = useState<ProxyDisplayInfo[]>([])
  const [loading, setLoading] = useState(true)
  const {
    coreDownloadOpen,
    coreDownloadType,
    setCoreDownloadType,
    coreDownloadGOOS,
    setCoreDownloadGOOS,
    coreDownloadGOARCH,
    setCoreDownloadGOARCH,
    coreDownloadProxy,
    setCoreDownloadProxy,
    coreDownloadProgress,
    currentCoreStatus,
    downloadCoreStatus,
    downloadCoreStatusLoading,
    loadBrowserSettings,
    handleStartCoreDownload,
    openCoreDownload,
    closeCoreDownload,
  } = useProxyCoreDownload()
  const [groups, setGroups] = useState<string[]>([])

  const [filterProtocol, setFilterProtocol] = useState<string>('all')
  const [filterKeyword, setFilterKeyword] = useState('')
  const [filterGroup, setFilterGroup] = useState<string>('all')
  const [filterAvailableOnly, setFilterAvailableOnly] = useState(false)
  const [sortColumn, setSortColumn] = useState<string>('') // 默认不排序
  const [sortOrder, setSortOrder] = useState<SortOrder>(undefined)
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(uiConfig.pagination.defaultPageSize)
  const [deleteFailedConfirmOpen, setDeleteFailedConfirmOpen] = useState(false)
  const [deletingFailed, setDeletingFailed] = useState(false)

  const {
    checkSettingsOpen,
    setCheckSettingsOpen,
    checkSettings,
    setCheckSettings,
    checkTargetsText,
    setCheckTargetsText,
    savingCheckSettings,
    openCheckSettings,
    saveCheckSettings,
  } = useProxyCheckSettingsModal()

  const {
    globalAutoRefreshEnabled,
    setGlobalAutoRefreshEnabled,
    globalRefreshInterval,
    globalRefreshIntervalM,
    setGlobalRefreshIntervalM,
  } = useProxyGlobalRefreshConfig()

  const [editModalOpen, setEditModalOpen] = useState(false)
  const [editingProxy, setEditingProxy] = useState<BrowserProxy | null>(null)
  const [chainEditMode, setChainEditMode] = useState(false)
  const [chainEditForm, setChainEditForm] = useState<ChainImportForm>(() => createInitialChainImportForm())
  const [editForm, setEditForm] = useState<ProxyEditFormValue>({
    proxyName: '',
    proxyConfig: '',
    preferredKernel: 'auto',
    dnsServers: '',
    groupName: '',
  })
  const [saving, setSaving] = useState(false)
  const saveProxies = useCallback(async (list: BrowserProxy[]) => {
    await saveBrowserProxies(list)
    setProxies(list)
    setDisplayList(toDisplayList(list))
    const grps = await fetchBrowserProxyGroups()
    setGroups(grps)
  }, [])

  const {
    importModalOpen, setImportModalOpen, importMode, importUrl, importFetchProxyId, importResolvedUrl, importText,
    importDnsServers, importNamePrefix, importGroupName, chainImportText, directImportText,
    chainImportForm, directImportForm, previewModalOpen, setPreviewModalOpen, previewList, removedPreviewProxyNames,
    importing, fetchingImportUrl, canParseImport, setImportText, setImportDnsServers,
    setImportNamePrefix, setImportGroupName, setImportFetchProxyId, setChainImportText, setDirectImportText,
    setChainImportForm, setDirectImportForm, handleRemovePreviewProxy, updateChainImportHop,
    handleImportModeChange, handleFillChainTemplate, handleFillDirectTemplate, handleCopyChainTemplate,
    handleCopyDirectTemplate, handleApplyChainJSON, handleApplyDirectText, handleImportUrlChange,
    handleFetchImportURL, handleParseImport, handleConfirmImport,
  } = useProxyImportFlow({
    proxies,
    globalAutoRefreshEnabled,
    globalRefreshInterval,
    saveProxies,
  })

  const {
    hasURLImportSources,
    refreshingAllSources,
    refreshingSourceIds,
    refreshSingleSource,
    handleRefreshAllSources,
  } = useProxySourceRefresh({
    proxies,
    globalAutoRefreshEnabled,
    globalRefreshInterval,
    saveProxies,
  })

  const {
    latencyMap,
    latencyEngineMap,
    latencyErrorMap,
    sessionTestedIds,
    testingAll,
    ipHealthMap,
    checkingIPHealthIds,
    checkingAllIPHealth,
    warmingBridgeIds,
    warmingAllBridges,
    ipHealthDetailOpen,
    setIPHealthDetailOpen,
    currentIPHealthDetail,
    setLatencyMap,
    setLatencyEngineMap,
    setIPHealthMap,
    handleTestOne,
    handleTestAll,
    handleWarmupOne,
    handleWarmupAll,
    handleCheckOneIPHealth,
    handleCheckAllIPHealth,
    openIPHealthDetail,
  } = useProxyChecks({ proxies })

  const loadProxies = useCallback(async () => {
    setLoading(true)
    try {
      const [list, groupList] = await Promise.all([
        fetchBrowserProxies(),
        fetchBrowserProxyGroups(),
      ])
      const finalList = await ensureBuiltinProxies(list)
      setProxies(finalList)
      setDisplayList(toDisplayList(finalList))
      setGroups(groupList)

      setLatencyMap(prev => {
        const validIds = new Set(finalList.map(p => p.proxyId))
        const next: Record<string, number> = {}
        Object.entries(prev).forEach(([proxyId, latency]) => {
          if (validIds.has(proxyId)) next[proxyId] = latency
        })
        return next
      })

      setLatencyEngineMap(prev => {
        const validIds = new Set(finalList.map(p => p.proxyId))
        const next: Record<string, string> = {}
        Object.entries(prev).forEach(([proxyId, engine]) => {
          if (validIds.has(proxyId)) next[proxyId] = engine
        })
        return next
      })

      setIPHealthMap(prev => {
        const validIds = new Set(finalList.map(p => p.proxyId))
        const next: Record<string, ProxyIPHealthResult> = {}
        Object.entries(prev).forEach(([proxyId, health]) => {
          if (validIds.has(proxyId)) next[proxyId] = health
        })
        return next
      })
    } catch (error: any) {
      toast.error(error?.message || '加载代理失败')
    } finally {
      setLoading(false)
    }
  }, [setIPHealthMap, setLatencyEngineMap, setLatencyMap])

  useEffect(() => {
    void loadProxies()
    void loadBrowserSettings()
  }, [loadProxies, loadBrowserSettings])

  const { protocolOptions, filteredList } = useProxyPoolFilter({
    displayList,
    filterProtocol,
    filterKeyword,
    filterGroup,
    filterAvailableOnly,
    sortColumn,
    sortOrder,
    latencyMap,
    ipHealthMap,
  })

  useEffect(() => {
    setCurrentPage(1)
  }, [filterProtocol, filterKeyword, filterGroup, filterAvailableOnly, sortColumn, sortOrder, pageSize])

  const pageCount = Math.max(1, Math.ceil(filteredList.length / pageSize))

  useEffect(() => {
    setCurrentPage(prev => Math.min(prev, pageCount))
  }, [pageCount])

  const currentPageList = useMemo(() => {
    const offset = (currentPage - 1) * pageSize
    return filteredList.slice(offset, offset + pageSize)
  }, [currentPage, filteredList, pageSize])

  const failedProxyIds = useMemo(
    () => getFailedProxyIds(filteredList, proxies, latencyMap, sessionTestedIds),
    [filteredList, latencyMap, proxies, sessionTestedIds],
  )
  const isTesting = testingAll || Object.values(latencyMap).some(latency => latency === -1)

  const {
    selectedIds,
    selectedCount,
    allCurrentPageSelected,
    someCurrentPageSelected,
    batchDeleteConfirmOpen,
    setBatchDeleteConfirmOpen,
    handleToggleAll,
    handleToggleOne,
    handleBatchDeleteConfirm,
    removeSelectedId,
    removeSelectedIds,
  } = useProxySelection({ proxies, currentPageList, saveProxies })

  const handleDeleteFailedConfirm = async () => {
    if (failedProxyIds.length === 0 || isTesting) return
    const failedIdSet = new Set(failedProxyIds)
    setDeletingFailed(true)
    try {
      await saveProxies(proxies.filter(proxy => !failedIdSet.has(proxy.proxyId)))
      removeSelectedIds(failedIdSet)
      setDeleteFailedConfirmOpen(false)
      toast.success(`已删除 ${failedIdSet.size} 个测试失败代理`)
    } catch (error: any) {
      toast.error(error?.message || '删除测试失败代理失败')
    } finally {
      setDeletingFailed(false)
    }
  }

  const updateChainEditHop = (hop: 'first' | 'second', field: keyof ChainImportForm['first'], value: string) => {
    setChainEditForm(prev => ({
      ...prev,
      [hop]: {
        ...prev[hop],
        [field]: value,
      },
    }))
  }

  const handleEdit = (record: ProxyDisplayInfo) => {
    const proxy = proxies.find(p => p.proxyId === record.proxyId)
    if (proxy) {
      setEditingProxy(proxy)
      setEditForm({
        proxyName: proxy.proxyName,
        proxyConfig: proxy.proxyConfig,
        preferredKernel: proxy.preferredKernel || 'auto',
        dnsServers: proxy.dnsServers || '',
        groupName: proxy.groupName || '',
      })
      const nextChainForm = toChainImportForm(proxy.proxyName, proxy.proxyConfig)
      if (nextChainForm) {
        setChainEditMode(true)
        setChainEditForm(nextChainForm)
      } else {
        setChainEditMode(false)
        setChainEditForm(createInitialChainImportForm())
      }
      setEditModalOpen(true)
    }
  }

  const handleSaveProxy = async () => {
    if (!editingProxy) return

    let nextProxyName = editForm.proxyName.trim()
    let nextProxyConfig = editForm.proxyConfig
    if (chainEditMode) {
      try {
        const candidate = buildChainImportCandidate(chainEditForm)
        nextProxyName = candidate.proxyName
        nextProxyConfig = candidate.proxyConfig
      } catch (error: any) {
        toast.error(error?.message || '链式代理配置无效')
        return
      }
    } else if (!nextProxyName) {
      toast.error('请输入代理名称')
      return
    }

    setSaving(true)
    try {
      const newProxies = proxies.map(p =>
        p.proxyId === editingProxy.proxyId
          ? {
            ...p,
            proxyName: nextProxyName,
            proxyConfig: nextProxyConfig,
            preferredKernel: editForm.preferredKernel === 'auto' ? undefined : editForm.preferredKernel,
            dnsServers: editForm.dnsServers.trim() || undefined,
            groupName: editForm.groupName.trim() || undefined,
          }
          : p
      )
      await saveProxies(newProxies)
      setEditModalOpen(false)
      toast.success('代理已更新')
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }
  const {
    deleteConfirmOpen,
    setDeleteConfirmOpen,
    handleDeleteClick,
    handleDeleteConfirm,
  } = useProxyDeleteFlow({ proxies, saveProxies, removeSelectedId })
  return (
    <div className="space-y-5 animate-fade-in">
      <ProxyPoolHeader
        checkingAllIPHealth={checkingAllIPHealth}
        currentConnectorStatus={currentCoreStatus?.message || '未知'}
        hasURLImportSources={hasURLImportSources}
        onCheckAllIPHealth={() => void handleCheckAllIPHealth(filteredList)}
        onOpenSettings={() => void openCheckSettings()}
        onOpenImport={() => setImportModalOpen(true)}
        onOpenCoreDownload={openCoreDownload}
        onRefreshAllSources={() => void handleRefreshAllSources(false)}
        onTestAll={() => void handleTestAll(filteredList)}
        refreshingAllSources={refreshingAllSources}
        testingAll={testingAll}
        totalCount={filteredList.length}
      />

      <ProxyPoolTableCard
        allCurrentPageSelected={allCurrentPageSelected}
        checkingIPHealthIds={checkingIPHealthIds}
        data={currentPageList}
        failedCount={failedProxyIds.length}
        filterGroup={filterGroup}
        filterKeyword={filterKeyword}
        filterProtocol={filterProtocol}
        filterAvailableOnly={filterAvailableOnly}
        globalAutoRefreshEnabled={globalAutoRefreshEnabled}
        globalRefreshInterval={globalRefreshInterval}
        globalRefreshIntervalM={globalRefreshIntervalM}
        groups={groups}
        ipHealthMap={ipHealthMap}
        latencyMap={latencyMap}
        latencyEngineMap={latencyEngineMap}
        latencyErrorMap={latencyErrorMap}
        loading={loading}
        isTesting={isTesting}
        onCheckOneIPHealth={(record) => void handleCheckOneIPHealth(record)}
        onClearFilters={() => {
          setFilterProtocol('all')
          setFilterKeyword('')
          setFilterGroup('all')
          setFilterAvailableOnly(false)
        }}
        onDelete={handleDeleteClick}
        onEdit={handleEdit}
        onFilterGroupChange={setFilterGroup}
        onFilterKeywordChange={setFilterKeyword}
        onFilterProtocolChange={setFilterProtocol}
        onFilterAvailableOnlyChange={setFilterAvailableOnly}
        onGlobalAutoRefreshEnabledChange={setGlobalAutoRefreshEnabled}
        onGlobalRefreshIntervalMChange={setGlobalRefreshIntervalM}
        onOpenBatchDelete={() => setBatchDeleteConfirmOpen(true)}
        onOpenDeleteFailed={() => setDeleteFailedConfirmOpen(true)}
        onOpenIPHealthDetail={openIPHealthDetail}
        onRefreshSingleSource={(sourceId) => void refreshSingleSource(sourceId, false)}
        onSort={({ column, order }) => {
          setSortColumn(column)
          setSortOrder(order)
        }}
        onTestOne={(record) => void handleTestOne(record)}
        onToggleAll={handleToggleAll}
        onToggleOne={handleToggleOne}
        onWarmupOne={(record) => void handleWarmupOne(record)}
        onWarmupSelected={() => void handleWarmupAll(filteredList.filter(item => selectedIds.has(item.proxyId)))}
        protocolOptions={protocolOptions}
        refreshingSourceIds={refreshingSourceIds}
        selectedCount={selectedCount}
        selectedIds={selectedIds}
        someCurrentPageSelected={someCurrentPageSelected}
        sortColumn={sortColumn}
        sortOrder={sortOrder}
        warmingAllBridges={warmingAllBridges}
        warmingBridgeIds={warmingBridgeIds}
        currentPage={currentPage}
        end={filteredList.length === 0 ? 0 : Math.min(currentPage * pageSize, filteredList.length)}
        onPageChange={setCurrentPage}
        onPageSizeChange={setPageSize}
        pageCount={pageCount}
        pageSize={pageSize}
        pageSizeOptions={uiConfig.pagination.pageSizeOptions}
        start={filteredList.length === 0 ? 0 : (currentPage - 1) * pageSize + 1}
        total={filteredList.length}
      />

      <ProxyPoolImportModal
        open={importModalOpen}
        groups={groups}
        importMode={importMode}
        importUrl={importUrl}
        importFetchProxyId={importFetchProxyId}
        importResolvedUrl={importResolvedUrl}
        importText={importText}
        importDnsServers={importDnsServers}
        importNamePrefix={importNamePrefix}
        importGroupName={importGroupName}
        chainImportText={chainImportText}
        directImportText={directImportText}
        chainImportForm={chainImportForm}
        directImportForm={directImportForm}
        fetchingImportUrl={fetchingImportUrl}
        fetchProxyOptions={proxies.filter(proxy => proxy.proxyConfig.trim() && !proxy.proxyConfig.trim().toLowerCase().startsWith('direct://'))}
        canParseImport={canParseImport}
        onClose={() => setImportModalOpen(false)}
        onParse={handleParseImport}
        onFetchImportUrl={handleFetchImportURL}
        onImportModeChange={handleImportModeChange}
        onImportUrlChange={handleImportUrlChange}
        onImportFetchProxyIdChange={setImportFetchProxyId}
        onImportTextChange={setImportText}
        onImportDnsServersChange={setImportDnsServers}
        onImportNamePrefixChange={setImportNamePrefix}
        onImportGroupNameChange={setImportGroupName}
        onChainImportTextChange={setChainImportText}
        onDirectImportTextChange={setDirectImportText}
        onApplyChainJSON={handleApplyChainJSON}
        onApplyDirectText={handleApplyDirectText}
        onChainImportFormChange={(patch) => setChainImportForm((prev) => ({ ...prev, ...patch }))}
        onChainImportHopChange={updateChainImportHop}
        onFillChainTemplate={handleFillChainTemplate}
        onCopyChainTemplate={() => void handleCopyChainTemplate()}
        onFillDirectTemplate={handleFillDirectTemplate}
        onCopyDirectTemplate={() => void handleCopyDirectTemplate()}
        onDirectImportFormChange={(patch) => setDirectImportForm((prev) => ({ ...prev, ...patch }))}
      />

      <ProxyPoolPreviewModal
        open={previewModalOpen}
        importMode={importMode}
        importDnsServers={importDnsServers}
        previewList={previewList}
        removedPreviewProxyNames={removedPreviewProxyNames}
        importing={importing}
        onClose={() => setPreviewModalOpen(false)}
        onBack={() => {
          setPreviewModalOpen(false)
          setImportModalOpen(true)
        }}
        onConfirm={handleConfirmImport}
        onRemoveProxy={handleRemovePreviewProxy}
      />

      <ProxyPoolEditModal
        open={editModalOpen}
        saving={saving}
        groups={groups}
        editForm={editForm}
        chainEditMode={chainEditMode}
        chainEditForm={chainEditForm}
        onClose={() => setEditModalOpen(false)}
        onSave={handleSaveProxy}
        onChange={(patch) => setEditForm((prev) => ({ ...prev, ...patch }))}
        onChainEditFormChange={(patch) => setChainEditForm((prev) => ({ ...prev, ...patch }))}
        onChainEditHopChange={updateChainEditHop}
      />

      <ProxyPoolIPHealthDetailModal
        open={ipHealthDetailOpen}
        detail={currentIPHealthDetail}
        onClose={() => setIPHealthDetailOpen(false)}
      />

      <ProxyPoolCheckSettingsModal
        open={checkSettingsOpen}
        checkSettings={checkSettings}
        checkTargetsText={checkTargetsText}
        saving={savingCheckSettings}
        onClose={() => setCheckSettingsOpen(false)}
        onSave={saveCheckSettings}
        onCheckSettingsChange={setCheckSettings}
        onCheckTargetsTextChange={setCheckTargetsText}
      />

      <ProxyCoreDownloadModal
        open={coreDownloadOpen}
        core={coreDownloadType}
        goos={coreDownloadGOOS}
        goarch={coreDownloadGOARCH}
        downloadProxy={coreDownloadProxy}
        progress={coreDownloadProgress}
        status={downloadCoreStatus}
        statusLoading={downloadCoreStatusLoading}
        onCoreChange={setCoreDownloadType}
        onGOOSChange={setCoreDownloadGOOS}
        onGOARCHChange={setCoreDownloadGOARCH}
        onDownloadProxyChange={setCoreDownloadProxy}
        onClose={closeCoreDownload}
        onStart={handleStartCoreDownload}
      />

      <ConfirmModal open={deleteConfirmOpen} onClose={() => setDeleteConfirmOpen(false)} onConfirm={handleDeleteConfirm}
        title="确认删除" content="确定要删除这个代理吗？此操作不可恢复。" confirmText="删除" danger />

      <ConfirmModal open={batchDeleteConfirmOpen} onClose={() => setBatchDeleteConfirmOpen(false)} onConfirm={handleBatchDeleteConfirm}
        title="批量删除" content={`确定要删除选中的 ${selectedCount} 个代理吗？此操作不可恢复。`} confirmText="删除" danger />

      <ConfirmModal
        open={deleteFailedConfirmOpen}
        onClose={() => { if (!deletingFailed) setDeleteFailedConfirmOpen(false) }}
        onConfirm={() => { void handleDeleteFailedConfirm() }}
        title="删除测试失败代理"
        content={(
          <div className="space-y-2">
            <p>确定删除当前筛选结果中的 {failedProxyIds.length} 个测试失败代理吗？此操作不可恢复。</p>
            <p className="text-sm text-[var(--color-text-muted)]">这些代理不会写入订阅忽略列表，刷新订阅后可能重新出现。</p>
          </div>
        )}
        confirmText="删除失败代理"
        confirmLoading={deletingFailed}
        closeOnConfirm={false}
        danger
      />
    </div>
  )
}
