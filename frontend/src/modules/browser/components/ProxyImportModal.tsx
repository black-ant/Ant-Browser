import { useEffect, useMemo, useState } from 'react'
import { Button, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { BrowserProxy } from '../types'
import { fetchClashImportFromURL, saveBrowserProxies } from '../api'
import { DIRECT_QUICK_IMPORT_TEMPLATE, buildDirectImportCandidatesFromText, parseDirectImportText } from '../pages/proxyPool/helpers'
import {
  INITIAL_CHAIN_IMPORT_FORM,
  INITIAL_DIRECT_IMPORT_FORM,
  type ChainHopForm,
  type ChainImportForm,
  type DirectImportForm,
  type ProxyDisplayInfo,
  type ProxyImportModalProps,
  type ProxyImportMode,
} from './ProxyImportModal.types'
import {
  buildChainImportCandidate,
  buildDirectImportCandidate,
  buildImportCandidatesFromClash,
  buildImportPreview,
  createExistingProxyPicker,
  parseClashImportText,
  nextProxyID,
  normalizeRefreshIntervalM,
  resolveImportSourceID,
} from './ProxyImportModal.helpers'
import { ProxyImportModalView } from './ProxyImportModalView'

export function ProxyImportModal({
  open,
  onClose,
  existingProxies,
  groups,
  globalAutoRefreshEnabled = false,
  globalRefreshIntervalM = 60,
  onImported,
}: ProxyImportModalProps) {
  const [importMode, setImportMode] = useState<ProxyImportMode>('clash')
  const [importUrl, setImportUrl] = useState('')
  const [importFetchProxyId, setImportFetchProxyId] = useState('')
  const [importResolvedUrl, setImportResolvedUrl] = useState('')
  const [importText, setImportText] = useState('')
  const [importDnsServers, setImportDnsServers] = useState('')
  const [importNamePrefix, setImportNamePrefix] = useState('')
  const [importGroupName, setImportGroupName] = useState('')
  const [directImportText, setDirectImportText] = useState('')
  const [directImportForm, setDirectImportForm] = useState<DirectImportForm>(() => ({ ...INITIAL_DIRECT_IMPORT_FORM }))
  const [chainImportForm, setChainImportForm] = useState<ChainImportForm>(() => ({ ...INITIAL_CHAIN_IMPORT_FORM }))
  const [previewModalOpen, setPreviewModalOpen] = useState(false)
  const [previewList, setPreviewList] = useState<ProxyDisplayInfo[]>([])
  const [importing, setImporting] = useState(false)
  const [fetchingImportUrl, setFetchingImportUrl] = useState(false)

  useEffect(() => {
    if (open) return
    setPreviewModalOpen(false)
  }, [open])

  const resetImportState = () => {
    setImportMode('clash')
    setImportUrl('')
    setImportFetchProxyId('')
    setImportResolvedUrl('')
    setImportText('')
    setImportDnsServers('')
    setImportNamePrefix('')
    setImportGroupName('')
    setDirectImportText('')
    setDirectImportForm({ ...INITIAL_DIRECT_IMPORT_FORM })
    setChainImportForm({ ...INITIAL_CHAIN_IMPORT_FORM })
    setPreviewList([])
  }

  const handleImportModeChange = (nextMode: ProxyImportMode) => {
    setImportMode(nextMode)
    setImportResolvedUrl('')
    if (nextMode !== 'clash') {
      setImportUrl('')
      setImportFetchProxyId('')
      setImportDnsServers('')
    }
  }

  const updateChainLanding = (field: keyof ChainHopForm, value: string) => {
    setChainImportForm(prev => ({
      ...prev,
      landing: {
        ...prev.landing,
        [field]: value,
      },
    }))
  }

  const handleFetchImportURL = async () => {
    const targetURL = importUrl.trim()
    if (!targetURL) {
      toast.error('请输入订阅 URL')
      return
    }

    setFetchingImportUrl(true)
    try {
      const result = await fetchClashImportFromURL(targetURL, importFetchProxyId)
      const content = (result?.content || '').trim()
      if (!content) {
        throw new Error('订阅内容为空')
      }

      setImportResolvedUrl((result?.url || targetURL).trim())
      setImportText(content)

      if (!importDnsServers.trim() && typeof result?.dnsServers === 'string' && result.dnsServers.trim()) {
        setImportDnsServers(result.dnsServers.trim())
      }
      if (!importGroupName.trim() && typeof result?.suggestedGroup === 'string' && result.suggestedGroup.trim()) {
        setImportGroupName(result.suggestedGroup.trim())
      }

      toast.success(`URL 获取成功，检测到 ${Math.max(0, Number(result?.proxyCount || 0))} 个代理`)
    } catch (error: any) {
      setImportResolvedUrl('')
      toast.error(error?.message || 'URL 获取失败')
    } finally {
      setFetchingImportUrl(false)
    }
  }

  const handleParseImport = () => {
    try {
      const prefix = importNamePrefix.trim()
      let candidates
      let previewGroupName = importGroupName.trim()
      if (importMode === 'clash') {
        candidates = buildImportCandidatesFromClash(parseClashImportText(importText), prefix)
      } else if (importMode === 'direct') {
        if (directImportText.trim()) {
          const parsed = buildDirectImportCandidatesFromText(directImportText)
          candidates = parsed.candidates
          if (!previewGroupName) {
            previewGroupName = parsed.defaultGroupName
          }
        } else {
          candidates = [buildDirectImportCandidate(directImportForm)]
        }
      } else {
        candidates = [buildChainImportCandidate(chainImportForm)]
      }
      if (!candidates.length) {
        toast.error('未解析到可导入代理')
        return
      }
      const preview = buildImportPreview(candidates, previewGroupName)
      setPreviewList(preview)
      setPreviewModalOpen(true)
    } catch (error: any) {
      toast.error(`解析失败: ${error?.message || '未知错误'}`)
    }
  }

  const handleFillDirectTemplate = () => {
    setDirectImportText(DIRECT_QUICK_IMPORT_TEMPLATE)
  }

  const handleCopyDirectTemplate = async () => {
    try {
      if (!navigator?.clipboard?.writeText) {
        throw new Error('当前环境不支持剪贴板')
      }
      await navigator.clipboard.writeText(DIRECT_QUICK_IMPORT_TEMPLATE)
      toast.success('JSON 模板已复制')
    } catch (error: any) {
      toast.error(error?.message || '复制模板失败')
    }
  }

  const handleApplyDirectText = () => {
    try {
      const { form, groupName } = parseDirectImportText(directImportText)
      setDirectImportForm(form)
      if (groupName) {
        setImportGroupName(groupName)
      }
      setDirectImportText('')
      toast.success('文本已应用')
    } catch (error: any) {
      toast.error(error?.message || '文本应用失败')
    }
  }

  const handleConfirmImport = async () => {
    if (previewList.length === 0) {
      toast.error('请至少保留 1 个代理后再导入')
      return
    }
    setImporting(true)
    try {
      const sourceURL = importMode === 'clash' ? importResolvedUrl.trim() : ''
      const isURLImport = !!sourceURL
      const sourceNamePrefix = importMode === 'clash' ? importNamePrefix.trim() : ''
      const sourceID = isURLImport ? resolveImportSourceID(existingProxies, sourceURL, sourceNamePrefix) : ''
      const sourceAutoRefresh = isURLImport ? !!globalAutoRefreshEnabled : false
      const sourceRefreshIntervalM = sourceAutoRefresh
        ? normalizeRefreshIntervalM(Number(globalRefreshIntervalM || 0))
        : 0
      const sourceLastRefreshAt = isURLImport ? new Date().toISOString() : ''
      const oldSourceProxies = isURLImport
        ? existingProxies.filter(item => (item.sourceId || '').trim() === sourceID)
        : []
      const pickExisting = createExistingProxyPicker(oldSourceProxies)

      const newProxies: BrowserProxy[] = previewList.map((p) => {
        const existingProxy = pickExisting(p.proxyName, p.proxyConfig)
        return {
        proxyId: existingProxy?.proxyId || nextProxyID(),
        proxyName: p.proxyName,
        proxyConfig: p.proxyConfig,
        preferredKernel: existingProxy?.preferredKernel || undefined,
        dnsServers: importMode === 'clash' ? importDnsServers.trim() || undefined : undefined,
        groupName: p.groupName.trim() || undefined,
        sourceId: sourceID || undefined,
        sourceUrl: sourceURL || undefined,
        sourceNamePrefix: sourceNamePrefix || undefined,
        sourceAutoRefresh,
        sourceRefreshIntervalM,
        sourceLastRefreshAt: sourceLastRefreshAt || undefined,
        }
      })
      const allProxies = isURLImport
        ? existingProxies.filter(item => (item.sourceId || '').trim() !== sourceID).concat(newProxies)
        : [...existingProxies, ...newProxies]

      await saveBrowserProxies(allProxies)
      await onImported?.(newProxies)
      toast.success(`成功导入 ${newProxies.length} 个代理`)
      setPreviewModalOpen(false)
      resetImportState()
      onClose()
    } catch (error: any) {
      toast.error(error?.message || '导入失败')
    } finally {
      setImporting(false)
    }
  }

  const handleRemovePreviewProxy = (proxyId: string) => {
    setPreviewList(prev => prev.filter(item => item.proxyId !== proxyId))
  }

  const canParseImport = importMode === 'clash'
    ? !!importText.trim()
    : importMode === 'direct'
      ? !!directImportText.trim() || (!!directImportForm.server.trim() && !!directImportForm.port.trim())
      : !!chainImportForm.frontProxyId.trim()
        && !!chainImportForm.landing.server.trim()
        && !!chainImportForm.landing.port.trim()

  const previewColumns = useMemo<TableColumn<ProxyDisplayInfo>[]>(() => [
    { key: 'proxyName', title: '代理名称', width: '200px' },
    { key: 'type', title: '类型', width: '100px' },
    { key: 'server', title: '服务器', width: '200px' },
    { key: 'port', title: '端口', width: '100px', render: (val) => val || '-' },
    {
      key: 'actions',
      title: '操作',
      width: '96px',
      render: (_, record) => (
        <Button
          size="sm"
          variant="danger"
          onClick={() => handleRemovePreviewProxy(record.proxyId)}
        >
          删除
        </Button>
      ),
    },
  ], [])

  return (
    <ProxyImportModalView
      open={open}
      onClose={onClose}
      fetchingImportUrl={fetchingImportUrl}
      canParseImport={canParseImport}
      importMode={importMode}
      importUrl={importUrl}
      importFetchProxyId={importFetchProxyId}
      importResolvedUrl={importResolvedUrl}
      importText={importText}
      importDnsServers={importDnsServers}
      importNamePrefix={importNamePrefix}
      importGroupName={importGroupName}
      directImportText={directImportText}
      directImportForm={directImportForm}
      chainImportForm={chainImportForm}
      groups={groups}
      fetchProxyOptions={existingProxies.filter(proxy => {
        const config = proxy.proxyConfig.trim().toLowerCase()
        return !!config && !config.startsWith('direct://') && !config.startsWith('chain+proxy://') && !config.startsWith('chain+socks5://')
      })}
      previewModalOpen={previewModalOpen}
      previewList={previewList}
      importing={importing}
      previewColumns={previewColumns}
      onParseImport={handleParseImport}
      onImportModeChange={handleImportModeChange}
      onImportUrlChange={setImportUrl}
      onImportFetchProxyIdChange={setImportFetchProxyId}
      onImportResolvedUrlChange={setImportResolvedUrl}
      onFetchImportURL={handleFetchImportURL}
      onImportTextChange={setImportText}
      onImportDnsServersChange={setImportDnsServers}
      onImportNamePrefixChange={setImportNamePrefix}
      onImportGroupNameChange={setImportGroupName}
      onDirectImportTextChange={setDirectImportText}
      onDirectImportFormChange={setDirectImportForm}
      onChainImportFormChange={setChainImportForm}
      onUpdateChainLanding={updateChainLanding}
      onFillDirectTemplate={handleFillDirectTemplate}
      onCopyDirectTemplate={handleCopyDirectTemplate}
      onApplyDirectText={handleApplyDirectText}
      onPreviewModalOpenChange={setPreviewModalOpen}
      onConfirmImport={handleConfirmImport}
    />
  )
}
