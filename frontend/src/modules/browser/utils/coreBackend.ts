// 浏览器内核后端（fingerprint-chromium / Cloak）的展示与归一化工具。
//
// 后端差异不只是"参数多寡"，而是同一参数的结论可能相反：
// 例如 GPU vendor / renderer、设备内存、屏幕宽高在 fingerprint-chromium 上实测无效，
// 但在 Cloak 上是受支持参数。所以指纹面板必须知道当前实例用的是哪个后端。

import type { BrowserCoreBackend } from '../types'

export const CORE_BACKEND_FINGERPRINT_CHROMIUM = 'fingerprint_chromium'
export const CORE_BACKEND_CLOAK = 'cloak'

export interface CoreBackendOption {
  value: BrowserCoreBackend
  label: string
  description: string
}

export const CORE_BACKEND_OPTIONS: CoreBackendOption[] = [
  {
    value: CORE_BACKEND_FINGERPRINT_CHROMIUM,
    label: 'fingerprint-chromium',
    description: '目录下需存在 chrome.exe 与 manifest.json，指纹参数按 Chrome 144+ 实测矩阵处理',
  },
  {
    value: CORE_BACKEND_CLOAK,
    label: 'Cloak',
    description: 'CloakBrowser 源码级 patch 内核，目录下需存在 chromium-<版本>/ 子目录（可带 -pro 后缀），噪声由指纹种子驱动',
  },
]

// normalizeCoreBackend 归一化后端标记，空值和未知值都按 fingerprint-chromium 处理，
// 与后端 config.NormalizeCoreBackend 保持一致。
export function normalizeCoreBackend(value: string | undefined | null): BrowserCoreBackend {
  const text = (value || '').trim().toLowerCase()
  if (text === CORE_BACKEND_CLOAK || text === 'cloakbrowser' || text === 'cloak-browser' || text === 'cloak_browser') {
    return CORE_BACKEND_CLOAK
  }
  return CORE_BACKEND_FINGERPRINT_CHROMIUM
}

export function coreBackendLabel(value: string | undefined | null): string {
  return normalizeCoreBackend(value) === CORE_BACKEND_CLOAK ? 'Cloak' : 'fingerprint-chromium'
}

export function isCloakBackend(value: string | undefined | null): boolean {
  return normalizeCoreBackend(value) === CORE_BACKEND_CLOAK
}

// parseCoreEnvInput 把多行文本解析为 KEY=VALUE 列表，忽略空行与注释行。
export function parseCoreEnvInput(text: string): string[] {
  return (text || '')
    .split('\n')
    .map(line => line.trim())
    .filter(line => line !== '' && !line.startsWith('#'))
}

