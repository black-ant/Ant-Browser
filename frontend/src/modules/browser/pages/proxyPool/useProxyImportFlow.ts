import { useState } from 'react'
import { toast } from '../../../../shared/components'
import type { BrowserProxy } from '../../types'
import { fetchClashImportFromURL } from '../../api'
import {
  CHAIN_QUICK_IMPORT_TEMPLATE,
  DIRECT_QUICK_IMPORT_TEMPLATE,
  INITIAL_CHAIN_IMPORT_FORM,
  INITIAL_DIRECT_IMPORT_FORM,
  buildChainImportCandidate,
  buildDirectImportCandidate,
  buildDirectImportCandidatesFromText,
  buildImportCandidatesFromClash,
  buildImportPreview,
  createExistingProxyPicker,
  nextProxyID,
  parseChainImportJSON,
  parseClashImportText,
  parseDirectImportText,
  resolveImportSourceID,
  type ChainImportForm,
  type DirectImportForm,
  type ProxyDisplayInfo,
  type ProxyImportMode,
} from './helpers'
import { appendSourceIgnoredProxyNames } from './storage'

interface UseProxyImportFlowOptions {
  proxies: BrowserProxy[]
  globalAutoRefreshEnabled: boolean
  globalRefreshInterval: number
  saveProxies: (list: BrowserProxy[]) => Promise<void>
}

function createInitialChainImportForm(): ChainImportForm {
  return {
    ...INITIAL_CHAIN_IMPORT_FORM,
    landing: { ...INITIAL_CHAIN_IMPORT_FORM.landing },
  }
}

