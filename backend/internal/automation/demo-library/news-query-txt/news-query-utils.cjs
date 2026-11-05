const DEFAULT_EXCLUDED_DOMAINS = [
  'zhihu.com',
  'baidu.com',
  'qq.com',
  '36kr.com',
  'apifox.com',
  'chatgpt-chinese.com',
  'openwebui.cn',
  'open-openai.com',
  'xiniushu.com',
  'reddit.com',
  'quora.com',
  'tieba.baidu.com',
  'weibo.com',
  'x.com',
  'twitter.com',
  'youtube.com',
  'bilibili.com',
  'douyin.com',
  'xiaohongshu.com',
]

function normalizeInt(value, fallback, min, max) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) {
    return fallback
  }

  const rounded = Math.round(parsed)
  if (rounded < min) {
    return min
  }
  if (rounded > max) {
    return max
  }
  return rounded
}

function normalizeText(value) {
  return String(value || '').trim()
}

function normalizeDomainList(value) {
  if (!Array.isArray(value)) {
    return []
  }

  const deduped = new Set()
  for (const item of value) {
    const normalized = normalizeText(item).replace(/^https?:\/\//, '').replace(/^www\./, '').toLowerCase()
    if (normalized) {
      deduped.add(normalized)
    }
  }
  return Array.from(deduped)
}

function buildDefaultQuery(keyword) {
  const normalizedKeyword = normalizeText(keyword) || 'OpenAI'
  if (/[\u3400-\u9fff]/.test(normalizedKeyword)) {
    return normalizedKeyword + ' 新闻'
  }
  return normalizedKeyword + ' news'
}

function buildFallbackQueries(keyword, baseQuery) {
  const normalizedKeyword = normalizeText(keyword) || 'OpenAI'
  const normalizedBaseQuery = normalizeText(baseQuery)
  const candidates = [
    normalizedBaseQuery,
  ]

  if (/[\u3400-\u9fff]/.test(normalizedKeyword)) {
    candidates.push(normalizedKeyword + ' 最新新闻')
  } else {
    candidates.push(normalizedKeyword + ' latest news')
  }

  const deduped = new Set()
  for (const item of candidates) {
    const normalized = normalizeText(item)
    if (normalized) {
      deduped.add(normalized)
    }
  }
  return Array.from(deduped)
}

function buildSearchQuery(baseQuery, excludedDomains) {
  const normalizedBaseQuery = normalizeText(baseQuery)
  const normalizedDomains = normalizeDomainList(excludedDomains)
  const parts = [normalizedBaseQuery]

  for (const domain of normalizedDomains) {
    parts.push('-site:' + domain)
  }

  return parts.filter(Boolean).join(' ')
}

function mapTimeRangeToBingFilter(value) {
  switch (normalizeText(value).toLowerCase()) {
    case 'day':
    case '24h':
    case 'today':
      return 'ex1:"ez1"'
    case 'week':
      return 'ex1:"ez2"'
    case 'month':
      return 'ex1:"ez3"'
    default:
      return ''
  }
}

function buildSearchURL(query, timeRange, firstResultIndex) {
  const searchParams = new URLSearchParams({ q: query })
  const filter = mapTimeRangeToBingFilter(timeRange)
  if (filter) {
    searchParams.set('filters', filter)
  }
  if (Number.isFinite(firstResultIndex) && firstResultIndex > 1) {
    searchParams.set('first', String(firstResultIndex))
  }
  return 'https://www.bing.com/search?' + searchParams.toString()
}

function splitSnippet(snippet) {
  const normalized = normalizeText(snippet)
  if (!normalized) {
    return { publishedAt: '', summary: '' }
  }

  const match = normalized.match(/^([^·]{0,40})\s*·\s*(.+)$/)
  if (
    match &&
    /(前|分钟|小时|天前|周前|月前|昨天|\d{4}|\d{1,2}[/-]\d{1,2})/.test(match[1])
  ) {
    return {
      publishedAt: normalizeText(match[1]),
      summary: normalizeText(match[2]),
    }
  }

  return {
    publishedAt: '',
    summary: normalized,
  }
}

function parseHostname(rawUrl) {
  const normalized = normalizeText(rawUrl)
  if (!normalized) {
    return ''
  }

  try {
    return new URL(normalized).hostname.replace(/^www\./, '').toLowerCase()
  } catch {
    return ''
  }
}

function parsePathname(rawUrl) {
  const normalized = normalizeText(rawUrl)
  if (!normalized) {
    return ''
  }

  try {
    const pathname = new URL(normalized).pathname.replace(/\/+/g, '/').toLowerCase()
    if (!pathname) {
      return ''
    }
    return pathname === '/' ? pathname : pathname.replace(/\/$/, '')
  } catch {
    return ''
  }
}

function looksLikeQuestionTitle(title) {
  const normalized = normalizeText(title)
  if (!normalized) {
    return false
  }

  if (/[？?]/.test(normalized)) {
    return true
  }

  return /^(如何|为什么|怎么看|怎样|怎么|是否|有没有|谁能|请问|评价|如何评价|如何看待|为什么说)/.test(normalized)
}

function looksLikeAggregateText(text) {
  const normalized = normalizeText(text).toLowerCase()
  if (!normalized) {
    return false
  }

  return /(roundup|digest|flash report|llm news today|ai news today|daily ai news|news today|model releases)/.test(normalized)
}

function looksLikeListingPath(pathname) {
  const normalized = normalizeText(pathname).toLowerCase()
  if (!normalized || normalized === '/') {
    return false
  }

  if (/(^|\/)(tag|tags|topic|topics|category|categories|label|labels|brand|brands)(\/|$)/.test(normalized)) {
    return true
  }

  if (/(^|\/)(news|latest|headlines|insights)$/.test(normalized)) {
    return true
  }

  return /\/news\/(brand|brands|topic|topics|tag|tags)(\/|$)/.test(normalized)
}

function looksLikeListingText(text) {
  const normalized = normalizeText(text).toLowerCase()
  if (!normalized) {
    return false
  }

  return /(latest news|breaking headlines|news and insights|news and analysis|everything you need to know|get the latest|最新资讯|最新动态|实时追踪|热点快讯|快讯)/.test(normalized)
}

function isBlockedHostname(hostname) {
  const normalized = normalizeText(hostname).toLowerCase()
  if (!normalized) {
    return false
  }

  const blockedSuffixes = DEFAULT_EXCLUDED_DOMAINS
  const blockedKeywords = [
    'aitrack',
    'aitoolly',
    'aiflashreport',
    'llm-stats',
    'opentools',
  ]

  if (blockedSuffixes.some(function (suffix) {
    return normalized === suffix || normalized.endsWith('.' + suffix)
  })) {
    return true
  }

  return blockedKeywords.some(function (keyword) {
    return normalized.includes(keyword)
  })
}

function evaluateNewsItem(item) {
  const hostname = parseHostname(item.url)
  const pathname = parsePathname(item.url)
  const summary = normalizeText(item.summary)
  const source = normalizeText(item.source)
  const reasons = []

  if (!normalizeText(item.url)) {
    reasons.push('missing-url')
  }
  if (!hostname) {
    reasons.push('invalid-url')
  }
  if (hostname && isBlockedHostname(hostname)) {
    reasons.push('blocked-host')
  }
  if (!source) {
    reasons.push('missing-source')
  }
  if (summary.length < 20) {
    reasons.push('summary-too-short')
  }
  if (looksLikeQuestionTitle(item.title)) {
    reasons.push('question-title')
  }
  if (looksLikeAggregateText(item.title) || looksLikeAggregateText(summary)) {
    reasons.push('aggregate-page')
  }
  if (looksLikeListingPath(pathname) || looksLikeListingText(item.title) || looksLikeListingText(summary)) {
    reasons.push('listing-page')
  }

  return Object.assign({}, item, {
    hostname: hostname,
    pathname: pathname,
    qualityAccepted: reasons.length === 0,
    qualityReasons: reasons,
  })
}

function formatRejectedReason(reason) {
  switch (reason) {
    case 'missing-url':
      return '缺少链接'
    case 'invalid-url':
      return '链接无效'
    case 'blocked-host':
      return '来源站点已过滤'
    case 'missing-source':
      return '缺少来源'
    case 'summary-too-short':
      return '摘要过短'
    case 'question-title':
      return '标题更像问答'
    case 'aggregate-page':
      return '更像聚合页'
    case 'listing-page':
      return '更像列表页/专题页'
    default:
      return reason
  }
}

function formatReport(items, metadata) {
  const lines = [
    '新闻抓取结果',
    '查询词: ' + metadata.query,
    '抓取时间: ' + metadata.generatedAt,
    '搜索地址: ' + metadata.searchUrl,
    '原始结果: ' + metadata.rawCount,
    '通过校验: ' + items.length,
    '过滤数量: ' + metadata.rejectedItems.length,
    '',
  ]

  for (const item of items) {
    lines.push(item.rank + '. ' + item.title)
    if (item.source) {
      lines.push('来源: ' + item.source)
    }
    if (item.publishedAt) {
      lines.push('时间: ' + item.publishedAt)
    }
    lines.push('链接: ' + item.url)
    if (item.summary) {
      lines.push('摘要: ' + item.summary)
    }
    lines.push('')
  }

  if (metadata.rejectedItems.length > 0) {
    lines.push('被过滤结果（最多展示 5 条）')
    lines.push('')
    for (const item of metadata.rejectedItems.slice(0, 5)) {
      lines.push(item.rank + '. ' + item.title)
      if (item.hostname) {
        lines.push('站点: ' + item.hostname)
      }
      lines.push('原因: ' + item.qualityReasons.map(formatRejectedReason).join(' / '))
      lines.push('')
    }
  }

  return lines.join('\n')
}

function pickBestAttempt(current, candidate) {
  if (!current) {
    return candidate
  }

  if (candidate.acceptedItems.length !== current.acceptedItems.length) {
    return candidate.acceptedItems.length > current.acceptedItems.length ? candidate : current
  }

  if (candidate.distinctHostCount !== current.distinctHostCount) {
    return candidate.distinctHostCount > current.distinctHostCount ? candidate : current
  }

  if (candidate.rawItems.length !== current.rawItems.length) {
    return candidate.rawItems.length > current.rawItems.length ? candidate : current
  }

  return candidate
}


module.exports = {
  DEFAULT_EXCLUDED_DOMAINS,
  normalizeInt,
  normalizeText,
  normalizeDomainList,
  buildDefaultQuery,
  buildFallbackQueries,
  buildSearchQuery,
  buildSearchURL,
  splitSnippet,
  evaluateNewsItem,
  formatReport,
  pickBestAttempt,
};