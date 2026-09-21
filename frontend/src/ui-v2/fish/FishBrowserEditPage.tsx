import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, ChevronDown, ChevronUp, FolderOpen, Gauge, HelpCircle, Layers3, MapPin, Save, ShieldCheck } from 'lucide-react'
import { toast } from '../../shared/components'
import type { BrowserCore, BrowserFingerprintCapabilityReport, BrowserFingerprintCheckResult, BrowserGroup, BrowserProfileInput, BrowserProxy, ProxyLocationResolveResult } from '../../modules/browser/types'
import {
  browserProxyResolveLocation,
  checkBrowserProfileFingerprint,
  createBrowserProfile,
  fetchAllTags,
  fetchBrowserCores,
  fetchBrowserProfileFingerprintMatrix,
  fetchBrowserProfiles,
  fetchBrowserProxies,
  fetchBrowserSettings,
  fetchGroups,
  openBrowserFingerprintCheck,
  openUserDataDir,
  updateBrowserProfile,
  validateProxyConfig,
} from '../../modules/browser/api'
import { applyLocaleToFingerprintArgs, validateFingerprintArgs, withAdaptiveDefaultWindowSize } from '../../modules/browser/utils/fingerprintSerializer'
import { FishConfirm, FishModal, FishTagInput } from './FishFormPrimitives'
import { FishFingerprintEditor } from './FishFingerprintEditor'
import { FishFingerprintCheckReport, FishFingerprintMatrixReport } from './FishFingerprintReports'

type ProxySourceMode = 'pool' | 'local'
type EditForm = BrowserProfileInput & { lastLaunchArgs?: string[] }
const DIRECT_PROXY_ID = '__direct__'
const FALLBACK_LAUNCH_ARGS = ['--disable-sync', '--no-first-run']

function normalizeLaunchArgs(args: string[]) { return (args || []).map((item) => item.trim()).filter(Boolean) }
function resolveDefaultLaunchArgs(args: string[]) { const normalized = normalizeLaunchArgs(args); return normalized.length ? normalized : FALLBACK_LAUNCH_ARGS }
function resolvePoolProxySelection(proxyId: string, proxyConfig: string, proxies: BrowserProxy[]): { mode: ProxySourceMode; proxyId: string; proxyConfig: string } {
  const id = proxyId.trim()
  if (id) {
    const matched = proxies.find((proxy) => proxy.proxyId.trim() === id)
    if (matched) return { mode: 'pool', proxyId: matched.proxyId, proxyConfig: '' }
  }
  const config = proxyConfig.trim()
  if (config) {
    const matched = proxies.find((proxy) => (proxy.proxyConfig || '').trim().toLowerCase() === config.toLowerCase())
    if (matched) return { mode: 'pool', proxyId: matched.proxyId, proxyConfig: '' }
    return { mode: 'local', proxyId: '', proxyConfig: config }
  }
  return { mode: 'pool', proxyId: proxies.find((proxy) => proxy.proxyId === DIRECT_PROXY_ID)?.proxyId || '', proxyConfig: '' }
}

function flattenGroups(groups: BrowserGroup[]) {
  const result: Array<BrowserGroup & { level: number }> = []
  const add = (parentId: string, level: number) => groups.filter((group) => group.parentId === parentId).sort((a, b) => a.sortOrder - b.sortOrder).forEach((group) => { result.push({ ...group, level }); add(group.groupId, level + 1) })
  add('', 0)
  return result
}

function FishField({ label, children, hint, required = false }: { label: string; children: React.ReactNode; hint?: string; required?: boolean }) {
  return <div className="fish-field"><label>{label}{required ? <span className="fish-required"> *</span> : null}</label>{children}{hint ? <small>{hint}</small> : null}</div>
}