export function useProxyImportFlow({
  proxies,
  globalAutoRefreshEnabled,
  globalRefreshInterval,
  saveProxies,
}: UseProxyImportFlowOptions) {
  const [importModalOpen, setImportModalOpen] = useState(false)
  const [importMode, setImportMode] = useState<ProxyImportMode>('clash')
  const [importUrl, setImportUrl] = useState('')
  const [importFetchProxyId, setImportFetchProxyId] = useState('')
  const [importResolvedUrl, setImportResolvedUrl] = useState('')
  const [importText, setImportText] = useState('')
  const [importDnsServers, setImportDnsServers] = useState('')
  const [importNamePrefix, setImportNamePrefix] = useState('')
  const [importGroupName, setImportGroupName] = useState('')
  const [chainImportText, setChainImportText] = useState('')
  const [directImportText, setDirectImportText] = useState('')
  const [chainImportForm, setChainImportForm] = useState<ChainImportForm>(() => createInitialChainImportForm())
  const [directImportForm, setDirectImportForm] = useState<DirectImportForm>(() => ({ ...INITIAL_DIRECT_IMPORT_FORM }))
  const [previewModalOpen, setPreviewModalOpen] = useState(false)
  const [previewList, setPreviewList] = useState<ProxyDisplayInfo[]>([])
  const [removedPreviewProxyNames, setRemovedPreviewProxyNames] = useState<string[]>([])
  const [importing, setImporting] = useState(false)
  const [fetchingImportUrl, setFetchingImportUrl] = useState(false)

  const handleRemovePreviewProxy = (proxyId: string) => {
    const target = previewList.find(item => item.proxyId === proxyId)
    if (target) {
      setRemovedPreviewProxyNames(prev => prev.includes(target.proxyName) ? prev : [...prev, target.proxyName])
    }
    setPreviewList(prev => prev.filter(item => item.proxyId !== proxyId))
  }

  const updateChainImportLanding = (field: keyof ChainImportForm['landing'], value: string) => {
    setChainImportForm(prev => ({
      ...prev,
      landing: {
        ...prev.landing,
        [field]: value,
      },
    }))
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

  const handleFillChainTemplate = () => {
    setChainImportText(CHAIN_QUICK_IMPORT_TEMPLATE)
  }

  const handleFillDirectTemplate = () => {
    setDirectImportText(DIRECT_QUICK_IMPORT_TEMPLATE)
  }

  const handleCopyChainTemplate = async () => {
    try {
      if (!navigator?.clipboard?.writeText) throw new Error('当前环境不支持剪贴板')
      await navigator.clipboard.writeText(CHAIN_QUICK_IMPORT_TEMPLATE)
      toast.success('JSON 模板已复制')
    } catch (error: any) {
      toast.error(error?.message || '复制模板失败')
    }
  }

  const handleCopyDirectTemplate = async () => {
    try {
      if (!navigator?.clipboard?.writeText) throw new Error('当前环境不支持剪贴板')
      await navigator.clipboard.writeText(DIRECT_QUICK_IMPORT_TEMPLATE)
      toast.success('JSON 模板已复制')
    } catch (error: any) {
      toast.error(error?.message || '复制模板失败')
    }
  }

  const handleApplyChainJSON = () => {
    try {
      const { form, groupName } = parseChainImportJSON(chainImportText)
      setChainImportForm(form)
      setImportGroupName(groupName)
      toast.success('JSON 已应用')
    } catch (error: any) {
      toast.error(error?.message || 'JSON 应用失败')
    }
  }

  const handleApplyDirectText = () => {
    try {
      const { form, groupName } = parseDirectImportText(directImportText)
      setDirectImportForm(form)
      if (groupName) setImportGroupName(groupName)
      setDirectImportText('')
      toast.success('文本已应用')
    } catch (error: any) {
      toast.error(error?.message || '文本应用失败')
    }
  }

  const handleImportUrlChange = (nextValue: string) => {
    setImportUrl(nextValue)
    if (importResolvedUrl.trim() && nextValue.trim() !== importResolvedUrl.trim()) {
      setImportResolvedUrl('')
    }
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
      if (!content) throw new Error('订阅内容为空')

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
          if (!previewGroupName) previewGroupName = parsed.defaultGroupName
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
      setRemovedPreviewProxyNames([])
      setPreviewList(preview)
      setImportModalOpen(false)
      setPreviewModalOpen(true)
    } catch (error: any) {
      toast.error(`解析失败: ${error?.message || '未知错误'}`)
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
      const sourceID = isURLImport ? resolveImportSourceID(proxies, sourceURL, sourceNamePrefix) : ''
      const sourceAutoRefresh = isURLImport ? globalAutoRefreshEnabled : false
      const sourceRefreshIntervalM = sourceAutoRefresh ? globalRefreshInterval : 0
      const sourceLastRefreshAt = isURLImport ? new Date().toISOString() : ''
      const oldSourceProxies = isURLImport
        ? proxies.filter(item => (item.sourceId || '').trim() === sourceID)
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
        ? proxies.filter(item => (item.sourceId || '').trim() !== sourceID).concat(newProxies)
        : [...proxies, ...newProxies]
      await saveProxies(allProxies)
      if (isURLImport && removedPreviewProxyNames.length > 0) {
        appendSourceIgnoredProxyNames(sourceID, removedPreviewProxyNames)
      }
      setPreviewModalOpen(false)
      setImportUrl('')
      setImportFetchProxyId('')
      setImportResolvedUrl('')
      setImportText('')
      setImportDnsServers('')
      setImportNamePrefix('')
      setImportGroupName('')
      setChainImportText('')
      setDirectImportText('')
      setChainImportForm(createInitialChainImportForm())
      setDirectImportForm({ ...INITIAL_DIRECT_IMPORT_FORM })
      setPreviewList([])
      setRemovedPreviewProxyNames([])
      toast.success(`成功导入 ${newProxies.length} 个代理`)
    } catch (error: any) {
      toast.error(error?.message || '导入失败')
    } finally {
      setImporting(false)
    }
  }

  const canParseImport = importMode === 'clash'
    ? !!importText.trim()
    : importMode === 'direct'
      ? !!directImportText.trim() || (!!directImportForm.server.trim() && !!directImportForm.port.trim())
      : !!chainImportForm.frontProxyId.trim()
        && !!chainImportForm.landing.server.trim()
        && !!chainImportForm.landing.port.trim()

  return {
    importModalOpen, setImportModalOpen, importMode, importUrl, importFetchProxyId, importResolvedUrl, importText,
    importDnsServers, importNamePrefix, importGroupName, chainImportText, directImportText,
    chainImportForm, directImportForm, previewModalOpen, setPreviewModalOpen, previewList, removedPreviewProxyNames,
    importing, fetchingImportUrl, canParseImport, setImportText, setImportDnsServers,
    setImportNamePrefix, setImportGroupName, setImportFetchProxyId, setChainImportText, setDirectImportText,
    setChainImportForm, setDirectImportForm, handleRemovePreviewProxy, updateChainImportLanding,
    handleImportModeChange, handleFillChainTemplate, handleFillDirectTemplate, handleCopyChainTemplate,
    handleCopyDirectTemplate, handleApplyChainJSON, handleApplyDirectText, handleImportUrlChange,
    handleFetchImportURL, handleParseImport, handleConfirmImport,
  }
}
