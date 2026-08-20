import { useState } from 'react'

import { Button, Modal, toast } from '../../../shared/components'
import type { BrowserProfile, BrowserProxy } from '../types'
import { updateBrowserProfile } from '../api'

interface BatchProxyModalProps {
  open: boolean
  profiles: BrowserProfile[]
  proxies: BrowserProxy[]
  onClose: () => void
  onDone: () => void
}

export function BatchProxyModal({ open, profiles, proxies, onClose, onDone }: BatchProxyModalProps) {
  const [proxyId, setProxyId] = useState('')
  const [busy, setBusy] = useState(false)

  const runningCount = profiles.filter(p => p.running).length

  const doApply = async () => {
    if (!proxyId) return
    setBusy(true)
    let success = 0
    let failed = 0
    for (const profile of profiles) {
      try {
        await updateBrowserProfile(profile.profileId, {
          profileName: profile.profileName,
          userDataDir: profile.userDataDir,
          coreId: profile.coreId,
          fingerprintArgs: profile.fingerprintArgs,
          proxyId,
          proxyConfig: '',
          memoryLimitMb: profile.memoryLimitMb || 0,
          launchArgs: profile.launchArgs,
          tags: profile.tags,
          keywords: profile.keywords || [],
          groupId: profile.groupId || '',
        })
        success++
      } catch {
        failed++
      }
    }
    setBusy(false)
    const parts = [`已设置 ${success} 个实例`]
    if (failed > 0) parts.push(`失败 ${failed} 个`)
    if (runningCount > 0) parts.push(`${runningCount} 个运行中，重启后生效`)
    if (failed > 0) {
      toast.warning(parts.join('，'))
    } else {
      toast.success(parts.join('，'))
    }
    onDone()
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={`批量设置代理(已选 ${profiles.length} 个)`}
      width="420px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>取消</Button>
          <Button onClick={doApply} loading={busy} disabled={!proxyId}>应用</Button>
        </>
      }
    >
      <div className="space-y-3">
        <select
          value={proxyId}
          onChange={(e) => setProxyId(e.target.value)}
          className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-accent)]"
        >
          <option value="">选择代理</option>
          {proxies.map(p => (
            <option key={p.proxyId} value={p.proxyId}>{p.proxyName || p.proxyId}</option>
          ))}
        </select>
        <p className="text-xs text-[var(--color-text-muted)]">
          所有选中实例将使用同一条代理（共用出口 IP，注意平台关联风险）。
          {runningCount > 0 && `其中 ${runningCount} 个正在运行，需重启实例后生效。`}
        </p>
      </div>
    </Modal>
  )
}