export function FishBrowserEditPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const isCreate = id === 'new'
  const [loading, setLoading] = useState(true)
  const [formData, setFormData] = useState<EditForm>({ profileName: '', userDataDir: '', coreId: '', restoreLastSession: '', fingerprintArgs: [], proxyId: DIRECT_PROXY_ID, proxyConfig: '', memoryLimitMb: 0, launchArgs: [], lastLaunchArgs: [], tags: [], keywords: [], groupId: '' })
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [groups, setGroups] = useState<BrowserGroup[]>([])
  const [allTags, setAllTags] = useState<string[]>([])
  const [launchArgsText, setLaunchArgsText] = useState('')
  const [proxyMode, setProxyMode] = useState<ProxySourceMode>('pool')
  const [proxyPickerOpen, setProxyPickerOpen] = useState(false)
  const [proxySearch, setProxySearch] = useState('')
  const [proxyGroup, setProxyGroup] = useState('')
  const [locationResolving, setLocationResolving] = useState(false)
  const [locationResult, setLocationResult] = useState<ProxyLocationResolveResult | null>(null)
  const [fingerprintMatrix, setFingerprintMatrix] = useState<BrowserFingerprintCapabilityReport | null>(null)
  const [matrixOpen, setMatrixOpen] = useState(false)
  const [fingerprintChecking, setFingerprintChecking] = useState(false)
  const [fingerprintPageOpening, setFingerprintPageOpening] = useState(false)
  const [fingerprintResult, setFingerprintResult] = useState<BrowserFingerprintCheckResult | null>(null)
  const [fingerprintResultOpen, setFingerprintResultOpen] = useState(false)
  const [launchArgsOpen, setLaunchArgsOpen] = useState(false)
  const [launchHelpOpen, setLaunchHelpOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [isDirty, setIsDirty] = useState(false)
  const [leaveConfirm, setLeaveConfirm] = useState(false)
  const [saveError, setSaveError] = useState('')

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      setLoading(true)
      try {
        const [coreList, proxyList, tagList, groupList, settings] = await Promise.all([fetchBrowserCores(), fetchBrowserProxies(), fetchAllTags(), fetchGroups(), fetchBrowserSettings()])
        if (cancelled) return
        setCores(coreList); setProxies(proxyList); setAllTags(tagList); setGroups(groupList)
        if (isCreate) {
          const resolved = resolvePoolProxySelection('', '', proxyList)
          setProxyMode('pool')
          setFormData((current) => ({ ...current, proxyId: resolved.proxyId || DIRECT_PROXY_ID, proxyConfig: '', restoreLastSession: '', fingerprintArgs: withAdaptiveDefaultWindowSize(settings.defaultFingerprintArgs || []) }))
          setLaunchArgsText(resolveDefaultLaunchArgs(settings.defaultLaunchArgs || []).join('\n'))
        } else {
          const profiles = await fetchBrowserProfiles()
          if (cancelled) return
          const current = profiles.find((profile) => profile.profileId === id)
          if (!current) { setSaveError('没有找到该浏览器环境'); return }
          const resolvedProxy = resolvePoolProxySelection(current.proxyId || '', current.proxyConfig || '', proxyList)
          const currentLaunchArgs = normalizeLaunchArgs(current.launchArgs)
          setProxyMode(resolvedProxy.mode)
          setFormData({ profileName: current.profileName, userDataDir: current.userDataDir, coreId: !current.coreId || current.coreId.toLowerCase() === 'default' ? '' : current.coreId, restoreLastSession: current.restoreLastSession || '', fingerprintArgs: current.fingerprintArgs, proxyId: resolvedProxy.proxyId, proxyConfig: resolvedProxy.proxyConfig, memoryLimitMb: current.memoryLimitMb || 0, launchArgs: currentLaunchArgs, lastLaunchArgs: current.lastLaunchArgs || [], tags: current.tags, keywords: current.keywords || [], groupId: current.groupId || '' })
          setLaunchArgsText(currentLaunchArgs.join('\n'))
        }
      } catch (error) {
        if (!cancelled) setSaveError(error instanceof Error ? error.message : '加载配置失败')
      } finally { if (!cancelled) setLoading(false) }
    }
    void load()
    return () => { cancelled = true }
  }, [id, isCreate])

  const fingerprintKey = formData.fingerprintArgs.join('\n')
  useEffect(() => {
    let cancelled = false
    const loadMatrix = async () => {
      try {
        const report = await fetchBrowserProfileFingerprintMatrix(isCreate ? '' : id || '', formData.coreId, formData.fingerprintArgs)
        if (!cancelled) setFingerprintMatrix(report)
      } catch { if (!cancelled) setFingerprintMatrix(null) }
    }
    void loadMatrix()
    return () => { cancelled = true }
  }, [id, isCreate, formData.coreId, fingerprintKey])

  const change = (field: keyof BrowserProfileInput, value: string | string[] | number) => {
    setIsDirty(true)
    setFormData((current) => ({ ...current, [field]: value }))
  }

  const flatGroups = useMemo(() => flattenGroups(groups), [groups])
  const defaultCore = cores.find((core) => core.isDefault)
  const selectedProxy = proxies.find((proxy) => proxy.proxyId === formData.proxyId)
  const proxyGroups = useMemo(() => Array.from(new Set(proxies.map((proxy) => proxy.groupName || '').filter(Boolean))), [proxies])
  const filteredProxies = useMemo(() => proxies.filter((proxy) => (!proxyGroup || (proxy.groupName || '') === proxyGroup) && (!proxySearch.trim() || `${proxy.proxyName} ${proxy.proxyId} ${proxy.proxyConfig} ${proxy.groupName || ''}`.toLowerCase().includes(proxySearch.trim().toLowerCase()))), [proxies, proxyGroup, proxySearch])
  const fingerprintValidation = useMemo(() => validateFingerprintArgs(formData.fingerprintArgs), [formData.fingerprintArgs])

  const handleProxyModeChange = (mode: ProxySourceMode) => {
    setIsDirty(true); setProxyMode(mode); setLocationResult(null)
    if (mode === 'pool' && !formData.proxyId.trim()) change('proxyId', proxies.find((proxy) => proxy.proxyId === DIRECT_PROXY_ID)?.proxyId || '')
  }

  const handleSave = async () => {
    const resolvedProxyId = proxyMode === 'pool' ? (formData.proxyId || '').trim() : ''
    const resolvedProxyConfig = proxyMode === 'local' ? (formData.proxyConfig || '').trim() : ''
    if (!formData.profileName.trim()) { setSaveError('请输入配置名称'); return }
    if (proxyMode === 'local' && !resolvedProxyConfig) { setSaveError('请输入本地代理地址'); return }
    if (!fingerprintValidation.valid) { setSaveError(fingerprintValidation.issues.filter((issue) => issue.level === 'error').map((issue) => issue.message).join('\n')); return }
    const payload: BrowserProfileInput = { profileName: formData.profileName.trim(), userDataDir: formData.userDataDir, coreId: formData.coreId, restoreLastSession: formData.restoreLastSession || '', fingerprintArgs: formData.fingerprintArgs, tags: formData.tags, keywords: formData.keywords, groupId: formData.groupId, proxyId: resolvedProxyId || DIRECT_PROXY_ID, proxyConfig: resolvedProxyConfig, memoryLimitMb: Math.max(0, Math.floor(Number(formData.memoryLimitMb) || 0)), launchArgs: normalizeLaunchArgs(launchArgsText.split('\n')) }
    setSaving(true)
    try {
      const validation = await validateProxyConfig(payload.proxyConfig, payload.proxyId)
      if (!validation.supported) { setSaveError(validation.errorMsg || '代理配置无效'); return }
      if (isCreate) { await createBrowserProfile(payload); toast.success('配置已创建') } else if (id) { await updateBrowserProfile(id, payload); toast.success('配置已更新') }
      setIsDirty(false); navigate('/browser/list')
    } catch (error) { setSaveError(error instanceof Error ? error.message : typeof error === 'string' ? error : '保存失败') } finally { setSaving(false) }
  }

  const handleBack = () => { if (isDirty) setLeaveConfirm(true); else navigate('/browser/list') }
  const handleOpenDataDir = async () => { if (!formData.userDataDir.trim()) { toast.error('请先输入用户数据目录'); return } try { await openUserDataDir(formData.userDataDir) } catch (error) { toast.error(error instanceof Error ? error.message : '打开目录失败') } }
  const handleApplyProxyLocation = async () => {
    if (proxyMode !== 'pool' || !formData.proxyId || formData.proxyId === DIRECT_PROXY_ID) { toast.error('请选择代理池中的非直连节点'); return }
    setLocationResolving(true); setLocationResult(null)
    try {
      const result = await browserProxyResolveLocation(formData.proxyId); setLocationResult(result)
      if (!result.ok || !result.lang || !result.timezone) { toast.error(result.error || '无法根据代理 IP 匹配定位'); return }
      change('fingerprintArgs', applyLocaleToFingerprintArgs(formData.fingerprintArgs, result.lang, result.timezone)); toast.success(`已设置 ${result.lang} / ${result.timezone}`)
    } catch (error) { toast.error(error instanceof Error ? error.message : '代理定位失败') } finally { setLocationResolving(false) }
  }
  const handleFingerprintCheck = async () => {
    if (isCreate || !id) { toast.warning('请先保存实例，再启动后自测'); return }
    if (isDirty) { toast.warning('当前有未保存修改，请先保存后再自测'); return }
    setFingerprintChecking(true)
    try { const result = await checkBrowserProfileFingerprint(id); setFingerprintResult(result); setFingerprintResultOpen(true) } catch (error) { toast.error(error instanceof Error ? error.message : '指纹自测失败') } finally { setFingerprintChecking(false) }
  }
  const handleOpenFingerprintPage = async () => {
    if (isCreate || !id) { toast.warning('请先保存实例，再打开检测页'); return }
    if (isDirty) { toast.warning('当前有未保存修改，请先保存后再检测'); return }
    setFingerprintPageOpening(true)
    try { const profile = await openBrowserFingerprintCheck(id); if (profile?.lastLaunchArgs) setFormData((current) => ({ ...current, lastLaunchArgs: profile.lastLaunchArgs || [] })); toast.success('已在目标浏览器打开指纹检测页') } catch (error) { toast.error(error instanceof Error ? error.message : '打开指纹检测页失败') } finally { setFingerprintPageOpening(false) }
  }

  if (loading) return <div className="fish-content"><div className="fish-empty"><strong>正在读取环境配置</strong><span>载入内核、代理、标签与指纹默认值。</span></div></div>

  return (
    <div className="fish-content fish-edit-content">
      <div className="fish-edit-hero">
        <div><h1>{isCreate ? '创建新的浏览器环境' : '调整环境配置，不改变它的业务身份。'}</h1><p>基础信息、代理、指纹和启动行为集中在一个清晰的配置流里。</p></div>
        <div className="fish-edit-actions"><button className="fish-btn" type="button" onClick={handleBack}><ArrowLeft size={16} strokeWidth={1.8} />返回</button><button className="fish-btn primary" type="button" disabled={saving} onClick={() => void handleSave()}><Save size={16} strokeWidth={1.8} />{saving ? '保存中' : '保存配置'}</button></div>
      </div>

      <div className="fish-editor-grid">
        <div className="fish-editor-main">
          <section className="fish-panel">
            <div className="fish-panel-head"><div><strong>基础配置</strong><span>定义这个环境的身份、目录和运行资源。</span></div></div>
            <div className="fish-panel-body fish-form-grid two">
              <FishField label="配置名称" required><input className="fish-input" value={formData.profileName} onChange={(event) => change('profileName', event.target.value)} placeholder="例如：工作主账号" /></FishField>
              <FishField label="分组"><select className="fish-select" value={formData.groupId || ''} onChange={(event) => change('groupId', event.target.value)}><option value="">未分组</option>{flatGroups.map((group) => <option key={group.groupId} value={group.groupId}>{`${'　'.repeat(group.level)}${group.groupName}`}</option>)}</select></FishField>
              <FishField label="用户数据目录" hint="留空时由应用自动生成。"><div className="fish-inline-control"><input className="fish-input" value={formData.userDataDir} onChange={(event) => change('userDataDir', event.target.value)} placeholder="留空自动生成" /><button className="fish-icon-btn" type="button" title="打开目录" onClick={() => void handleOpenDataDir()}><FolderOpen size={16} strokeWidth={1.8} /></button></div></FishField>
              <FishField label="内核"><select className="fish-select" value={formData.coreId} onChange={(event) => change('coreId', event.target.value)}><option value="">{defaultCore ? `使用默认 (${defaultCore.coreName})` : '使用默认内核'}</option>{cores.map((core) => <option value={core.coreId} key={core.coreId}>{core.coreName}</option>)}</select></FishField>
              <FishField label="历史标签"><select className="fish-select" value={formData.restoreLastSession || ''} onChange={(event) => change('restoreLastSession', event.target.value)}><option value="">跟随内核默认</option><option value="enabled">开启：恢复历史标签</option><option value="disabled">关闭：不恢复历史标签</option></select></FishField>
              <FishField label="最大内存 MB" hint="0 表示不限制。"><input className="fish-input" type="number" min="0" step="128" value={formData.memoryLimitMb || 0} onChange={(event) => change('memoryLimitMb', Math.max(0, Math.floor(Number(event.target.value) || 0)))} /></FishField>
              <FishField label="标签"><FishTagInput value={formData.tags} onChange={(tags) => change('tags', tags)} suggestions={allTags} placeholder="输入标签后按回车" /></FishField>
            </div>
          </section>

          <section className="fish-panel">
            <div className="fish-panel-head"><div><strong>代理与定位</strong><span>选择代理来源，并可按出口 IP 同步语言与时区。</span></div><div className="fish-segment"><button type="button" className={proxyMode === 'pool' ? 'active' : ''} onClick={() => handleProxyModeChange('pool')}>代理池</button><button type="button" className={proxyMode === 'local' ? 'active' : ''} onClick={() => handleProxyModeChange('local')}>本地代理</button></div></div>
            <div className="fish-panel-body">
              {proxyMode === 'pool' ? <div className="fish-proxy-config"><FishField label="代理节点"><div className="fish-inline-control"><select className="fish-select" value={formData.proxyId} onChange={(event) => { change('proxyId', event.target.value); setLocationResult(null) }}>{proxies.length ? proxies.map((proxy) => <option key={proxy.proxyId} value={proxy.proxyId}>{proxy.proxyName || proxy.proxyId}</option>) : <option value="">暂无代理</option>}</select><button className="fish-icon-btn" type="button" title="按分组选择" onClick={() => setProxyPickerOpen(true)}><Layers3 size={16} strokeWidth={1.8} /></button></div></FishField><button className="fish-btn" type="button" disabled={!formData.proxyId || formData.proxyId === DIRECT_PROXY_ID || locationResolving} onClick={() => void handleApplyProxyLocation()}><MapPin size={16} strokeWidth={1.8} />{locationResolving ? '定位中' : '按代理匹配定位'}</button></div> : <FishField label="本地代理地址" hint="支持 http://、https://、socks5://；只对当前实例保存生效。"><input className="fish-input" value={formData.proxyConfig} onChange={(event) => change('proxyConfig', event.target.value)} placeholder="http://127.0.0.1:7890" /></FishField>}
              {locationResult ? <div className={`fish-location-result${locationResult.ok ? '' : ' error'}`}><MapPin size={16} strokeWidth={1.8} /><span>{locationResult.ok ? `出口 ${locationResult.ip || '—'} · ${[locationResult.country, locationResult.region, locationResult.city].filter(Boolean).join(' / ') || '—'} · ${locationResult.lang} · ${locationResult.timezone}` : locationResult.error || '未匹配到定位'}</span></div> : null}
            </div>
          </section>

          <section className="fish-panel">
            <div className="fish-panel-head"><div><strong>指纹配置</strong><span>保持身份、定位、设备与防泄漏策略一致。</span></div><div className="fish-panel-actions"><button className="fish-icon-btn" type="button" title="版本适配矩阵" onClick={() => setMatrixOpen(true)}><HelpCircle size={16} strokeWidth={1.8} /></button><button className="fish-btn" type="button" disabled={isCreate || fingerprintPageOpening} onClick={() => void handleOpenFingerprintPage()}>{fingerprintPageOpening ? '打开中' : '浏览器内检测'}</button><button className="fish-btn" type="button" disabled={isCreate || fingerprintChecking} onClick={() => void handleFingerprintCheck()}><ShieldCheck size={16} strokeWidth={1.8} />{fingerprintChecking ? '自测中' : '自测当前实例'}</button></div></div>
            <div className="fish-panel-body"><FishFingerprintEditor value={formData.fingerprintArgs} onChange={(args) => change('fingerprintArgs', args)} /></div>
          </section>

          <section className="fish-collapsible fish-launch-collapsible">
            <div className="fish-collapsible-trigger"><button className="fish-collapsible-main" type="button" onClick={() => setLaunchArgsOpen((current) => !current)}><span>高级启动参数</span>{launchArgsOpen ? <ChevronUp size={16} strokeWidth={1.8} /> : <ChevronDown size={16} strokeWidth={1.8} />}</button><button className="fish-help-inline" type="button" aria-label="查看启动参数说明" onClick={() => setLaunchHelpOpen(true)}><HelpCircle size={16} strokeWidth={1.8} /></button></div>
            {launchArgsOpen ? <div className="fish-collapsible-body"><textarea className="fish-textarea" rows={7} value={launchArgsText} onChange={(event) => { setLaunchArgsText(event.target.value); setIsDirty(true) }} placeholder="--disable-sync" />{!isCreate && formData.lastLaunchArgs?.length ? <div className="fish-readonly-block"><strong>上次实际启动参数</strong><textarea className="fish-textarea" rows={6} readOnly value={formData.lastLaunchArgs.join('\n')} /></div> : null}</div> : null}
          </section>
        </div>

        <aside className="fish-editor-aside">
          <section className="fish-panel sticky"><div className="fish-panel-head"><div><strong>配置摘要</strong><span>保存前快速确认关键状态。</span></div></div><div className="fish-panel-body fish-summary-list"><div><span>名称</span><strong>{formData.profileName || '未命名'}</strong></div><div><span>分组</span><strong>{groups.find((group) => group.groupId === formData.groupId)?.groupName || '未分组'}</strong></div><div><span>内核</span><strong>{cores.find((core) => core.coreId === formData.coreId)?.coreName || defaultCore?.coreName || '默认内核'}</strong></div><div><span>代理</span><strong>{proxyMode === 'pool' ? selectedProxy?.proxyName || selectedProxy?.proxyId || 'DIRECT' : formData.proxyConfig || '未填写'}</strong></div><div><span>指纹</span><strong className={fingerprintValidation.valid ? 'fish-good' : 'fish-bad'}>{fingerprintValidation.valid ? '可保存' : '需要修正'}</strong></div><div><span>状态</span><strong>{isDirty ? '有未保存修改' : isCreate ? '新建配置' : '已同步'}</strong></div></div></section>
        </aside>
      </div>

      <FishModal open={proxyPickerOpen} title="选择代理节点" onClose={() => setProxyPickerOpen(false)} wide>
        <div className="fish-proxy-picker-tools"><label className="fish-search"><Gauge size={16} strokeWidth={1.8} /><input value={proxySearch} onChange={(event) => setProxySearch(event.target.value)} placeholder="搜索代理名称、地址或分组" /></label><select className="fish-select" value={proxyGroup} onChange={(event) => setProxyGroup(event.target.value)}><option value="">全部分组</option>{proxyGroups.map((group) => <option value={group} key={group}>{group}</option>)}</select></div>
        <div className="fish-choice-list">{filteredProxies.map((proxy) => <button className={`fish-choice-row${formData.proxyId === proxy.proxyId ? ' active' : ''}`} type="button" key={proxy.proxyId} onClick={() => { change('proxyId', proxy.proxyId); setLocationResult(null); setProxyPickerOpen(false) }}><div><strong>{proxy.proxyName || proxy.proxyId}</strong><span>{proxy.groupName || '未分组'} · {proxy.proxyConfig || 'DIRECT'}</span></div><small>{proxy.lastTestOk ? `${proxy.lastLatencyMs || 0} ms` : '未测速'}</small></button>)}</div>
      </FishModal>

      <FishModal open={matrixOpen} title="版本适配矩阵" onClose={() => setMatrixOpen(false)} extraWide footer={<button className="fish-btn" type="button" onClick={() => setMatrixOpen(false)}>关闭</button>}><FishFingerprintMatrixReport report={fingerprintMatrix} /></FishModal>
      <FishModal open={fingerprintResultOpen} title="指纹自测结果" onClose={() => setFingerprintResultOpen(false)} extraWide footer={<button className="fish-btn" type="button" onClick={() => setFingerprintResultOpen(false)}>关闭</button>}>{fingerprintResult ? <FishFingerprintCheckReport result={fingerprintResult} /> : <div className="fish-empty compact"><strong>暂无自测结果</strong></div>}</FishModal>
      <FishModal open={launchHelpOpen} title="高级启动参数说明" onClose={() => setLaunchHelpOpen(false)} extraWide footer={<button className="fish-btn" type="button" onClick={() => setLaunchHelpOpen(false)}>关闭</button>}><div className="fish-report-table help"><div className="fish-report-head"><span>参数</span><span>含义</span><span>示例</span></div>{[['--disable-sync','关闭 Chrome 同步，减少账号和同步服务干扰。','--disable-sync'],['--no-first-run','跳过首次运行向导，启动更干净。','--no-first-run'],['--start-maximized','启动后最大化窗口；会影响窗口大小检测口径。','--start-maximized'],['--disable-background-networking','减少后台网络请求；可能影响部分 Chrome 服务。','--disable-background-networking'],['--disable-features=<list>','关闭指定 Chromium Feature；仅在明确知道影响时使用。','--disable-features=Translate'],['--enable-features=<list>','开启指定 Chromium Feature。','--enable-features=NetworkService'],['不建议手填','指纹、代理、用户数据目录、调试端口属于托管参数。','--user-data-dir / --proxy-server'],['上次实际启动参数','只读记录，展示最终启动 Chrome 的完整参数。','下方只读区域']].map(([arg, meaning, example]) => <div className="fish-report-row" key={arg}><code>{arg}</code><span>{meaning}</span><code>{example}</code></div>)}</div></FishModal>
      <FishConfirm open={leaveConfirm} title="放弃未保存的更改？" content="当前页面有未保存的修改，离开后将丢失这些更改。" confirmText="放弃并离开" cancelText="继续编辑" danger onClose={() => setLeaveConfirm(false)} onConfirm={() => navigate('/browser/list')} />
      <FishModal open={Boolean(saveError)} title="操作失败" onClose={() => setSaveError('')} footer={<button className="fish-btn primary" type="button" onClick={() => setSaveError('')}>知道了</button>}><div className="fish-modal-copy">{saveError}</div></FishModal>
    </div>
  )
}
