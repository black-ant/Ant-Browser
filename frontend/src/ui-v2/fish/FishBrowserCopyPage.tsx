import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, Check, Copy, Fingerprint, Sparkles } from 'lucide-react'
import type { BrowserProfile, BrowserProfileAutomationTarget, BrowserProfileCopyOptions } from '../../modules/browser/types'
import { copyBrowserProfile, fetchBrowserProfiles } from '../../modules/browser/api'
import {
  BROWSER_PROFILE_AUTOMATION_TARGET_OPTIONS,
  createBrowserProfileCopyOptions,
  isBrowserProfileCopyOptionsValid,
} from '../../modules/browser/copyOptions'
import { buildBrowserProfileCopyName } from '../../modules/browser/copyName'
import { toast } from '../../shared/components'
import './fish-copy.css'

export function FishBrowserCopyPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [sourceId, setSourceId] = useState(id || '')
  const [targetName, setTargetName] = useState('')
  const [copyOptions, setCopyOptions] = useState<BrowserProfileCopyOptions>(() => createBrowserProfileCopyOptions())
  const [saving, setSaving] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      setLoading(true)
      try {
        const list = await fetchBrowserProfiles()
        if (cancelled) return
        setProfiles(list)
        if (!sourceId && list.length) setSourceId(list[0].profileId)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [])

  const sourceProfile = useMemo(() => profiles.find((item) => item.profileId === sourceId), [profiles, sourceId])

  useEffect(() => {
    if (sourceProfile) setTargetName(buildBrowserProfileCopyName(sourceProfile.profileName))
  }, [sourceProfile?.profileId, sourceProfile?.profileName])

  const setMode = (mode: BrowserProfileCopyOptions['mode']) => setCopyOptions((current) => ({ ...current, mode }))
  const toggleTarget = (target: BrowserProfileAutomationTarget) => setCopyOptions((current) => ({
    ...current,
    automationTargets: current.automationTargets.includes(target)
      ? current.automationTargets.filter((item) => item !== target)
      : [...current.automationTargets, target],
  }))

  const valid = Boolean(sourceProfile && targetName.trim() && isBrowserProfileCopyOptionsValid(copyOptions))

  const handleCopy = async () => {
    if (!sourceProfile || !targetName.trim()) { toast.error('请填写目标名称'); return }
    if (!isBrowserProfileCopyOptionsValid(copyOptions)) { toast.error('请至少选择一个自动化指纹项'); return }
    setSaving(true)
    try {
      await copyBrowserProfile(sourceProfile.profileId, targetName.trim(), copyOptions)
      toast.success('实例已复制')
      navigate('/browser/list')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '复制失败')
    } finally { setSaving(false) }
  }

  if (loading) return <div className="fish-content"><div className="fish-empty"><strong>正在读取实例</strong><span>准备复制来源与默认选项。</span></div></div>

  return <div className="fish-content fish-copy-content">
    <div className="fish-edit-hero fish-copy-hero">
      <div><span className="fish-eyebrow">复制实例</span><h1>保留业务配置，重新生成需要变化的指纹。</h1><p>复制不会改动源实例；自动化模式只处理你明确选择的指纹范围。</p></div>
      <div className="fish-edit-actions"><Link className="fish-btn" to="/browser/list"><ArrowLeft size={16} strokeWidth={1.8} />返回</Link><button className="fish-btn primary" type="button" disabled={!valid || saving} onClick={() => void handleCopy()}><Copy size={16} strokeWidth={1.8} />{saving ? '复制中' : '生成副本'}</button></div>
    </div>

    <div className="fish-editor-grid fish-copy-grid">
      <main className="fish-editor-main">
        <section className="fish-panel">
          <div className="fish-panel-head"><div><strong>复制来源</strong><span>选择源实例并确认新实例名称。</span></div></div>
          <div className="fish-panel-body"><div className="fish-form-grid two"><label className="fish-field"><span>源实例</span><select className="fish-select" value={sourceId} onChange={(event) => setSourceId(event.target.value)}>{profiles.map((profile) => <option value={profile.profileId} key={profile.profileId}>{profile.profileName}</option>)}</select></label><label className="fish-field"><span>新实例名称 <b className="fish-required">*</b></span><input className="fish-input" value={targetName} onChange={(event) => setTargetName(event.target.value)} placeholder="请输入新实例名称" /></label></div></div>
        </section>

        <section className="fish-panel">
          <div className="fish-panel-head"><div><strong>复制方式</strong><span>直接复制保持原指纹；自动化指纹会按选定类别重新处理。</span></div></div>
          <div className="fish-panel-body">
            <div className="fish-copy-mode-grid">
              <button className={`fish-copy-mode${copyOptions.mode === 'regular' ? ' active' : ''}`} type="button" onClick={() => setMode('regular')}><span className="fish-copy-mode-icon"><Copy size={20} strokeWidth={1.8} /></span><span><strong>直接复制</strong><small>完整保留源实例当前的指纹参数。</small></span>{copyOptions.mode === 'regular' ? <Check size={16} strokeWidth={1.8} /> : null}</button>
              <button className={`fish-copy-mode${copyOptions.mode === 'auto_fingerprint' ? ' active' : ''}`} type="button" onClick={() => setMode('auto_fingerprint')}><span className="fish-copy-mode-icon accent"><Sparkles size={20} strokeWidth={1.8} /></span><span><strong>自动化指纹</strong><small>按你选择的范围生成新的指纹组合。</small></span>{copyOptions.mode === 'auto_fingerprint' ? <Check size={16} strokeWidth={1.8} /> : null}</button>
            </div>

            {copyOptions.mode === 'auto_fingerprint' ? <div className="fish-copy-targets"><div className="fish-copy-targets-head"><div><strong>自动化哪些指纹</strong><small>至少选择一项；默认只生成新的指纹种子。</small></div><span>{copyOptions.automationTargets.length} / {BROWSER_PROFILE_AUTOMATION_TARGET_OPTIONS.length}</span></div><div className="fish-copy-target-grid">{BROWSER_PROFILE_AUTOMATION_TARGET_OPTIONS.map((option) => {
              const checked = copyOptions.automationTargets.includes(option.value)
              return <button className={`fish-copy-target${checked ? ' active' : ''}`} type="button" key={option.value} onClick={() => toggleTarget(option.value)}><span className={`fish-copy-check${checked ? ' active' : ''}`}>{checked ? <Check size={16} strokeWidth={1.8} /> : null}</span><span><strong>{option.label}</strong><small>{option.detail}</small></span></button>
            })}</div>{!isBrowserProfileCopyOptionsValid(copyOptions) ? <div className="fish-validation error"><strong>至少选择一个自动化指纹项</strong><div><span>否则无法生成有效的自动化副本。</span></div></div> : null}</div> : null}
          </div>
        </section>
      </main>

      <aside className="fish-editor-aside">
        <section className="fish-panel sticky"><div className="fish-panel-head"><div><strong>副本摘要</strong><span>生成前的最终范围。</span></div></div><div className="fish-panel-body fish-summary-list"><div><span>源实例</span><strong>{sourceProfile?.profileName || '—'}</strong></div><div><span>新名称</span><strong>{targetName.trim() || '—'}</strong></div><div><span>复制方式</span><strong>{copyOptions.mode === 'regular' ? '直接复制' : '自动化指纹'}</strong></div><div><span>自动化范围</span><strong>{copyOptions.mode === 'regular' ? '不修改指纹' : `${copyOptions.automationTargets.length} 项`}</strong></div><div><span>源指纹参数</span><strong>{sourceProfile?.fingerprintArgs.length ?? 0} 项</strong></div><div><span>源代理</span><strong>{sourceProfile?.proxyId || sourceProfile?.proxyConfig || 'DIRECT'}</strong></div></div></section>
        <div className="fish-side-note"><Fingerprint size={20} strokeWidth={1.8} /><div><strong>源实例不会被修改</strong><span>副本会生成新的实例 ID、用户数据目录和快捷码；所选指纹项按现有复制业务规则处理。</span></div></div>
      </aside>
    </div>
  </div>
}
