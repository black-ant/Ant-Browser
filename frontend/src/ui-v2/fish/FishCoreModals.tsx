import type { Dispatch, SetStateAction } from 'react'
import { Download, ExternalLink, RefreshCw } from 'lucide-react'
import { toast } from '../../shared/components'
import type { BrowserCoreValidateResult, BrowserProxy } from '../../modules/browser/types'
import type { CoreDownloadForm, CoreDownloadProgress, CoreEditForm, CoreSettingsForm } from '../../modules/browser/pages/coreManagement.types'
import { FINGERPRINT_CHROMIUM_RELEASES_URL, type CoreDownloadRecommendation } from '../../modules/browser/pages/coreManagement/coreDownloadRecommendation'
import { FishModal, FishSwitch } from './FishFormPrimitives'

function openExternal(url: string) {
  const runtime = (window as Window & { runtime?: { BrowserOpenURL?: (target: string) => Promise<void> | void } }).runtime
  if (runtime?.BrowserOpenURL) {
    void runtime.BrowserOpenURL(url)
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

export function FishCoreSettingsModal({ open, form, saving, setForm, onClose, onSave }: {
  open: boolean
  form: CoreSettingsForm
  saving: boolean
  setForm: Dispatch<SetStateAction<CoreSettingsForm>>
  onClose: () => void
  onSave: () => void
}) {
  return <FishModal open={open} title="编辑全局浏览器基线" onClose={onClose} extraWide footer={<><button className="fish-btn" type="button" disabled={saving} onClick={onClose}>取消</button><button className="fish-btn primary" type="button" disabled={saving} onClick={onSave}>{saving ? '保存中' : '保存基线'}</button></>}>
    <div className="fish-core-modal-stack">
      <label className="fish-field"><span>用户数据根目录</span><input className="fish-input" value={form.userDataRoot} onChange={(event) => setForm((current) => ({ ...current, userDataRoot: event.target.value }))} placeholder="例如：data" /></label>
      <div className="fish-form-grid two">
        <label className="fish-field"><span>默认指纹参数</span><textarea className="fish-textarea" rows={7} value={form.defaultFingerprintArgs} onChange={(event) => setForm((current) => ({ ...current, defaultFingerprintArgs: event.target.value }))} placeholder="每行一个参数，如 --fingerprint-brand=Chrome" /></label>
        <label className="fish-field"><span>默认启动参数</span><textarea className="fish-textarea" rows={7} value={form.defaultLaunchArgs} onChange={(event) => setForm((current) => ({ ...current, defaultLaunchArgs: event.target.value }))} placeholder="每行一个参数，如 --disable-sync" /></label>
      </div>
      <label className="fish-field"><span>默认启动页面</span><textarea className="fish-textarea compact" rows={4} value={form.defaultStartUrls} onChange={(event) => setForm((current) => ({ ...current, defaultStartUrls: event.target.value }))} placeholder="每行一个 URL；留空则不自动打开页面" /></label>
      <div className="fish-core-setting-stack">
        <div className="fish-setting-row"><div><strong>轻启动模式</strong><span>先起空白页，实例就绪后再打开默认页面。</span></div><FishSwitch checked={form.lightStartEnabled} onChange={(checked) => setForm((current) => ({ ...current, lightStartEnabled: checked }))} /></div>
        <div className="fish-setting-row"><div><strong>默认恢复历史标签</strong><span>实例未覆盖时，下次启动恢复之前的标签页和窗口。</span></div><FishSwitch checked={form.restoreLastSession} onChange={(checked) => setForm((current) => ({ ...current, restoreLastSession: checked }))} /></div>
      </div>
      <div className="fish-form-grid two">
        <label className="fish-field"><span>启动就绪超时（毫秒）</span><input className="fish-input" type="number" min={1000} step={500} value={form.startReadyTimeoutMs} onChange={(event) => setForm((current) => ({ ...current, startReadyTimeoutMs: Math.max(1000, Number(event.target.value) || 3000) }))} /></label>
        <label className="fish-field"><span>启动稳定窗口（毫秒）</span><input className="fish-input" type="number" min={0} step={100} value={form.startStableWindowMs} onChange={(event) => setForm((current) => ({ ...current, startStableWindowMs: Math.max(0, Number(event.target.value) || 1200) }))} /></label>
      </div>
    </div>
  </FishModal>
}

export function FishCoreEditModal({ open, editing, form, saving, validating, validation, setForm, onClose, onSave }: {
  open: boolean
  editing: boolean
  form: CoreEditForm
  saving: boolean
  validating: boolean
  validation: BrowserCoreValidateResult | null
  setForm: Dispatch<SetStateAction<CoreEditForm>>
  onClose: () => void
  onSave: () => void
}) {
  return <FishModal open={open} title={editing ? '编辑 Chrome 内核' : '新增 Chrome 内核'} onClose={onClose} wide footer={<><button className="fish-btn" type="button" disabled={saving} onClick={onClose}>取消</button><button className="fish-btn primary" type="button" disabled={saving} onClick={onSave}>{saving ? '保存中' : editing ? '保存修改' : '添加内核'}</button></>}>
    <div className="fish-core-modal-stack">
      <label className="fish-field"><span>内核名称</span><input className="fish-input" value={form.coreName} onChange={(event) => setForm((current) => ({ ...current, coreName: event.target.value }))} placeholder="例如：Chrome 132" /></label>
      <label className="fish-field"><span>内核路径</span><input className="fish-input" value={form.corePath} onChange={(event) => setForm((current) => ({ ...current, corePath: event.target.value }))} placeholder="Chrome / Chromium 可执行文件路径" /></label>
      <div className={`fish-inline-note${validation?.valid ? ' success' : validation && !validation.valid ? ' danger' : ''}`}>{validating ? '正在验证路径…' : validation ? validation.message || (validation.valid ? '路径有效' : '路径无效') : '输入路径后会自动验证。'}</div>
    </div>
  </FishModal>
}

export function FishCoreDownloadModal({ open, form, progress, recommendation, recommendationLoading, recommendationError, proxies, setForm, setProgress, onRefreshRecommendation, onClose, onStart }: {
  open: boolean
  form: CoreDownloadForm
  progress: CoreDownloadProgress | null
  recommendation: CoreDownloadRecommendation | null
  recommendationLoading: boolean
  recommendationError: string
  proxies: BrowserProxy[]
  setForm: Dispatch<SetStateAction<CoreDownloadForm>>
  setProgress: Dispatch<SetStateAction<CoreDownloadProgress | null>>
  onRefreshRecommendation: () => void
  onClose: () => void
  onStart: () => void
}) {
  const downloading = Boolean(progress && progress.phase !== 'done' && progress.phase !== 'error')
  const isRedownload = form.mode === 'redownload'
  const requestClose = () => {
    if (downloading) { toast.warning('正在下载中，请稍候…'); return }
    onClose(); setProgress(null)
  }
  return <FishModal open={open} title={isRedownload ? '重新下载内核' : '下载 Chrome 内核'} onClose={requestClose} wide footer={<><button className="fish-btn" type="button" disabled={downloading} onClick={requestClose}>取消</button><button className="fish-btn primary" type="button" disabled={downloading} onClick={onStart}><Download size={16} strokeWidth={1.8} />{downloading ? '下载中' : isRedownload ? '开始重新下载' : '开始下载'}</button></>}>
    <div className="fish-core-modal-stack">
      <label className="fish-field"><span>内核名称</span><input className="fish-input" value={form.name} disabled={Boolean(progress) || isRedownload} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} placeholder={recommendation?.namePlaceholder || '例如：chrome-latest'} />{!isRedownload ? <small>名称同时作为内核数据子目录名。</small> : null}</label>
      {isRedownload ? <div className="fish-inline-note warning">重新下载会在校验新压缩包可用后替换当前目录；失败会恢复旧目录。正在使用该内核的实例应先停止。</div> : null}
      <label className="fish-field"><span>下载地址</span><input className="fish-input" value={form.url} disabled={Boolean(progress)} onChange={(event) => setForm((current) => ({ ...current, url: event.target.value }))} placeholder={recommendationLoading ? '正在匹配当前环境…' : 'https://github.com/.../chromium.zip'} /></label>
      <div className="fish-core-recommendation"><div><strong>{recommendation ? `${recommendation.target.label} · ${recommendation.assetName}` : recommendationLoading ? '正在按当前环境获取推荐版本' : recommendationError || '当前没有自动推荐版本'}</strong>{recommendation?.releaseTag ? <span>Release {recommendation.releaseTag}</span> : null}</div><div>{recommendation ? <button className="fish-mini-action" type="button" disabled={Boolean(progress)} onClick={() => setForm((current) => ({ ...current, url: recommendation.downloadUrl }))}>使用推荐</button> : !recommendationLoading ? <button className="fish-mini-action" type="button" onClick={onRefreshRecommendation}><RefreshCw size={16} strokeWidth={1.8} />重试</button> : null}<button className="fish-mini-action" type="button" onClick={() => openExternal(recommendation?.releasesUrl || FINGERPRINT_CHROMIUM_RELEASES_URL)}><ExternalLink size={16} strokeWidth={1.8} />Releases</button></div></div>
      <div className="fish-form-grid two">
        <label className="fish-field"><span>下载代理</span><select className="fish-select" value={form.proxyMode} disabled={Boolean(progress)} onChange={(event) => { const mode = event.target.value; setForm((current) => ({ ...current, proxyMode: mode, proxyId: mode === 'custom' && proxies.length ? proxies[0].proxyId : '' })) }}><option value="system">跟随系统全局代理</option><option value="direct">直连</option>{proxies.length ? <option value="custom">指定代理池节点</option> : null}</select></label>
        {form.proxyMode === 'custom' ? <label className="fish-field"><span>代理池节点</span><select className="fish-select" value={form.proxyId} disabled={Boolean(progress)} onChange={(event) => setForm((current) => ({ ...current, proxyId: event.target.value }))}>{proxies.map((proxy) => <option key={proxy.proxyId} value={proxy.proxyId}>{proxy.proxyName}</option>)}</select></label> : <div className="fish-core-download-hint"><strong>{form.proxyMode === 'direct' ? '直连下载' : '系统代理'}</strong><span>{form.proxyMode === 'direct' ? '忽略系统代理设置。' : '沿用系统当前全局代理。'}</span></div>}
      </div>
      {progress ? <div className="fish-progress-card"><div><strong>{progress.message}</strong><span>{Math.max(0, Math.min(100, progress.progress))}%</span></div><div className="fish-progress-track"><span style={{ width: `${Math.max(0, Math.min(100, progress.progress))}%` }} /></div></div> : null}
    </div>
  </FishModal>
}
