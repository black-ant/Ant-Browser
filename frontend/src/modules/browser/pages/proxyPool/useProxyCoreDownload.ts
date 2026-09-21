import { useCallback, useEffect, useState } from 'react'
import { toast } from '../../../../shared/components'
import { browserProxyCoreDownload, browserProxyCoreStatus } from '../../api'
import type { ProxyCoreDownloadProgress, ProxyCoreStatusResult } from '../../types'


function defaultProxyCoreTarget(): { goos: string; goarch: string } {
  const platform = navigator.platform.toLowerCase()
  const userAgent = navigator.userAgent.toLowerCase()
  const goos = platform.includes('mac') ? 'darwin' : platform.includes('linux') ? 'linux' : 'windows'
  const goarch = platform.includes('arm') || userAgent.includes('arm64') || userAgent.includes('aarch64') ? 'arm64' : 'amd64'
  return { goos, goarch }
}

export function useProxyCoreDownload() {
  const defaultTarget = defaultProxyCoreTarget()
  const [coreDownloadOpen, setCoreDownloadOpen] = useState(false)
  const [coreDownloadType, setCoreDownloadType] = useState('xray')
  const [coreDownloadGOOS, setCoreDownloadGOOS] = useState(defaultTarget.goos)
  const [coreDownloadGOARCH, setCoreDownloadGOARCH] = useState(defaultTarget.goarch)
  const [coreDownloadProxy, setCoreDownloadProxy] = useState('')
  const [coreDownloadProgress, setCoreDownloadProgress] = useState<ProxyCoreDownloadProgress | null>(null)
  const [currentCoreStatus, setCurrentCoreStatus] = useState<ProxyCoreStatusResult | null>(null)
  const [downloadCoreStatus, setDownloadCoreStatus] = useState<ProxyCoreStatusResult | null>(null)
  const [downloadCoreStatusLoading, setDownloadCoreStatusLoading] = useState(false)

  const refreshCurrentCoreStatus = useCallback(async (core: string) => {
    const target = defaultProxyCoreTarget()
    const status = await browserProxyCoreStatus(core || 'xray', target.goos, target.goarch)
    setCurrentCoreStatus(status)
    return status
  }, [])

  const refreshDownloadCoreStatus = useCallback(async (
    core = coreDownloadType,
    goos = coreDownloadGOOS,
    goarch = coreDownloadGOARCH,
  ) => {
    setDownloadCoreStatusLoading(true)
    try {
      const status = await browserProxyCoreStatus(core, goos, goarch)
      setDownloadCoreStatus(status)
      return status
    } finally {
      setDownloadCoreStatusLoading(false)
    }
  }, [coreDownloadType, coreDownloadGOOS, coreDownloadGOARCH])

  const loadBrowserSettings = useCallback(async () => {
    try {
      await refreshCurrentCoreStatus('xray')
    } catch (error: any) {
      toast.error(error?.message || '读取代理内核状态失败')
    }
  }, [refreshCurrentCoreStatus])

  useEffect(() => {
    const onProgress = (data: ProxyCoreDownloadProgress) => {
      setCoreDownloadProgress(data)
      if (data.phase === 'done') {
        toast.success(data.message || '代理内核已安装')
        void refreshDownloadCoreStatus(data.core, data.goos, data.goarch)
        void refreshCurrentCoreStatus(data.core)
      }
      if (data.phase === 'error') toast.error(data.message || '代理内核下载失败')
    }
    const runtime = (window as Window & { runtime?: { EventsOn?: (event: string, callback: (data: ProxyCoreDownloadProgress) => void) => (() => void) | void } }).runtime
    const off = runtime?.EventsOn?.('proxy-core:download:progress', onProgress) || (() => {})
    return off
  }, [refreshCurrentCoreStatus, refreshDownloadCoreStatus])

  useEffect(() => {
    if (coreDownloadOpen) void refreshDownloadCoreStatus(coreDownloadType, coreDownloadGOOS, coreDownloadGOARCH)
  }, [coreDownloadOpen, coreDownloadType, coreDownloadGOOS, coreDownloadGOARCH, refreshDownloadCoreStatus])

  const handleStartCoreDownload = useCallback(async () => {
    const downloadProxy = coreDownloadProxy.trim()
    setCoreDownloadProgress({ core: coreDownloadType, goos: coreDownloadGOOS, goarch: coreDownloadGOARCH, phase: 'starting', progress: 0, message: downloadProxy ? `准备下载（指定代理：${downloadProxy}）` : '准备下载（直连）' })
    try {
      await browserProxyCoreDownload(coreDownloadType, coreDownloadGOOS, coreDownloadGOARCH, downloadProxy)
    } catch (error: any) {
      const message = error?.message || '启动下载失败'
      setCoreDownloadProgress({ core: coreDownloadType, goos: coreDownloadGOOS, goarch: coreDownloadGOARCH, phase: 'error', progress: 0, message })
      toast.error(message)
    }
  }, [coreDownloadProxy, coreDownloadType, coreDownloadGOOS, coreDownloadGOARCH])

  const openCoreDownload = useCallback(() => {
    setCoreDownloadProgress(null)
    setCoreDownloadOpen(true)
    void refreshDownloadCoreStatus()
  }, [refreshDownloadCoreStatus])

  const closeCoreDownload = useCallback(() => {
    setCoreDownloadOpen(false)
    setCoreDownloadProgress(null)
  }, [])

  return {
    coreDownloadOpen,
    setCoreDownloadOpen,
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
  }
}
