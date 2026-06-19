import type { BrowserProxy, ProxyIPHealthResult } from './types'

export type ProxyScoreGrade = '优' | '良' | '中' | '差' | '-'

export interface ProxyScore {
  score: number // 0-100；-1 表示无数据
  grade: ProxyScoreGrade
}

// computeProxyScore 综合评分（前端派生，不持久化）。
// 权重：延迟 40% / 成功率 30% / 最近失败惩罚 15% / IP 风险 15%。
// latencyMs 来自 ProxyPoolPage 的 latencyMap：-1=测试中, -2=超时, -3=不支持, >=0=毫秒。
export function computeProxyScore(
  proxy: BrowserProxy,
  latencyMs: number | undefined,
  ipHealth: ProxyIPHealthResult | undefined,
): ProxyScore {
  // 解析延迟与成功：优先实时 latencyMap，其次历史字段
  let ms = typeof latencyMs === 'number'
    ? latencyMs
    : (proxy.lastTestedAt ? (proxy.lastTestOk ? (proxy.lastLatencyMs ?? -1) : -2) : undefined)

  const hasSpeed = typeof ms === 'number' && ms !== -1 && ms !== -3 // -1 测试中 / -3 不支持 视为无数据
  const ok = hasSpeed ? (ms as number) >= 0 : undefined

  if (ok === undefined && !ipHealth) {
    return { score: -1, grade: '-' }
  }

  // 延迟分（越低越好）
  let latencyScore = 50
  if (ok === false) {
    latencyScore = 0
  } else if (hasSpeed) {
    const v = ms as number
    if (v < 200) latencyScore = 100
    else if (v < 500) latencyScore = 80
    else if (v < 1000) latencyScore = 55
    else if (v < 2000) latencyScore = 30
    else latencyScore = 10
  }

  // 成功率分（暂以最近一次成功/失败近似，无历史成功率字段）
  const successScore = ok === true ? 100 : ok === false ? 0 : 50

  // 最近失败惩罚 / 新鲜度分
  let recencyScore = 50
  if (ok === false) {
    recencyScore = 0
  } else if (ok === true) {
    recencyScore = 100
    if (proxy.lastTestedAt) {
      const testedMs = Date.parse(proxy.lastTestedAt)
      if (!Number.isNaN(testedMs)) {
        const age = Date.now() - testedMs
        const dayMs = 24 * 60 * 60 * 1000
        if (age > 3 * dayMs) recencyScore = 60
        else if (age > dayMs) recencyScore = 80
      }
    }
  }

  // IP 风险分：fraudScore 越低越好；住宅 IP 略加分
  let ipScore = 50
  if (ipHealth && ipHealth.ok) {
    const fraud = typeof ipHealth.fraudScore === 'number' ? ipHealth.fraudScore : 50
    ipScore = Math.max(0, 100 - fraud)
    if (ipHealth.isResidential) ipScore = Math.min(100, ipScore + 15)
  } else if (ipHealth && !ipHealth.ok) {
    ipScore = 20
  }

  const score = Math.round(latencyScore * 0.4 + successScore * 0.3 + recencyScore * 0.15 + ipScore * 0.15)
  return { score, grade: scoreToGrade(score) }
}

export function scoreToGrade(score: number): ProxyScoreGrade {
  if (score < 0) return '-'
  if (score >= 80) return '优'
  if (score >= 60) return '良'
  if (score >= 40) return '中'
  return '差'
}

// proxyScoreVariant 评分档位对应的 Badge variant
export function proxyScoreVariant(grade: ProxyScoreGrade): 'success' | 'info' | 'warning' | 'error' | 'default' {
  switch (grade) {
    case '优':
      return 'success'
    case '良':
      return 'info'
    case '中':
      return 'warning'
    case '差':
      return 'error'
    default:
      return 'default'
  }
}
