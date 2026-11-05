module.exports.run = async ({ launch, connect, selector, params, artifact, log }) => {
  const automation = arguments[0] || {}
  const openPage = automation.openPage
  const grantPermissions = automation.grantPermissions
  const normalizeText = (value) => String(value == null ? '' : value).trim()
  const normalizeLineText = (value) =>
    String(value == null ? '' : value)
      .replace(/\r/g, '')
      .split('\n')
      .map((line) => line.replace(/\s+/g, ' ').trim())
      .filter(Boolean)
      .join('\n')
  const normalizeInt = (value, fallback, min, max) => {
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
  const normalizeBool = (value, fallback) => {
    if (value === undefined || value === null) {
      return fallback
    }
    if (typeof value === 'boolean') {
      return value
    }
    const normalized = String(value).trim().toLowerCase()
    if (!normalized) {
      return fallback
    }
    if (['1', 'true', 'yes', 'on'].includes(normalized)) {
      return true
    }
    if (['0', 'false', 'no', 'off'].includes(normalized)) {
      return false
    }
    return fallback
  }
  const escapeRegExp = (value) => String(value == null ? '' : value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const splitLines = (value) =>
    normalizeLineText(value)
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)
  const uniqueLines = (...values) => {
    const result = []
    const seen = new Set()
    for (const value of values) {
      for (const line of splitLines(value)) {
        if (seen.has(line)) {
          continue
        }
        seen.add(line)
        result.push(line)
      }
    }
    return result
  }
  const splitQueryTerms = (value) => {
    const rawItems = Array.isArray(value) ? value : [value]
    const result = []
    const seen = new Set()
    for (const rawItem of rawItems) {
      const normalized = normalizeText(rawItem)
      if (!normalized) {
        continue
      }
      for (const item of normalized.split(/[,，;；|\n]+/g)) {
        const term = normalizeText(item)
        if (!term) {
          continue
        }
        const key = term.toLowerCase()
        if (seen.has(key)) {
          continue
        }
        seen.add(key)
        result.push(term)
      }
    }
    return result
  }
  const joinQueryTerms = (terms) => splitQueryTerms(terms).join(', ')
  const firstQueryParam = (...values) => {
    for (const value of values) {
      const terms = splitQueryTerms(value)
      if (terms.length > 0) {
        return terms.join(', ')
      }
    }
    return ''
  }
  const uniqueQueryTerms = (...groups) => {
    const result = []
    const seen = new Set()
    for (const group of groups) {
      for (const term of splitQueryTerms(group)) {
        const key = term.toLowerCase()
        if (seen.has(key)) {
          continue
        }
        seen.add(key)
        result.push(term)
      }
    }
    return result
  }
  const extractEmailAddress = (value) => {
    const match = String(value == null ? '' : value).match(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i)
    return match ? match[0] : ''
  }
  const buildMailboxInfo = (rawValue, labels) => {
    const raw = normalizeLineText(rawValue)
    if (!raw) {
      return {
        raw: '',
        name: '',
        email: '',
      }
    }

    const valueText = stripLabeledLine(raw.replace(/\n+/g, ' '), labels)
    const email = extractEmailAddress(valueText)
    let name = valueText
    if (email) {
      name = name
        .replace(new RegExp('<\\s*' + escapeRegExp(email) + '\\s*>', 'i'), ' ')
        .replace(new RegExp('\\(\\s*' + escapeRegExp(email) + '\\s*\\)', 'i'), ' ')
        .replace(new RegExp(escapeRegExp(email), 'i'), ' ')
    }
    name = splitLines(name.replace(/[<>()\[\]]/g, ' '))
      .filter((line) => {
        const lowered = line.toLowerCase()
        if (!line) {
          return false
        }
        if (labels.some((label) => lowered === label.toLowerCase())) {
          return false
        }
        if (extractEmailAddress(line)) {
          return false
        }
        return true
      })
      .slice(0, 2)
      .join(' ')

    return {
      raw,
      name: normalizeText(name),
      email,
    }
  }
  const stripLabeledLine = (line, labels) => {
    let normalized = normalizeText(line)
    for (const label of labels) {
      normalized = normalized.replace(new RegExp('^' + escapeRegExp(label) + '\\s*[:：]?\\s*', 'i'), '')
    }
    return normalized
  }
  const parseMailboxLine = (line, labels) => {
    const normalizedLine = normalizeText(line)
    if (!normalizedLine) {
      return null
    }
    const lowered = normalizedLine.toLowerCase()
    const matchedLabel = labels.some((label) => {
      const normalizedLabel = label.toLowerCase()
      return (
        lowered === normalizedLabel ||
        lowered.startsWith(normalizedLabel + ' ') ||
        lowered.startsWith(normalizedLabel + ':') ||
        lowered.startsWith(normalizedLabel + '：')
      )
    })
    if (!matchedLabel) {
      return null
    }
    const parsed = buildMailboxInfo(normalizedLine, labels)
    if (!parsed.email && !parsed.name) {
      return null
    }
    return parsed
  }
  const extractMailbox = (values, labels) => {
    const lines = uniqueLines(...values)
    for (let index = 0; index < lines.length; index += 1) {
      const line = lines[index]
      const parsed = parseMailboxLine(line, labels)
      if (parsed) {
        return parsed
      }
      const lowered = normalizeText(line).toLowerCase()
      const isLabelOnly = labels.some((label) => lowered === label.toLowerCase())
      if (!isLabelOnly) {
        continue
      }
      let nameOnlyParsed = null
      for (let offset = 1; offset <= 6 && index + offset < lines.length; offset += 1) {
        const combined = lines.slice(index, index + offset + 1).join('\n')
        const combinedParsed = buildMailboxInfo(combined, labels)
        if (combinedParsed.email) {
          return combinedParsed
        }
        if (!nameOnlyParsed && combinedParsed.name) {
          nameOnlyParsed = combinedParsed
        }
      }
      if (nameOnlyParsed) {
        return nameOnlyParsed
      }
    }
    return {
      raw: '',
      name: '',
      email: '',
    }
  }
  const extractVerificationCode = (...values) => {
    const lines = uniqueLines(...values)
    const sixDigitPattern = /(^|[^\d])([0-9]{6})(?!\d)/g
    const keywordPattern =
      /(验证码|校验码|临时验证码|动态码|一次性密码|verification code|verification|security code|passcode|one-time password|temporary code|log-?in code|login code|otp|code)/i
    const noisyLinePattern =
      /(\b\d+(?:\.\d+)?\s*(kb|mb|gb)\b|附件|attachment|embedded image|下载所有附件|星期|上午|下午|\d{4}年\d{1,2}月\d{1,2}日|\b20\d{2}[-/]\d{1,2}[-/]\d{1,2}\b|\b\d{1,2}:\d{2}\b)/i
    const pickSixDigitCode = (line) => {
      const normalized = normalizeText(line)
      if (!normalized || (noisyLinePattern.test(normalized) && !keywordPattern.test(normalized))) {
        return ''
      }
      const matches = Array.from(normalized.matchAll(sixDigitPattern), (match) => match[2]).filter(Boolean)
      if (matches.length === 0) {
        return ''
      }
      return matches[0]
    }
    const pickCode = (line) => {
      const matches = Array.from(normalizeText(line).matchAll(sixDigitPattern), (match) => match[2]).filter(Boolean)
      if (!matches || matches.length === 0) {
        return ''
      }
      for (const candidate of matches) {
        if (!candidate) {
          continue
        }
        if (/^20\d{2}$/.test(candidate) && noisyLinePattern.test(line)) {
          continue
        }
        if (noisyLinePattern.test(line) && !keywordPattern.test(line)) {
          continue
        }
        return candidate
      }
      return ''
    }

    for (let index = 0; index < lines.length; index += 1) {
      if (!keywordPattern.test(lines[index])) {
        continue
      }
      const nearby = [lines[index], lines[index + 1] || '', lines[index + 2] || '', lines[index - 1] || '']
      for (const candidate of nearby) {
        const code = pickCode(candidate)
        if (code) {
          return code
        }
      }
    }

    for (const line of lines) {
      const normalized = normalizeText(line)
      if (/^\d{6}$/.test(normalized)) {
        return normalized
      }
    }

    for (const line of lines) {
      const code = pickSixDigitCode(line)
      if (code) {
        return code
      }
    }

    return ''
  }
  const extractSignature = (value) => {
    const lines = splitLines(value)
    if (lines.length === 0) {
      return ''
    }

    const ignorePattern =
      /(验证码|校验码|临时验证码|一次性密码|verification code|otp|如果并非你本人|请忽略此电子邮件|ignore this email|do not share|不要分享|privacy|terms|unsubscribe|attachment|附件|embedded image|\b\d+(?:\.\d+)?\s*(kb|mb|gb)\b)/i

    for (let index = lines.length - 1; index >= 0; index -= 1) {
      const line = lines[index]
      if (line.length > 80 || /\d{4,8}/.test(line) || ignorePattern.test(line)) {
        continue
      }
      if (/^(thanks|regards|best|cheers|sincerely|谢谢|此致|敬上)/i.test(line)) {
        const nextLine = lines[index + 1] || ''
        if (nextLine && nextLine.length <= 60 && !ignorePattern.test(nextLine)) {
          return [line, nextLine].join('\n')
        }
        return line
      }
    }

    for (let index = lines.length - 1; index >= 0; index -= 1) {
      const line = lines[index]
      if (line.length === 0 || line.length > 60 || /\d{4,8}/.test(line) || ignorePattern.test(line)) {
        continue
      }
      if (!/[A-Za-z\u4e00-\u9fff]/.test(line)) {
        continue
      }
      return line
    }
    return ''
  }
  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

  const includesNormalized = (haystack, needle) => {
    const terms = splitQueryTerms(needle)
    if (terms.length === 0) {
      return false
    }
    const normalizedHaystack = normalizeLineText(haystack).toLowerCase()
    return terms.some((term) => normalizedHaystack.includes(normalizeLineText(term).toLowerCase()))
  }
  const matchesMailboxInfo = (info, query) => {
    const terms = splitQueryTerms(query)
    if (terms.length === 0) {
      return true
    }
    return terms.some(
      (term) =>
        includesNormalized(info.raw, term) ||
        includesNormalized(info.name, term) ||
        includesNormalized(info.email, term),
    )
  }
  const formatQueryMismatch = (label, value) => label + '未匹配：' + joinQueryTerms(value)
  const matchMailAgainstFilters = (mail, senderInfo, recipientInfo, filters) => {
    const mismatches = []
    if (
      filters.searchQuery &&
      !includesNormalized(mail.subject, filters.searchQuery) &&
      !includesNormalized(mail.headerText, filters.searchQuery) &&
      !includesNormalized(mail.contentText, filters.searchQuery)
    ) {
      mismatches.push(formatQueryMismatch('搜索词', filters.searchQuery))
    }
    if (
      filters.subjectQuery &&
      !includesNormalized(mail.subject, filters.subjectQuery) &&
      !includesNormalized(mail.contentText, filters.subjectQuery)
    ) {
      mismatches.push(formatQueryMismatch('主题', filters.subjectQuery))
    }
    if (
      filters.senderEmail &&
      !includesNormalized(senderInfo.email, filters.senderEmail) &&
      !includesNormalized(mail.headerText, filters.senderEmail) &&
      !includesNormalized(mail.contentText, filters.senderEmail)
    ) {
      mismatches.push(formatQueryMismatch('发件邮箱', filters.senderEmail))
    }
    if (
      filters.recipientQuery &&
      !matchesMailboxInfo(recipientInfo, filters.recipientQuery) &&
      !includesNormalized(mail.headerText, filters.recipientQuery) &&
      !includesNormalized(mail.contentText, filters.recipientQuery)
    ) {
      mismatches.push(formatQueryMismatch('收件人', filters.recipientQuery))
    }
    return {
      ok: mismatches.length === 0,
      mismatches,
    }
  }
  const recipientQuery = firstQueryParam(params.recipient, params.recipientQuery, params.receiver)
  const subjectQuery = firstQueryParam(params.subjectQuery, params.subject, params.mailSubject, params.title)
  const senderEmail = firstQueryParam(params.senderEmail, params.fromEmail)
  const explicitSearchQuery = firstQueryParam(params.searchQuery, params.search, params.query)
  const maxKeywordCandidates = 2
  const maxSenderCandidates = 2
  const maxStructuredSearchAttempts = 4
  const senderFieldQueries = uniqueQueryTerms(senderEmail).slice(0, maxSenderCandidates)
  const recipientFieldValue = normalizeText(recipientQuery)
  const searchQuery = explicitSearchQuery || subjectQuery
  const searchQueries = uniqueQueryTerms(searchQuery).slice(0, maxKeywordCandidates)
  if (searchQueries.length === 0 && !subjectQuery && !senderEmail && !recipientQuery) {
    throw new Error('至少需要提供 recipient / subjectQuery / senderEmail / searchQuery 之一')
  }

  const requestedInboxUrl = normalizeText(params.inboxUrl)
  const defaultInboxUrl = 'https://mail.proton.me/u/0/inbox'
  const timeoutMs = normalizeInt(params.timeoutMs, 90000, 5000, 300000)
  const waitAfterLoadMs = normalizeInt(params.waitAfterLoadMs, 0, 0, 10000)
  const waitAfterSearchMs = normalizeInt(params.waitAfterSearchMs, 0, 0, 15000)
  const waitAfterOpenMs = normalizeInt(params.waitAfterOpenMs, 0, 0, 15000)
  const searchResultTimeoutMs = normalizeInt(params.searchResultTimeoutMs, 12000, 0, 30000)
  const searchSettleMs = normalizeInt(params.searchSettleMs, 900, 0, 5000)
  const maxSearchPasses = normalizeInt(params.maxSearchPasses, 2, 1, 3)
  const firstAttemptEmptyRetryDelayMs = normalizeInt(params.firstAttemptEmptyRetryDelayMs, 10000, 0, 60000)
  const openMailTimeoutMs = normalizeInt(params.openMailTimeoutMs, 3000, 0, 15000)
  const maxBodyChars = normalizeInt(params.maxBodyChars, 12000, 500, 50000)
  const maxCandidateChecks = normalizeInt(params.maxCandidateChecks, 5, 1, 20)
  const preferLatest = normalizeBool(params.preferLatest, true)
  const allowOpenedMailShortcut = normalizeBool(params.allowOpenedMailShortcut, false)
  const captureScreenshot = normalizeBool(params.captureScreenshot, false)
  const matchFilters = {
    searchQuery,
    recipientQuery,
    subjectQuery,
    senderEmail,
  }
  const buildAttemptMatchFilters = (attempt) => ({
    searchQuery: normalizeText(attempt && attempt.keywordQuery),
    recipientQuery: normalizeText(attempt && attempt.recipientFieldValue),
    subjectQuery: '',
    senderEmail: normalizeText(attempt && attempt.senderFieldValue),
  })
  const buildSearchAttempts = () => {
    const attempts = []
    const seen = new Set()
    const addAttempt = (label, keywordQuery, senderFieldValue, recipientFieldValue) => {
      const normalizedAttempt = {
        label: normalizeText(label) || 'search',
        keywordQuery: normalizeText(keywordQuery),
        senderFieldValue: normalizeText(senderFieldValue),
        recipientFieldValue: normalizeText(recipientFieldValue),
      }
      if (
        !normalizedAttempt.keywordQuery &&
        !normalizedAttempt.senderFieldValue &&
        !normalizedAttempt.recipientFieldValue
      ) {
        return
      }
      const dedupeKey = JSON.stringify([
        normalizedAttempt.keywordQuery.toLowerCase(),
        normalizedAttempt.senderFieldValue.toLowerCase(),
        normalizedAttempt.recipientFieldValue.toLowerCase(),
      ])
      if (seen.has(dedupeKey)) {
        return
      }
      seen.add(dedupeKey)
      attempts.push(normalizedAttempt)
    }

    const keywordVariants = searchQueries.length > 0 ? searchQueries : ['']
    const senderVariants = senderFieldQueries.length > 0 ? senderFieldQueries : ['']
    for (const term of keywordVariants) {
      for (const senderFieldValue of senderVariants) {
        addAttempt('structured-primary', term, senderFieldValue, recipientFieldValue)
        if (attempts.length >= maxStructuredSearchAttempts) {
          return attempts
        }
      }
    }
    return attempts
  }
  const searchAttempts = buildSearchAttempts()
  log(
    'searchAttempts',
    searchAttempts.map((attempt) => ({
      label: attempt.label,
      keywordQuery: attempt.keywordQuery,
      senderFieldValue: attempt.senderFieldValue,
      recipientFieldValue: attempt.recipientFieldValue,
    })),
  )

  const session = await launch({
    skipDefaultStartUrls: true,
    startUrls: [],
  })
  const connection = await connect(session, { timeoutMs })
  const activeBrowser = connection.browser
  if (!activeBrowser) {
    throw new Error('browser connection is unavailable')
  }

  const context =
    connection.context ||
    activeBrowser.contexts()[0] ||
    (typeof activeBrowser.newContext === 'function' ? await activeBrowser.newContext() : null)
  if (!context) {
    throw new Error('browser context is unavailable')
  }

  const listOpenPages = () => {
    const seen = new Set()
    return [connection.page, ...context.pages()].filter((candidate) => {
      if (!candidate || candidate.isClosed() || seen.has(candidate)) {
        return false
      }
      seen.add(candidate)
      return true
    })
  }

  const inspectOpenPage = async (candidate, index) => {
    const url = normalizeText(candidate && !candidate.isClosed() ? candidate.url() : '')
    const runtimeState = await Promise.resolve()
      .then(() =>
        candidate.evaluate(() => ({
          visibilityState: document.visibilityState || '',
          hasFocus: typeof document.hasFocus === 'function' ? document.hasFocus() : false,
          title: document.title || '',
        })),
      )
      .catch(() => ({
        visibilityState: '',
        hasFocus: false,
        title: '',
      }))
    return {
      candidate,
      index,
      url,
      derivedInboxUrl: deriveInboxUrlFromPageUrl(url),
      visibilityState: normalizeText(runtimeState.visibilityState).toLowerCase(),
      hasFocus: !!runtimeState.hasFocus,
      title: normalizeText(runtimeState.title),
      isConnectionPage: candidate === connection.page,
    }
  }

  const scoreOpenPage = (pageState) => {
    if (!pageState || !pageState.candidate || pageState.candidate.isClosed()) {
      return Number.NEGATIVE_INFINITY
    }
    let score = 0
    if (/https:\/\/mail\.proton\.me\//i.test(pageState.url)) {
      score += 420
    } else if (/mail\.proton\.me/i.test(pageState.url)) {
      score += 320
    }
    if (pageState.hasFocus) {
      score += 260
    }
    if (pageState.visibilityState === 'visible') {
      score += 220
    }
    if (pageState.isConnectionPage) {
      score += 140
    }
    if (requestedInboxUrl && pageState.derivedInboxUrl && pageState.derivedInboxUrl === requestedInboxUrl) {
      score += 180
    }
    if (/\/u\/\d+(?:\/|$)/i.test(pageState.url)) {
      score += 80
    }
    if (/#(?:keyword|from|to)=/i.test(pageState.url)) {
      score += 40
    }
    if (/proton mail/i.test(pageState.title)) {
      score += 20
    }
    score += pageState.index
    return score
  }

  const pickPreferredPage = async () => {
    const pages = listOpenPages()
    if (pages.length === 0) {
      return null
    }
    const pageStates = await Promise.all(pages.map((candidate, index) => inspectOpenPage(candidate, index)))
    pageStates.sort((left, right) => {
      const scoreDiff = scoreOpenPage(right) - scoreOpenPage(left)
      if (scoreDiff !== 0) {
        return scoreDiff
      }
      return right.index - left.index
    })
    return pageStates[0] ? pageStates[0].candidate : null
  }

  const deriveInboxUrlFromPageUrl = (pageUrl) => {
    const normalizedPageUrl = normalizeText(pageUrl)
    if (!normalizedPageUrl) {
      return ''
    }
    try {
      const parsed = new URL(normalizedPageUrl)
      if (!/mail\.proton\.me$/i.test(parsed.hostname)) {
        return ''
      }
      const mailboxMatch = parsed.pathname.match(/^\/u\/(\d+)(?:\/|$)/i)
      if (!mailboxMatch) {
        return parsed.origin + '/u/0/inbox'
      }
      return parsed.origin + '/u/' + mailboxMatch[1] + '/inbox'
    } catch {
      return ''
    }
  }

  const parseProtonMailboxRoute = (pageUrl) => {
    const normalizedPageUrl = normalizeText(pageUrl)
    if (!normalizedPageUrl) {
      return null
    }
    try {
      const parsed = new URL(normalizedPageUrl)
      if (!/mail\.proton\.me$/i.test(parsed.hostname)) {
        return null
      }
      const segments = parsed.pathname.split('/').filter(Boolean)
      if (segments[0] !== 'u' || !/^\d+$/.test(segments[1] || '')) {
        return null
      }
      return {
        origin: parsed.origin,
        accountIndex: segments[1],
        folder: segments[2] || '',
        extraSegments: segments.slice(3),
      }
    } catch {
      return null
    }
  }
  const isProtonMailboxUrl = (pageUrl) => !!parseProtonMailboxRoute(pageUrl)
  const isProtonInboxUrl = (pageUrl) => {
    const route = parseProtonMailboxRoute(pageUrl)
    return !!route && route.folder === 'inbox' && route.extraSegments.length === 0
  }
  const isProtonMessageDetailUrl = (pageUrl) => {
    const route = parseProtonMailboxRoute(pageUrl)
    return !!route && !!route.folder && route.extraSegments.length > 0
  }

  const attachedPage = await pickPreferredPage()
  let page = attachedPage && !attachedPage.isClosed() ? attachedPage : null
  const preferredPageUrl = normalizeText(page && !page.isClosed() ? page.url() : '')
  const preferredPageInboxUrl = deriveInboxUrlFromPageUrl(preferredPageUrl)
  const isAttachedMailboxPage = isProtonMailboxUrl(preferredPageUrl)
  const isAttachedInboxPage = isProtonInboxUrl(preferredPageUrl)
  const isAttachedMessageDetailPage = isProtonMessageDetailUrl(preferredPageUrl)
  const inboxUrl = (isAttachedMailboxPage && preferredPageInboxUrl) || requestedInboxUrl || preferredPageInboxUrl || defaultInboxUrl
  const shouldOpenInboxPage = !isAttachedMailboxPage

  const waitForPageSignal = async (pageFunction, arg, waitTimeoutMs) =>
    page
      .waitForFunction(pageFunction, arg, { timeout: waitTimeoutMs })
      .then((handle) => handle.jsonValue())
      .catch(() => null)

  const ensureSettingsDrawerProbe = async () =>
    page
      .evaluate(() => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width > 0 && rect.height > 0
        }
        const describeElement = (element) => {
          if (!(element instanceof Element)) {
            return ''
          }
          const parts = [element.tagName ? element.tagName.toLowerCase() : '']
          const dataTestId = normalize(element.getAttribute('data-testid') || '')
          const ariaControls = normalize(element.getAttribute('aria-controls') || '')
          const title = normalize(element.getAttribute('title') || '')
          const ariaLabel = normalize(element.getAttribute('aria-label') || '')
          const text = normalize(element.textContent || '').slice(0, 120)
          if (dataTestId) {
            parts.push('data-testid=' + dataTestId)
          }
          if (ariaControls) {
            parts.push('aria-controls=' + ariaControls)
          }
          if (title) {
            parts.push('title=' + title)
          }
          if (ariaLabel) {
            parts.push('aria-label=' + ariaLabel)
          }
          if (text) {
            parts.push('text=' + text)
          }
          return parts.filter(Boolean).join(' | ')
        }
        const probeKey = '__antSettingsDrawerProbe'
        const existing = window[probeKey]
        if (existing && existing.__installed) {
          return {
            installed: true,
            eventCount: Array.isArray(existing.events) ? existing.events.length : 0,
            isOpen: !!existing.isOpen,
          }
        }

        const probe = existing && typeof existing === 'object' ? existing : {}
        probe.__installed = true
        probe.installedAt = new Date().toISOString()
        probe.lastAction = normalize(probe.lastAction || '')
        probe.lastActionMeta = probe.lastActionMeta && typeof probe.lastActionMeta === 'object' ? probe.lastActionMeta : {}
        probe.events = Array.isArray(probe.events) ? probe.events : []
        probe.recentClicks = Array.isArray(probe.recentClicks) ? probe.recentClicks : []
        probe.recentKeys = Array.isArray(probe.recentKeys) ? probe.recentKeys : []
        probe.recentFocus = Array.isArray(probe.recentFocus) ? probe.recentFocus : []
        probe.isOpen = false

        const trimList = (items, limit) => {
          while (items.length > limit) {
            items.shift()
          }
        }
        const pushEvent = (type, payload) => {
          probe.events.push({
            index: probe.events.length,
            time: new Date().toISOString(),
            type: normalize(type),
            lastAction: normalize(probe.lastAction || ''),
            lastActionMeta: probe.lastActionMeta && typeof probe.lastActionMeta === 'object' ? probe.lastActionMeta : {},
            href: window.location.href || '',
            ...(payload && typeof payload === 'object' ? payload : {}),
          })
          trimList(probe.events, 240)
        }
        const readDrawerState = () => {
          const drawer = document.querySelector('#drawer-app-proton-settings')
          const drawerButton = document.querySelector(
            '[data-testid="settings-drawer-app-button:settings-icon"], [aria-controls="drawer-app-proton-settings"]',
          )
          return {
            isOpen:
              (drawer instanceof HTMLElement && isVisible(drawer)) ||
              (!!drawerButton &&
                document.body instanceof HTMLBodyElement &&
                /\bdrawer-is-open\b/.test(normalize(document.body.className || ''))),
            drawer: describeElement(drawer),
            drawerButton: describeElement(drawerButton),
            bodyClass: normalize(document.body ? document.body.className || '' : ''),
            activeElement: describeElement(document.activeElement),
          }
        }
        const syncDrawerState = (reason) => {
          const snapshot = readDrawerState()
          if (!!snapshot.isOpen !== !!probe.isOpen) {
            probe.isOpen = !!snapshot.isOpen
            pushEvent(probe.isOpen ? 'drawer-open' : 'drawer-close', {
              reason: normalize(reason),
              drawer: snapshot.drawer,
              drawerButton: snapshot.drawerButton,
              bodyClass: snapshot.bodyClass,
              activeElement: snapshot.activeElement,
              recentClicks: probe.recentClicks.slice(-6),
              recentKeys: probe.recentKeys.slice(-6),
              recentFocus: probe.recentFocus.slice(-6),
            })
            return
          }
          probe.isOpen = !!snapshot.isOpen
        }
        const scheduleSync = (() => {
          let pending = false
          return (reason) => {
            if (pending) {
              return
            }
            pending = true
            Promise.resolve().then(() => {
              pending = false
              syncDrawerState(reason)
            })
          }
        })()

        probe.markAction = (label, meta) => {
          probe.lastAction = normalize(label)
          probe.lastActionMeta = meta && typeof meta === 'object' ? meta : {}
          pushEvent('action', {
            action: probe.lastAction,
            meta: probe.lastActionMeta,
          })
        }

        document.addEventListener(
          'click',
          (event) => {
            const target = event.target instanceof Element ? event.target : null
            probe.recentClicks.push({
              time: new Date().toISOString(),
              target: describeElement(target),
              x: Number.isFinite(event.clientX) ? event.clientX : 0,
              y: Number.isFinite(event.clientY) ? event.clientY : 0,
            })
            trimList(probe.recentClicks, 24)
            scheduleSync('click')
          },
          true,
        )
        document.addEventListener(
          'keydown',
          (event) => {
            probe.recentKeys.push({
              time: new Date().toISOString(),
              key: normalize(event.key),
              code: normalize(event.code),
              ctrlKey: !!event.ctrlKey,
              altKey: !!event.altKey,
              shiftKey: !!event.shiftKey,
              metaKey: !!event.metaKey,
              target: describeElement(event.target instanceof Element ? event.target : null),
            })
            trimList(probe.recentKeys, 24)
            scheduleSync('keydown')
          },
          true,
        )
        document.addEventListener(
          'focusin',
          (event) => {
            probe.recentFocus.push({
              time: new Date().toISOString(),
              target: describeElement(event.target instanceof Element ? event.target : null),
            })
            trimList(probe.recentFocus, 24)
            scheduleSync('focusin')
          },
          true,
        )
        window.addEventListener('hashchange', () => scheduleSync('hashchange'), true)
        window.addEventListener('popstate', () => scheduleSync('popstate'), true)
        const observer = new MutationObserver(() => scheduleSync('mutation'))
        observer.observe(document.documentElement, {
          attributes: true,
          childList: true,
          subtree: true,
          attributeFilter: ['class', 'style', 'hidden', 'aria-hidden', 'aria-expanded'],
        })
        probe.__observer = observer
        window[probeKey] = probe
        syncDrawerState('install')
        return {
          installed: true,
          eventCount: probe.events.length,
          isOpen: !!probe.isOpen,
        }
      })
      .catch(() => ({ installed: false, eventCount: 0, isOpen: false }))

  const markSettingsDrawerProbeAction = async (action, meta = {}) =>
    page
      .evaluate(({ action, meta }) => {
        const probe = window.__antSettingsDrawerProbe
        if (!probe || typeof probe.markAction !== 'function') {
          return false
        }
        probe.markAction(String(action || ''), meta && typeof meta === 'object' ? meta : {})
        return true
      }, { action, meta })
      .catch(() => false)

  let settingsDrawerProbeCursor = 0
  const flushSettingsDrawerProbe = async (stage, extra = {}) => {
    const state = await page
      .evaluate(() => {
        const probe = window.__antSettingsDrawerProbe
        if (!probe) {
          return null
        }
        return {
          isOpen: !!probe.isOpen,
          lastAction: String(probe.lastAction || ''),
          eventCount: Array.isArray(probe.events) ? probe.events.length : 0,
          events: Array.isArray(probe.events) ? probe.events : [],
        }
      })
      .catch(() => null)
    if (!state) {
      return
    }
    const newEvents = state.events.slice(settingsDrawerProbeCursor)
    settingsDrawerProbeCursor = state.events.length
    if (newEvents.length === 0 && !extra.force) {
      return
    }
    log('settingsDrawerProbe', {
      stage,
      isOpen: state.isOpen,
      lastAction: state.lastAction,
      eventCount: state.eventCount,
      events: newEvents,
      ...extra,
    })
  }

  const safeDismissButtons = async (labels, timeout = 150) => {
    for (const label of labels) {
      const locator = page.getByRole('button', { name: label, exact: false })
      const count = Math.min(await locator.count().catch(() => 0), 8)
      for (let index = 0; index < count; index += 1) {
        const candidate = locator.nth(index)
        const candidateState = await candidate
          .evaluate((element, expectedLabel) => {
            const normalize = (value) =>
              String(value || '')
                .replace(/\s+/g, ' ')
                .trim()
            if (!(element instanceof HTMLElement)) {
              return null
            }
            const style = window.getComputedStyle(element)
            if (!style || style.visibility === 'hidden' || style.display === 'none') {
              return null
            }
            const rect = element.getBoundingClientRect()
            if (rect.width < 18 || rect.height < 18 || rect.bottom < 0 || rect.top > window.innerHeight) {
              return null
            }
            const dataTestId = normalize(element.getAttribute('data-testid') || '')
            const ariaControls = normalize(element.getAttribute('aria-controls') || '')
            const names = [
              normalize(element.innerText || element.textContent || ''),
              normalize(element.getAttribute('aria-label') || ''),
              normalize(element.getAttribute('title') || ''),
            ].filter(Boolean)
            const exactLabel = normalize(expectedLabel)
            return {
              blocked:
                ariaControls === 'drawer-app-proton-settings' || dataTestId === 'settings-drawer-app-button:settings-icon',
              names,
              exactMatched: names.some((name) => name === exactLabel),
            }
          }, label)
          .catch(() => null)
        if (!candidateState || candidateState.blocked || !candidateState.exactMatched) {
          continue
        }
        try {
          await markSettingsDrawerProbeAction('safe-dismiss-button', {
            label,
            matchedNames: candidateState.names,
          })
          await candidate.click({ timeout })
          await candidate.waitFor({ state: 'hidden', timeout: 300 }).catch(() => {})
          break
        } catch {}
      }
    }
  }

  const stabilizeMessageListSurface = async () => {
    await markSettingsDrawerProbeAction('stabilize-surface:start')
    await safeDismissButtons(['关闭', '知道了', '稍后', '以后再说', 'Not now', 'Maybe later', 'Skip'], 120)
    await flushSettingsDrawerProbe('stabilize-before-escape')
    await markSettingsDrawerProbeAction('stabilize-surface:escape')
    await page.keyboard.press('Escape').catch(() => {})
    await sleep(80)
    await safeDismissButtons(['关闭', '知道了', '稍后', '以后再说', 'Not now', 'Maybe later', 'Skip'], 120)
    await flushSettingsDrawerProbe('stabilize-after-escape')
  }

  const waitForMailboxShell = async () => {
    return waitForPageSignal(
      () => {
        const text = String(document.body ? document.body.innerText || document.body.textContent || '' : '')
          .replace(/\s+/g, ' ')
          .trim()
        const hasSearchElement = !!document.querySelector(
          '[data-testid="search-keyword"], form[name="advanced-search"], [role="searchbox"], input[type="search"]',
        )
        if (hasSearchElement) {
          return {
            text,
            hasSearchElement,
          }
        }
        if (text && !/^Loading Proton Mail(?: Loading)?$/i.test(text)) {
          return {
            text,
            hasSearchElement,
          }
        }
        return false
      },
      undefined,
      Math.min(timeoutMs, 12000),
    )
  }

  const detectProtonLoginPage = async () =>
    page
      .evaluate(() => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const title = document.title || ''
        const url = window.location.href || ''
        const bodyPreview = normalize(document.body ? document.body.innerText || document.body.textContent || '' : '').slice(0, 400)
        const hasPasswordInput = !!document.querySelector('input[type="password"]')
        const hasLoginAction = Array.from(document.querySelectorAll('button, [role="button"], a'))
          .map((element) => normalize(element.textContent || element.getAttribute('aria-label') || element.getAttribute('title') || ''))
          .some((text) => /^(登录|login)$/i.test(text))
        const isLoginUrl = /(?:account|mail)\.proton\.me\/login(?:[#/?]|$)/i.test(url) || /account\.proton\.me\/mail/i.test(url)
        const isLoginTitle = /(登录|login)/i.test(title)
        return {
          isLoginPage: isLoginUrl || (hasPasswordInput && (isLoginTitle || hasLoginAction)),
          title,
          url,
          bodyPreview,
        }
      })
      .catch(() => ({
        isLoginPage: false,
        title: '',
        url: '',
        bodyPreview: '',
      }))

  const detectProtonSessionBootstrapPage = async () =>
    page
      .evaluate(() => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const title = document.title || ''
        const url = window.location.href || ''
        const bodyPreview = normalize(document.body ? document.body.innerText || document.body.textContent || '' : '').slice(0, 400)
        const hasPasswordInput = !!document.querySelector('input[type="password"]')
        const hasEmailInput = !!document.querySelector('input[type="email"]')
        const hasSearchElement = !!document.querySelector(
          '[data-testid="search-keyword"], form[name="advanced-search"], [role="searchbox"], input[type="search"]',
        )
        return {
          isBootstrapPage:
            /mail\.proton\.me\/login#selector=/i.test(url) &&
            /^Loading Proton Mail(?: Loading)?$/i.test(bodyPreview) &&
            !hasPasswordInput &&
            !hasEmailInput &&
            !hasSearchElement,
          title,
          url,
          bodyPreview,
        }
      })
      .catch(() => ({
        isBootstrapPage: false,
        title: '',
        url: '',
        bodyPreview: '',
      }))

  const waitForProtonSessionBootstrapRecovery = async () =>
    waitForPageSignal(
      () => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const title = document.title || ''
        const url = window.location.href || ''
        const bodyPreview = normalize(document.body ? document.body.innerText || document.body.textContent || '' : '').slice(0, 400)
        const hasPasswordInput = !!document.querySelector('input[type="password"]')
        const hasEmailInput = !!document.querySelector('input[type="email"]')
        const hasSearchElement = !!document.querySelector(
          '[data-testid="search-keyword"], form[name="advanced-search"], [role="searchbox"], input[type="search"]',
        )
        const isLoadingBody = /^Loading Proton Mail(?: Loading)?$/i.test(bodyPreview)
        const isSelectorLoginUrl = /mail\.proton\.me\/login#selector=/i.test(url)
        if (hasSearchElement) {
          return {
            state: 'mailbox-ready',
            title,
            url,
            bodyPreview,
          }
        }
        if (hasPasswordInput || hasEmailInput) {
          return {
            state: 'login-ready',
            title,
            url,
            bodyPreview,
          }
        }
        if (!isSelectorLoginUrl && bodyPreview && !isLoadingBody) {
          return {
            state: 'page-changed',
            title,
            url,
            bodyPreview,
          }
        }
        return false
      },
      undefined,
      Math.min(timeoutMs, 30000),
    )

  const extractCurrentOpenedMail = async () => {
    const baseMail = await page
      .evaluate(
        ({ maxChars }) => {
          const normalizeLines = (value) =>
            String(value || '')
              .replace(/\r/g, '')
              .split('\n')
              .map((line) => line.replace(/\s+/g, ' ').trim())
              .filter(Boolean)
              .join('\n')
          const visible = (el) => {
            if (!el) {
              return false
            }
            const style = window.getComputedStyle(el)
            if (!style || style.visibility === 'hidden' || style.display === 'none') {
              return false
            }
            const rect = el.getBoundingClientRect()
            return rect.width >= 240 && rect.height >= 100 && rect.bottom >= 60 && rect.top <= window.innerHeight
          }
          const roots = [
            ...Array.from(document.querySelectorAll('[data-testid*="message-view"], [data-shortcut-target="message-container"], .message-container.is-opened, article[role="article"], [role="article"]')),
          ].filter((element) => element instanceof HTMLElement && visible(element))
          const root = roots[0] || null
          if (!root) {
            return null
          }
          const rootText = normalizeLines(root.innerText || root.textContent || '')
          const iframeTexts = []
          const iframeSubjects = []
          const iframeElements = Array.from(root.querySelectorAll('iframe'))
          for (const frameElement of iframeElements) {
            if (!(frameElement instanceof HTMLIFrameElement) || !visible(frameElement)) {
              continue
            }
            const subject = normalizeLines(
              [
                frameElement.getAttribute('data-subject') || '',
                frameElement.getAttribute('aria-label') || '',
              ].join('\n'),
            )
            if (subject.length >= 4 && subject.length <= 240) {
              iframeSubjects.push(subject)
            }
            try {
              const frameDocument = frameElement.contentDocument
              const frameBody = frameDocument && frameDocument.body
              const frameText = normalizeLines(frameBody ? frameBody.innerText || frameBody.textContent || '' : '')
              if (frameText.length >= 6) {
                iframeTexts.push(frameText)
              }
            } catch {}
          }
          let subject = iframeSubjects[0] || ''
          if (!subject) {
            const heading = root.querySelector('h1, h2, h3')
            const headingText = normalizeLines(heading ? heading.textContent || '' : '')
            if (headingText.length >= 4 && headingText.length <= 240) {
              subject = headingText
            }
          }
          const headerText = rootText.split('\n').filter(Boolean).slice(0, 40).join('\n')
          const contentText = [...iframeTexts, rootText].filter(Boolean).join('\n').slice(0, maxChars)
          const lines = [rootText, ...iframeTexts].filter(Boolean).join('\n').split('\n').filter(Boolean)
          return {
            subject,
            contentText,
            metaText: lines.slice(0, 20).join('\n'),
            headerText,
          }
        },
        { maxChars: maxBodyChars },
      )
      .catch(() => null)
    if (!baseMail) {
      return null
    }

    const normalizeFrameText = (value) =>
      String(value || '')
        .replace(/\r/g, '')
        .split('\n')
        .map((line) => line.replace(/\s+/g, ' ').trim())
        .filter(Boolean)
        .join('\n')
    const frameTexts = []
    for (const frame of page.frames()) {
      if (frame === page.mainFrame()) {
        continue
      }
      try {
        const frameText = normalizeFrameText(await frame.locator('body').innerText({ timeout: 250 }))
        if (frameText.length >= 6) {
          frameTexts.push(frameText)
        }
      } catch {}
    }
    const contentText = Array.from(new Set([...frameTexts, baseMail.contentText].filter(Boolean))).join('\n').slice(0, maxBodyChars)
    const lines = [baseMail.headerText, contentText].filter(Boolean).join('\n').split('\n').filter(Boolean)
    return {
      ...baseMail,
      contentText,
      metaText: lines.slice(0, 20).join('\n') || baseMail.metaText,
      headerText: lines.slice(0, 40).join('\n') || baseMail.headerText,
    }
  }

  log('pageAttachTarget', {
    requestedInboxUrl,
    preferredPageUrl,
    preferredPageInboxUrl,
    isAttachedMailboxPage,
    isAttachedInboxPage,
    isAttachedMessageDetailPage,
    shouldOpenInboxPage,
    inboxUrl,
  })

  const notificationPermissionOrigin = 'https://mail.proton.me'
  if (shouldOpenInboxPage) {
    if (typeof openPage === 'function') {
      const opened = await openPage(connection, {
        url: inboxUrl,
        timeoutMs,
        waitUntil: 'domcontentloaded',
        permissions: ['notifications'],
        permissionOrigin: notificationPermissionOrigin,
      })
      page = opened && opened.page ? opened.page : page
      log('pageOpenResult', {
        strategy: 'openPage',
        permissionResult: opened && opened.permissionResult ? opened.permissionResult : null,
        reusedPage: !!(opened && opened.reusedPage),
        url: page && typeof page.url === 'function' ? page.url() : '',
      })
    } else {
      if (!page || page.isClosed()) {
        page = await context.newPage()
      }
      if (typeof grantPermissions === 'function') {
        const permissionResult = await grantPermissions(context, {
          origin: notificationPermissionOrigin,
          permissions: ['notifications'],
        })
        log('pageOpenResult', {
          strategy: 'fallback-grantPermissions',
          permissionResult,
          url: page && typeof page.url === 'function' ? page.url() : '',
        })
      }
      await page.goto(inboxUrl, { waitUntil: 'domcontentloaded', timeout: timeoutMs })
    }
  } else {
    await page.bringToFront().catch(() => {})
    if (isAttachedMessageDetailPage) {
      log('attachedMessageDetailPage', {
        url: preferredPageUrl,
        action: 'reuse-current-mailbox-and-search-directly-if-needed',
      })
    }
  }
  let mailboxShell = await waitForMailboxShell()
  if (!mailboxShell && waitAfterLoadMs > 0) {
    log('legacyLoadWaitFallback', { waitAfterLoadMs })
    await sleep(waitAfterLoadMs)
    mailboxShell = await waitForMailboxShell()
  }
  if (!mailboxShell) {
    const bootstrapState = await detectProtonSessionBootstrapPage()
    if (bootstrapState.isBootstrapPage) {
      log('protonSessionBootstrapDetected', bootstrapState)
      const bootstrapRecovery = await waitForProtonSessionBootstrapRecovery()
      log('protonSessionBootstrapRecovery', bootstrapRecovery || { state: 'timeout' })
      mailboxShell = await waitForMailboxShell()
    }
  }
  if (!mailboxShell) {
    await page.reload({ waitUntil: 'domcontentloaded', timeout: Math.min(timeoutMs, 15000) }).catch(() => {})
    mailboxShell = await waitForMailboxShell()
    if (mailboxShell) {
      log('mailboxShellRecoveredByReload', {
        url: page.url(),
      })
    }
  }
  const loginPageState = await detectProtonLoginPage()
  if (loginPageState.isLoginPage) {
    throw new Error(
      '当前实例未恢复 Proton 登录会话，页面停留在登录页: ' +
        JSON.stringify({
          title: loginPageState.title,
          url: loginPageState.url,
          bodyPreview: loginPageState.bodyPreview,
        }),
    )
  }

  await safeDismissButtons(['以后再说', '稍后再说', 'Not now', 'Maybe later', 'Skip'], 150)
  const drawerProbeInstallState = await ensureSettingsDrawerProbe()
  log('settingsDrawerProbeInstalled', drawerProbeInstallState)
  await flushSettingsDrawerProbe('probe-installed', { force: true })

  if (allowOpenedMailShortcut) {
    const openedMail = await extractCurrentOpenedMail()
    if (openedMail) {
      const openedSenderInfo = extractMailbox([openedMail.headerText, openedMail.metaText, openedMail.contentText], ['从', '发件人', 'from', 'sender'])
      const openedRecipientInfo = extractMailbox([openedMail.headerText, openedMail.metaText, openedMail.contentText], ['收件人', 'to', 'recipient'])
      const openedVerificationCode = extractVerificationCode(openedMail.contentText, openedMail.metaText, openedMail.headerText)
      const openedSignature = extractSignature(openedMail.contentText)
      const openedMatchReport = matchMailAgainstFilters(openedMail, openedSenderInfo, openedRecipientInfo, matchFilters)
      if (openedMatchReport.ok && openedVerificationCode) {
        return {
          ok: true,
          summary: '已返回当前打开邮件内容',
          verificationCode: openedVerificationCode,
          subject: openedMail.subject,
          mailboxName: openedSenderInfo.name || openedSenderInfo.email,
          senderName: openedSenderInfo.name,
          senderEmail: openedSenderInfo.email,
          recipientEmail: openedRecipientInfo.email,
          signature: openedSignature,
          checkedCandidateCount: 1,
        }
      }
    }
  }

  const messageRowSelectors = [
    '[data-testid*="message-item"]',
    '[data-testid*="conversation"]',
    '[role="row"]',
    'main li',
    '[data-testid*="message-row"]',
  ]

  const collapsedSearchEntryRootSelector = '[role="search"] [data-testid="input-root"]'
  const collapsedSearchReadonlyInputSelector = '[role="search"] input[data-testid="search-keyword"][readonly]'
  const searchSelectors = [
    'form[name="advanced-search"] input#search-keyword[data-testid="input-input-element"][data-shorcut-target="searchbox-field"]',
    'form[name="advanced-search"] input#search-keyword[data-testid="input-input-element"]',
    'form[name="advanced-search"] input#search-keyword[title="关键词"]',
  ]

  const isUsableSearchCandidate = async (locator) =>
    locator
      .evaluate((element) => {
        if (!(element instanceof HTMLElement)) {
          return false
        }
        const style = window.getComputedStyle(element)
        if (!style || style.visibility === 'hidden' || style.display === 'none') {
          return false
        }
        if (element instanceof HTMLInputElement && element.id === 'commander-search-input') {
          return false
        }
        const rect = element.getBoundingClientRect()
        return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
      })
      .catch(() => false)

  const findSearchInput = async () => {
    for (const selectorText of searchSelectors) {
      const locator = page.locator(selectorText)
      const count = Math.min(await locator.count().catch(() => 0), 12)
      for (let index = count - 1; index >= 0; index -= 1) {
        const candidate = locator.nth(index)
        try {
          if (await isUsableSearchCandidate(candidate)) {
            return candidate
          }
        } catch {}
        try {
          await candidate.waitFor({ state: 'attached', timeout: 300 })
          return candidate
        } catch {}
      }
    }
    return null
  }

  const waitForAdvancedSearchForm = async (waitTimeoutMs = Math.max(1200, Math.min(timeoutMs, 4000))) => {
    const signal = await waitForPageSignal(
      () => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
        }
        const forms = Array.from(document.querySelectorAll('form[name="advanced-search"]')).filter((form) =>
          isVisible(form),
        )
        const form = forms[forms.length - 1]
        if (!(form instanceof HTMLFormElement)) {
          return false
        }
        const keywordInput = form.querySelector(
          '#search-keyword, [data-shortcut-target="searchbox-field"], [data-shorcut-target="searchbox-field"], [data-testid="input-input-element"]',
        )
        return {
          hasKeywordInput: keywordInput instanceof HTMLInputElement || keywordInput instanceof HTMLTextAreaElement,
          hasShowMore: !!form.querySelector('[data-testid="advanced-search:show-more"]'),
          formText: normalize(form.innerText || form.textContent || '').slice(0, 300),
        }
      },
      undefined,
      waitTimeoutMs,
    )
    return signal ? { ready: true, ...signal } : { ready: false }
  }

  const ensureAdvancedSearchFormVisible = async () => {
    const existing = await waitForAdvancedSearchForm(Math.max(800, Math.min(timeoutMs, 1200)))
    if (existing.ready) {
      return {
        ready: true,
        reason: 'already-visible',
        revealHint: '',
        revealCandidates: [],
        commanderActionLabel: '',
      }
    }

    let lastReveal = { clicked: false, hint: '', candidates: [] }

    for (let attempt = 0; attempt < 2; attempt += 1) {
      lastReveal = await revealSearchSurface()
      const advancedAfterReveal = await waitForAdvancedSearchForm(Math.max(1200, Math.min(timeoutMs, 2200)))
      if (advancedAfterReveal.ready) {
        return {
          ready: true,
          reason: 'reveal-search',
          revealHint: lastReveal.hint,
          revealCandidates: lastReveal.candidates,
          commanderActionLabel: '',
        }
      }
    }

    return {
      ready: false,
      reason: 'timeout',
      revealHint: lastReveal.hint,
      revealCandidates: lastReveal.candidates,
      commanderActionLabel: '',
    }
  }

  const needsExpandedAdvancedSearch = (attempt) =>
    !!normalizeText(attempt && attempt.senderFieldValue) || !!normalizeText(attempt && attempt.recipientFieldValue)

  const ensureAdvancedSearchExpanded = async (attempt) => {
    if (!needsExpandedAdvancedSearch(attempt)) {
      return { expanded: true, reason: 'not-needed' }
    }

    const fieldsVisible = await waitForPageSignal(
      ({ senderSelectors, recipientSelectors }) => {
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
        }
        const hasVisibleField = (selectors) => {
          for (const selector of selectors || []) {
            const elements = Array.from(document.querySelectorAll(selector))
            if (elements.some((element) => isVisible(element))) {
              return true
            }
          }
          return false
        }
        if (hasVisibleField(senderSelectors) && hasVisibleField(recipientSelectors)) {
          return {
            senderVisible: true,
            recipientVisible: true,
          }
        }
        return false
      },
      {
        senderSelectors: senderFieldSelectors,
        recipientSelectors: recipientFieldSelectors,
      },
      250,
    )
    if (fieldsVisible) {
      return { expanded: true, reason: 'already-expanded' }
    }

    const showMore = page.locator('[data-testid="advanced-search:show-more"]').first()
    if (await showMore.isVisible().catch(() => false)) {
      await markSettingsDrawerProbeAction('advanced-search:show-more')
      await showMore.click({ timeout: 1200 }).catch(() => {})
      await flushSettingsDrawerProbe('after-show-more')
    }

    const expanded = await waitForPageSignal(
      ({ senderSelectors, recipientSelectors }) => {
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
        }
        const hasVisibleField = (selectors) => {
          for (const selector of selectors || []) {
            const elements = Array.from(document.querySelectorAll(selector))
            if (elements.some((element) => isVisible(element))) {
              return true
            }
          }
          return false
        }
        if (hasVisibleField(senderSelectors) && hasVisibleField(recipientSelectors)) {
          return {
            senderVisible: true,
            recipientVisible: true,
          }
        }
        return false
      },
      {
        senderSelectors: senderFieldSelectors,
        recipientSelectors: recipientFieldSelectors,
      },
      1800,
    )
    return expanded ? { expanded: true, reason: 'show-more-clicked' } : { expanded: false, reason: 'fields-not-visible' }
  }

  const waitForSearchSurfaceSignal = async () => {
    const existingInput = await findSearchInput()
    if (existingInput) {
      return { ready: true, reason: 'search-input-present' }
    }
    const signal = await waitForPageSignal(
      ({ selectors }) => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
        }

        for (const selector of selectors) {
          const elements = Array.from(document.querySelectorAll(selector))
          for (let index = elements.length - 1; index >= 0; index -= 1) {
            const element = elements[index]
            if (element instanceof HTMLElement && isVisible(element)) {
              return {
                reason: 'search-input-visible',
                selector,
              }
            }
          }
        }

        const form = Array.from(document.querySelectorAll('form[name="advanced-search"]')).find((element) => isVisible(element))
        if (form) {
          return {
            reason: 'advanced-search-form-visible',
          }
        }

        const isAdvancedSearchRoot = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          return !!element.querySelector(
            'form[name="advanced-search"], [data-testid="advanced-search:submit"], [data-testid="advanced-search:show-more"], [data-testid^="location-"]',
          )
        }
        const dialogRoots = Array.from(
          document.querySelectorAll('[role="dialog"], [data-testid="overlay-button"], div[id^="advanced-search-overlay-"]'),
        ).filter((element) => isVisible(element) && isAdvancedSearchRoot(element))
        const searchButtons = dialogRoots
          .flatMap((root) => Array.from(root.querySelectorAll('button, [role="button"]')))
          .filter((element) => isVisible(element))
          .map((element) =>
            normalize(
              [
                element.textContent || '',
                element.getAttribute('aria-label') || '',
                element.getAttribute('title') || '',
                element.getAttribute('data-testid') || '',
              ].join(' | '),
            ),
          )
          .filter((text) => /(search|搜索)/i.test(text))
          .slice(0, 6)

        if (dialogRoots.length > 0 && searchButtons.length > 0) {
          return {
            reason: 'search-dialog-visible',
            dialogCount: dialogRoots.length,
            searchButtons,
          }
        }
        return false
      },
      { selectors: searchSelectors },
      Math.max(1000, Math.min(timeoutMs, 2200)),
    )
    return signal ? { ready: true, ...signal } : { ready: false, reason: 'timeout' }
  }

  const revealSearchSurface = async () => {
    const topSearchRoot = page.locator(collapsedSearchEntryRootSelector).first()
    if (await topSearchRoot.isVisible().catch(() => false)) {
      await markSettingsDrawerProbeAction('reveal-search-surface', {
        path: 'collapsed-search-root',
        selector: collapsedSearchEntryRootSelector,
      })
      await topSearchRoot.click({ timeout: 1200, force: true }).catch(() => {})
      await flushSettingsDrawerProbe('after-reveal-search', { path: 'collapsed-search-root' })
      return {
        clicked: true,
        hint: 'collapsed-search-root',
        candidates: ['collapsed-search-root'],
      }
    }

    const topSearchReadonlyInput = page.locator(collapsedSearchReadonlyInputSelector).first()
    if (await topSearchReadonlyInput.isVisible().catch(() => false)) {
      await markSettingsDrawerProbeAction('reveal-search-surface', {
        path: 'collapsed-search-readonly-input',
        selector: collapsedSearchReadonlyInputSelector,
      })
      await topSearchReadonlyInput.click({ timeout: 1200, force: true }).catch(() => {})
      await flushSettingsDrawerProbe('after-reveal-search', { path: 'collapsed-search-readonly-input' })
      return {
        clicked: true,
        hint: 'collapsed-search-readonly-input',
        candidates: ['collapsed-search-readonly-input'],
      }
    }

    return {
      clicked: false,
      hint: '',
      candidates: ['collapsed-search-root', 'collapsed-search-readonly-input'],
    }
  }

  const collectSearchDiagnostics = async () => {
    const title = await page.title().catch(() => '')
    const url = page.url()
    const payload = await page
      .evaluate(() => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
        }
        const visibleInputs = Array.from(document.querySelectorAll('input, textarea, [role="searchbox"]'))
          .filter((element) => isVisible(element))
          .slice(0, 10)
          .map((element) =>
            normalize(
              [
                element.getAttribute('type') || '',
                element.getAttribute('placeholder') || '',
                element.getAttribute('aria-label') || '',
                element.getAttribute('data-testid') || '',
                element.className || '',
              ].join(' | '),
            ),
          )
          .filter(Boolean)
        const searchHints = Array.from(document.querySelectorAll('button, [role="button"], a, label, div, span'))
          .filter((element) => isVisible(element))
          .map((element) =>
            normalize(
              [
                element.innerText || element.textContent || '',
                element.getAttribute('aria-label') || '',
                element.getAttribute('title') || '',
                element.getAttribute('data-testid') || '',
                element.className || '',
              ].join(' | '),
            ),
          )
          .filter((text) => /(搜索|search)/i.test(text))
          .slice(0, 12)
        return {
          bodyPreview: normalize(document.body ? document.body.innerText || document.body.textContent || '' : '').slice(0, 800),
          visibleInputs,
          searchHints,
          hasPasswordInput: !!document.querySelector('input[type="password"]'),
          hasEmailInput: !!document.querySelector('input[type="email"]'),
        }
      })
      .catch(() => ({
        bodyPreview: '',
        visibleInputs: [],
        searchHints: [],
        hasPasswordInput: false,
        hasEmailInput: false,
      }))
    return { title, url, ...payload }
  }

  const collectSearchSurfaceState = async () =>
    page
      .evaluate(() => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
        }

        const isAdvancedSearchRoot = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          return !!element.querySelector(
            'form[name="advanced-search"], [data-testid="advanced-search:submit"], [data-testid="advanced-search:show-more"], [data-testid^="location-"]',
          )
        }

        const dialogRoots = Array.from(
          document.querySelectorAll('[role="dialog"], [data-testid="overlay-button"], div[id^="advanced-search-overlay-"]'),
        ).filter((element) => isVisible(element) && isAdvancedSearchRoot(element))

        const searchButtons = dialogRoots
          .flatMap((root) => Array.from(root.querySelectorAll('button, [role="button"]')))
          .filter((element) => isVisible(element))
          .map((element) =>
            normalize(
              [
                element.textContent || '',
                element.getAttribute('aria-label') || '',
                element.getAttribute('title') || '',
                element.getAttribute('data-testid') || '',
              ].join(' | '),
            ),
          )
          .filter((text) => /(search|搜索)/i.test(text))
          .slice(0, 8)

        return {
          dialogCount: dialogRoots.length,
          searchButtons,
        }
      })
      .catch(() => ({
        dialogCount: 0,
        searchButtons: [],
      }))

  const submitSearchAction = async () =>
    page
      .evaluate(() => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
        }

        const roots = Array.from(
          document.querySelectorAll('[role="dialog"], [data-testid="overlay-button"], div[id^="advanced-search-overlay-"]'),
        ).filter((element) => isVisible(element))

        const candidates = []
        for (const root of roots) {
          const buttons = Array.from(root.querySelectorAll('button, [role="button"]'))
          for (const button of buttons) {
            if (!(button instanceof HTMLElement) || !isVisible(button)) {
              continue
            }
            const label = normalize(
              [
                button.innerText || button.textContent || '',
                button.getAttribute('aria-label') || '',
                button.getAttribute('title') || '',
                button.getAttribute('data-testid') || '',
              ].join(' | '),
            )
            if (!/(^|\s)(search|搜索)(\s|$)/i.test(label)) {
              continue
            }
            const rect = button.getBoundingClientRect()
            candidates.push({
              button,
              label,
              score: /(^|\s)(search|搜索)(\s|$)/i.test(label) ? 200 : 0,
              top: rect.top,
              left: rect.left,
              clickX: Math.min(rect.right - 12, Math.max(rect.left + 120, rect.left + rect.width * 0.35)),
              clickY: rect.top + Math.min(rect.height - 8, Math.max(8, rect.height / 2)),
            })
          }
        }

        candidates.sort((a, b) => {
          if (b.score !== a.score) {
            return b.score - a.score
          }
          if (a.top !== b.top) {
            return a.top - b.top
          }
          return a.left - b.left
        })

        const picked = candidates[0]
        if (!picked) {
          return { clicked: false, label: '' }
        }

        picked.button.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window }))
        if (typeof picked.button.click === 'function') {
          picked.button.click()
        }
        return {
          clicked: true,
          label: picked.label,
        }
      })
      .catch(() => ({ clicked: false, label: '' }))

  const findAdvancedSearchField = async (selectors) => {
    for (const selector of selectors) {
      const locator = page.locator(selector)
      const count = Math.min(await locator.count().catch(() => 0), 12)
      for (let index = count - 1; index >= 0; index -= 1) {
        const candidate = locator.nth(index)
        if (await candidate.isVisible().catch(() => false)) {
          return candidate
        }
      }
    }
    return null
  }

  const keywordFieldSelectors = [
    'form[name="advanced-search"] input#search-keyword[data-testid="input-input-element"][data-shorcut-target="searchbox-field"]',
    'form[name="advanced-search"] input#search-keyword[data-testid="input-input-element"]',
    'form[name="advanced-search"] input#search-keyword[title="关键词"]',
  ]
  const senderFieldSelectors = [
    'form[name="advanced-search"] input[data-testid="advanced-search:sender"]#from',
    'form[name="advanced-search"] #from',
  ]
  const recipientFieldSelectors = [
    'form[name="advanced-search"] input[data-testid="advanced-search:recipient"]#to',
    'form[name="advanced-search"] #to',
  ]

  const readLocatorValue = async (locator) =>
    locator
      .evaluate((element) => {
        if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
          return String(element.value || '')
        }
        return String((element && (element.textContent || element.innerText)) || '')
      })
      .catch(() => '')

  const fillAdvancedSearchField = async (selectors, value) => {
    const normalizedValue = normalizeText(value)
    let field = await findAdvancedSearchField(selectors)
    if (!field) {
      await waitForPageSignal(
        ({ selectors }) => {
          const isVisible = (element) => {
            if (!(element instanceof HTMLElement)) {
              return false
            }
            const style = window.getComputedStyle(element)
            if (!style || style.visibility === 'hidden' || style.display === 'none') {
              return false
            }
            const rect = element.getBoundingClientRect()
            return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
          }
          for (const selector of selectors || []) {
            const elements = Array.from(document.querySelectorAll(selector))
            if (elements.some((element) => isVisible(element))) {
              return true
            }
          }
          return false
        },
        { selectors },
        1200,
      ).catch(() => false)
      field = await findAdvancedSearchField(selectors)
    }
    if (!field) {
      return {
        filled: !normalizedValue,
        reason: normalizedValue ? 'field-not-found' : 'field-optional-and-missing',
      }
    }

    await field.click({ timeout: 1000, force: true }).catch(() => {})
    await field.focus({ timeout: 1000 }).catch(() => {})
    try {
      await field.fill('', { timeout: 1200 })
      if (normalizedValue) {
        await field.fill(normalizedValue, { timeout: 1500 })
      }
      await sleep(80)
      const currentValue = normalizeText(await readLocatorValue(field))
      if (!normalizedValue || currentValue === normalizedValue) {
        return {
          filled: true,
          reason: 'locator-fill',
          value: currentValue,
        }
      }
    } catch {}

    await page.keyboard.press('Control+A').catch(() => {})
    await page.keyboard.insertText(normalizedValue).catch(() => {})
    await sleep(80)
    const currentValue = normalizeText(await readLocatorValue(field))
    return {
      filled: !normalizedValue || currentValue === normalizedValue,
      reason: 'keyboard-insert',
      value: currentValue,
    }
  }

  const readAdvancedSearchFormValues = async () =>
    page
      .evaluate(({ keywordSelectors, senderSelectors, recipientSelectors }) => {
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
        }
        const pickValue = (selectors) => {
          for (const selector of selectors) {
            const candidates = Array.from(document.querySelectorAll(selector))
            for (let index = candidates.length - 1; index >= 0; index -= 1) {
              const candidate = candidates[index]
              if (
                (candidate instanceof HTMLInputElement || candidate instanceof HTMLTextAreaElement) &&
                isVisible(candidate)
              ) {
                return String(candidate.value || '')
              }
            }
          }
          for (const selector of selectors) {
            const candidate = document.querySelector(selector)
            if (candidate instanceof HTMLInputElement || candidate instanceof HTMLTextAreaElement) {
              return String(candidate.value || '')
            }
          }
          return ''
        }
        return {
          keywordValue: pickValue(keywordSelectors || []),
          senderValue: pickValue(senderSelectors || []),
          recipientValue: pickValue(recipientSelectors || []),
        }
      }, {
        keywordSelectors: keywordFieldSelectors,
        senderSelectors: senderFieldSelectors,
        recipientSelectors: recipientFieldSelectors,
      })
      .catch(() => ({
        keywordValue: '',
        senderValue: '',
        recipientValue: '',
      }))

  const buildSearchHashParams = (attempt) => {
    const keywordValue = normalizeText(attempt && attempt.keywordQuery)
    const senderValue = normalizeText(attempt && attempt.senderFieldValue)
    const recipientValue = normalizeText(attempt && attempt.recipientFieldValue)
    const params = []
    if (keywordValue) {
      params.push(['keyword', keywordValue])
    }
    if (senderValue) {
      params.push(['from', senderValue])
    }
    if (recipientValue) {
      params.push(['to', recipientValue])
    }
    return params
  }

  const waitForSearchRouteApplied = async (attempt, waitTimeoutMs = Math.max(800, Math.min(timeoutMs, 2500))) => {
    const expectedHashParams = buildSearchHashParams(attempt).map(([key, value]) => [key, value.toLowerCase()])
    if (expectedHashParams.length === 0) {
      return { applied: false, reason: 'empty-search-params' }
    }
    const signal = await waitForPageSignal(
      ({ expectedHashParams }) => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const hashParams = new URLSearchParams(String(window.location.hash || '').replace(/^#/, ''))
        const hashMatched = expectedHashParams.every(
          ([key, value]) => normalize(hashParams.get(key)).toLowerCase() === value,
        )
        if (!hashMatched) {
          return false
        }
        return {
          applied: true,
          href: window.location.href || '',
        }
      },
      { expectedHashParams },
      waitTimeoutMs,
    )
    return signal || { applied: false, reason: 'timeout' }
  }

  const buildDirectSearchUrl = (attempt) => {
    const currentRoute = parseProtonMailboxRoute(page && !page.isClosed() ? page.url() : '')
    const inboxRoute = parseProtonMailboxRoute(inboxUrl)
    const route = currentRoute || inboxRoute || { origin: 'https://mail.proton.me', accountIndex: '0' }
    const params = new URLSearchParams()
    for (const [key, value] of buildSearchHashParams(attempt)) {
      params.set(key, value)
    }
    const hash = params.toString()
    return `${route.origin}/u/${route.accountIndex}/almost-all-mail${hash ? '#' + hash : ''}`
  }

  const submitDirectSearchRoute = async (attempt, reason) => {
    const directSearchUrl = buildDirectSearchUrl(attempt)
    await markSettingsDrawerProbeAction('direct-search-route:submit', {
      reason,
      url: directSearchUrl,
    })
    log('directSearchRouteSubmit', {
      reason,
      url: directSearchUrl,
      keywordQuery: normalizeText(attempt && attempt.keywordQuery),
      senderFieldValue: normalizeText(attempt && attempt.senderFieldValue),
      recipientFieldValue: normalizeText(attempt && attempt.recipientFieldValue),
    })
    await page.goto(directSearchUrl, { waitUntil: 'domcontentloaded', timeout: Math.min(timeoutMs, 15000) }).catch((error) => {
      log('directSearchRouteGotoFailed', String((error && error.message) || error || 'unknown'))
    })
    const routeState = await waitForSearchRouteApplied(attempt, Math.max(800, Math.min(timeoutMs, 3000)))
    await flushSettingsDrawerProbe('after-direct-search-route', {
      reason,
      applied: !!routeState.applied,
      url: directSearchUrl,
    })
    return {
      submitted: !!routeState.applied,
      reason: routeState.applied ? 'direct-route-' + reason : 'direct-route-not-applied',
      routeState,
      url: directSearchUrl,
    }
  }

  const shouldUseDirectSearchRoute = () => isProtonMessageDetailUrl(page && !page.isClosed() ? page.url() : '')

  const waitForAdvancedSearchReset = async (waitTimeoutMs = Math.max(800, Math.min(timeoutMs, 2500))) =>
    waitForPageSignal(
      () => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
        }
        const forms = Array.from(document.querySelectorAll('form[name="advanced-search"]')).filter((form) =>
          isVisible(form),
        )
        const form = forms[forms.length - 1]
        if (!(form instanceof HTMLFormElement)) {
          return false
        }

        const activeControls = []
        for (const element of Array.from(form.querySelectorAll('input, textarea, select'))) {
          if (!(element instanceof HTMLElement) || !isVisible(element)) {
            continue
          }
          if (element instanceof HTMLInputElement) {
            const type = String(element.type || '').toLowerCase()
            if (['hidden', 'submit', 'button', 'image'].includes(type)) {
              continue
            }
            if (['checkbox', 'radio'].includes(type)) {
              if (element.checked) {
                activeControls.push(type + ':' + (element.name || element.id || element.getAttribute('data-testid') || 'checked'))
              }
              continue
            }
            if (normalize(element.value || '')) {
              activeControls.push(type + ':' + normalize(element.value || '').slice(0, 60))
            }
            continue
          }
          if (element instanceof HTMLTextAreaElement) {
            if (normalize(element.value || '')) {
              activeControls.push('textarea:' + normalize(element.value || '').slice(0, 60))
            }
            continue
          }
          if (element instanceof HTMLSelectElement) {
            if (element.selectedIndex > 0 && normalize(element.value || '')) {
              activeControls.push('select:' + normalize(element.value || '').slice(0, 60))
            }
          }
        }

        return activeControls.length === 0
          ? {
              cleared: true,
            }
          : false
      },
      undefined,
      waitTimeoutMs,
    )

  const resetAdvancedSearchForm = async () => {
    await markSettingsDrawerProbeAction('advanced-search:reset')
    const resetResult = await page
      .evaluate(() => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const isVisible = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = element.getBoundingClientRect()
          return rect.width >= 18 && rect.height >= 18 && rect.bottom >= 0 && rect.top <= window.innerHeight
        }
        const forms = Array.from(document.querySelectorAll('form[name="advanced-search"]')).filter((form) =>
          isVisible(form),
        )
        const form = forms[forms.length - 1]
        if (!(form instanceof HTMLFormElement)) {
          return { reset: false, mode: 'missing-form' }
        }

        const clickElement = (element) => {
          if (!(element instanceof HTMLElement)) {
            return false
          }
          element.scrollIntoView({ behavior: 'auto', block: 'center', inline: 'center' })
          if (typeof element.focus === 'function') {
            element.focus()
          }
          element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window }))
          if (typeof element.click === 'function') {
            element.click()
          }
          return true
        }

        const buttonCandidates = Array.from(form.querySelectorAll('button, [role="button"]'))
          .filter((element) => isVisible(element))
          .map((element) => {
            const label = normalize(
              [
                element.textContent || '',
                element.getAttribute('aria-label') || '',
                element.getAttribute('title') || '',
                element.getAttribute('data-testid') || '',
              ].join(' | '),
            )
            const rect = element.getBoundingClientRect()
            let score = 0
            if (/(重置搜索条件|reset search|reset filters)/i.test(label)) {
              score += 300
            }
            if (/(^|\s)(重置|reset)(\s|$)/i.test(label)) {
              score += 200
            }
            if (rect.top > window.innerHeight * 0.5) {
              score += 20
            }
            if (rect.left > window.innerWidth * 0.5) {
              score += 10
            }
            return {
              element,
              label,
              score,
              top: rect.top,
              left: rect.left,
            }
          })
          .filter((item) => item.score > 0)

        buttonCandidates.sort((a, b) => {
          if (b.score !== a.score) {
            return b.score - a.score
          }
          if (a.top !== b.top) {
            return a.top - b.top
          }
          return a.left - b.left
        })

        const pickedButton = buttonCandidates[0]
        if (pickedButton && clickElement(pickedButton.element)) {
          return {
            reset: true,
            mode: 'button',
            label: pickedButton.label,
          }
        }

        const setInputValue = (element, value) => {
          if (!(element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement)) {
            return
          }
          const prototype =
            element instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype
          const valueDescriptor = Object.getOwnPropertyDescriptor(prototype, 'value')
          if (valueDescriptor && typeof valueDescriptor.set === 'function') {
            valueDescriptor.set.call(element, value)
          } else {
            element.value = value
          }
          element.dispatchEvent(new Event('input', { bubbles: true }))
          element.dispatchEvent(new Event('change', { bubbles: true }))
        }

        let touchedControls = 0
        for (const element of Array.from(form.querySelectorAll('input, textarea, select'))) {
          if (!(element instanceof HTMLElement) || !isVisible(element)) {
            continue
          }
          if (element instanceof HTMLInputElement) {
            const type = String(element.type || '').toLowerCase()
            if (['hidden', 'submit', 'button', 'image'].includes(type)) {
              continue
            }
            if (['checkbox', 'radio'].includes(type)) {
              if (element.checked) {
                element.checked = false
                element.dispatchEvent(new Event('input', { bubbles: true }))
                element.dispatchEvent(new Event('change', { bubbles: true }))
                touchedControls += 1
              }
              continue
            }
            if (normalize(element.value || '')) {
              setInputValue(element, '')
              touchedControls += 1
            }
            continue
          }
          if (element instanceof HTMLTextAreaElement) {
            if (normalize(element.value || '')) {
              setInputValue(element, '')
              touchedControls += 1
            }
            continue
          }
          if (element instanceof HTMLSelectElement) {
            if (element.selectedIndex !== 0) {
              element.selectedIndex = 0
              element.dispatchEvent(new Event('input', { bubbles: true }))
              element.dispatchEvent(new Event('change', { bubbles: true }))
              touchedControls += 1
            }
          }
        }

        const allMailButton = form.querySelector('[data-testid="location-15"]')
        if (allMailButton instanceof HTMLElement && isVisible(allMailButton)) {
          clickElement(allMailButton)
        }

        return {
          reset: true,
          mode: 'manual-clear',
          touchedControls,
        }
      })
      .catch((error) => ({
        reset: false,
        mode: 'error',
        reason: String((error && error.message) || error || 'unknown'),
      }))

    const resetSignal = await waitForAdvancedSearchReset().catch(() => null)
    const currentValues = await readAdvancedSearchFormValues()
    await flushSettingsDrawerProbe('after-advanced-search-reset', {
      mode: resetResult.mode,
      cleared: !!resetSignal,
    })
    return {
      ...resetResult,
      cleared: !!resetSignal,
      ...currentValues,
    }
  }

  const submitAdvancedSearchForm = async (attempt) => {
    const keywordValue = normalizeText(attempt && attempt.keywordQuery)
    const senderValue = normalizeText(attempt && attempt.senderFieldValue)
    const recipientValue = normalizeText(attempt && attempt.recipientFieldValue)

    await markSettingsDrawerProbeAction('advanced-search:fill-keyword', {
      valuePreview: keywordValue.slice(0, 120),
      valueLength: keywordValue.length,
    })
    const keywordFilled = await fillAdvancedSearchField(keywordFieldSelectors, keywordValue)
    await flushSettingsDrawerProbe('after-advanced-search-fill-keyword', {
      filled: keywordFilled.filled,
      reason: keywordFilled.reason,
    })
    if (!keywordFilled.filled) {
      return { submitted: false, reason: 'keyword-input-not-found' }
    }

    await markSettingsDrawerProbeAction('advanced-search:fill-sender', {
      valuePreview: senderValue.slice(0, 120),
      valueLength: senderValue.length,
    })
    const senderFilled = await fillAdvancedSearchField(senderFieldSelectors, senderValue)
    await flushSettingsDrawerProbe('after-advanced-search-fill-sender', {
      filled: senderFilled.filled,
      reason: senderFilled.reason,
    })
    if (!senderFilled.filled) {
      return { submitted: false, reason: 'sender-input-not-found' }
    }

    await markSettingsDrawerProbeAction('advanced-search:fill-recipient', {
      valuePreview: recipientValue.slice(0, 120),
      valueLength: recipientValue.length,
    })
    const recipientFilled = await fillAdvancedSearchField(recipientFieldSelectors, recipientValue)
    await flushSettingsDrawerProbe('after-advanced-search-fill-recipient', {
      filled: recipientFilled.filled,
      reason: recipientFilled.reason,
    })
    if (!recipientFilled.filled) {
      return { submitted: false, reason: 'recipient-input-not-found' }
    }

    const preSubmitValues = await readAdvancedSearchFormValues()

    const allMailButton = page.locator('form[name="advanced-search"] [data-testid="location-15"]').first()
    if (await allMailButton.isVisible().catch(() => false)) {
      await markSettingsDrawerProbeAction('advanced-search:location-all-mail')
      await allMailButton.click({ timeout: 1200 }).catch(() => {})
      await flushSettingsDrawerProbe('after-advanced-search-location-all-mail')
    }

    const submitButton = page.locator('form[name="advanced-search"] [data-testid="advanced-search:submit"]').first()
    if (await submitButton.isVisible().catch(() => false)) {
      await markSettingsDrawerProbeAction('advanced-search:submit')
      await submitButton.click({ timeout: 1500 }).catch(() => {})
      await flushSettingsDrawerProbe('after-advanced-search-submit')
      return {
        submitted: true,
        reason: 'button-clicked',
        ...preSubmitValues,
      }
    }

    return page
      .evaluate(() => {
        const form = Array.from(document.querySelectorAll('form[name="advanced-search"]')).find((element) => {
          if (!(element instanceof HTMLFormElement)) {
            return false
          }
          const style = window.getComputedStyle(element)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          return true
        })
        if (!(form instanceof HTMLFormElement)) {
          return { submitted: false, reason: 'form-not-found' }
        }
        if (typeof form.requestSubmit === 'function') {
          form.requestSubmit()
          return { submitted: true, reason: 'request-submit' }
        }
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
        return { submitted: true, reason: 'submit-event' }
      })
      .then(async (result) => ({
        ...result,
        ...(result && result.submitted ? await readAdvancedSearchFormValues() : {}),
      }))
      .catch((error) => ({ submitted: false, reason: String((error && error.message) || error || 'unknown') }))
  }

  let searchInput = null
  let searchReveal = { clicked: false, hint: '', candidates: [] }
  const useInitialDirectSearchRoute = shouldUseDirectSearchRoute()
  const advancedSearchSurface = useInitialDirectSearchRoute
    ? {
        ready: false,
        reason: 'message-detail-direct-route',
        revealHint: '',
        revealCandidates: ['message-detail-direct-route'],
        commanderActionLabel: '',
      }
    : await ensureAdvancedSearchFormVisible()
  searchReveal = {
    clicked: !!advancedSearchSurface.revealHint,
    hint: advancedSearchSurface.revealHint || '',
    candidates: advancedSearchSurface.revealCandidates || [],
  }
  log('searchSurfaceSignal', {
    ready: advancedSearchSurface.ready,
    reason: advancedSearchSurface.reason,
    revealHint: advancedSearchSurface.revealHint,
    revealCandidates: advancedSearchSurface.revealCandidates,
    commanderActionLabel: advancedSearchSurface.commanderActionLabel,
  })
  if (advancedSearchSurface.ready) {
    searchInput = await findSearchInput()
  }
  if (!searchInput) {
    const diagnostics = await collectSearchDiagnostics()
    const surfaceState = await collectSearchSurfaceState()
    log('searchSurfaceUnavailable', {
      title: diagnostics.title,
      url: diagnostics.url,
      revealHint: searchReveal.hint,
      revealCandidates: searchReveal.candidates,
      visibleInputs: diagnostics.visibleInputs,
      searchHints: diagnostics.searchHints,
      dialogCount: surfaceState.dialogCount,
      searchButtons: surfaceState.searchButtons,
      hasPasswordInput: diagnostics.hasPasswordInput,
      hasEmailInput: diagnostics.hasEmailInput,
      bodyPreview: diagnostics.bodyPreview,
    })
  }

  const fillSearchInput = async (locator, value) => {
    const normalizedValue = normalizeText(value)
    await markSettingsDrawerProbeAction('search-input:fill', {
      valuePreview: normalizedValue.slice(0, 120),
      valueLength: normalizedValue.length,
    })
    const domUpdated = await locator
      .evaluate((element, nextValue) => {
        if (
          !(
            element instanceof HTMLInputElement ||
            element instanceof HTMLTextAreaElement ||
            (element instanceof HTMLElement && element.isContentEditable)
          )
        ) {
          return false
        }
        if (element instanceof HTMLElement) {
          element.focus()
          element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window }))
          if (typeof element.click === 'function') {
            element.click()
          }
        }

        if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
          element.removeAttribute('readonly')
        }

        if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
          const prototype =
            element instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype
          const valueDescriptor = Object.getOwnPropertyDescriptor(prototype, 'value')
          const assignValue = (value) => {
            if (valueDescriptor && typeof valueDescriptor.set === 'function') {
              valueDescriptor.set.call(element, value)
            } else {
              element.value = value
            }
          }

          assignValue('')
          element.dispatchEvent(new Event('input', { bubbles: true }))
          assignValue(nextValue)
          element.dispatchEvent(new Event('input', { bubbles: true }))
          element.dispatchEvent(new Event('change', { bubbles: true }))
          return true
        }

        element.textContent = nextValue
        element.dispatchEvent(new InputEvent('input', { bubbles: true, data: nextValue, inputType: 'insertText' }))
        element.dispatchEvent(new Event('change', { bubbles: true }))
        return true
      }, normalizedValue)
      .catch(() => false)
    if (domUpdated) {
      return
    }

    await locator.click({ timeout: 800, force: true }).catch(() => {})
    await locator.focus({ timeout: 800 }).catch(() => {})
    try {
      await locator.fill('', { timeout: 1000 })
      await locator.fill(normalizedValue, { timeout: 1000 })
      return
    } catch {}

    await page.keyboard.press('Control+A').catch(() => {})
    await page.keyboard.insertText(normalizedValue).catch(() => {})
  }

    const searchSurfaceState = await collectSearchSurfaceState()
  log('searchSurfaceReady', {
    revealHint: searchReveal.hint,
    revealCandidates: searchReveal.candidates,
    dialogCount: searchSurfaceState.dialogCount,
    searchButtons: searchSurfaceState.searchButtons,
  })

  const submitSearchQuery = async (attempt) => {
    const normalizedAttempt =
      typeof attempt === 'string'
        ? {
            label: 'search',
            keywordQuery: normalizeText(attempt),
            senderFieldValue: '',
            recipientFieldValue: '',
          }
        : {
            label: normalizeText(attempt && attempt.label) || 'search',
            keywordQuery: normalizeText(attempt && attempt.keywordQuery),
            senderFieldValue: normalizeText(attempt && attempt.senderFieldValue),
          recipientFieldValue: normalizeText(attempt && attempt.recipientFieldValue),
        }

    if (shouldUseDirectSearchRoute()) {
      const directSubmit = await submitDirectSearchRoute(normalizedAttempt, 'message-detail-route')
      log('advancedSearchDirectFallback', directSubmit)
      return !!directSubmit.submitted
    }

    let advancedFormReady = await waitForAdvancedSearchForm(500)
    if (!advancedFormReady.ready) {
      const reopenedAdvancedForm = await ensureAdvancedSearchFormVisible()
      if (reopenedAdvancedForm.ready) {
        advancedFormReady = reopenedAdvancedForm
      }
    }
    if (advancedFormReady.ready) {
      let resetState = await resetAdvancedSearchForm()
      if (resetState.mode === 'missing-form') {
        const reopenedAdvancedForm = await ensureAdvancedSearchFormVisible()
        if (reopenedAdvancedForm.ready) {
          resetState = await resetAdvancedSearchForm()
        }
      }
      log('advancedSearchReset', {
        label: normalizedAttempt.label,
        keywordQuery: normalizedAttempt.keywordQuery,
        senderFieldValue: normalizedAttempt.senderFieldValue,
        recipientFieldValue: normalizedAttempt.recipientFieldValue,
        mode: resetState.mode,
        cleared: resetState.cleared,
        keywordValue: resetState.keywordValue,
        senderValue: resetState.senderValue,
        recipientValue: resetState.recipientValue,
        reason: resetState.reason || '',
      })
      let expandedState = await ensureAdvancedSearchExpanded(normalizedAttempt)
      if (!expandedState.expanded && needsExpandedAdvancedSearch(normalizedAttempt)) {
        const reopenedExpandedForm = await ensureAdvancedSearchFormVisible()
        if (reopenedExpandedForm.ready) {
          const retryResetState = await resetAdvancedSearchForm()
          log('advancedSearchResetRetry', {
            label: normalizedAttempt.label,
            keywordQuery: normalizedAttempt.keywordQuery,
            senderFieldValue: normalizedAttempt.senderFieldValue,
            recipientFieldValue: normalizedAttempt.recipientFieldValue,
            mode: retryResetState.mode,
            cleared: retryResetState.cleared,
            keywordValue: retryResetState.keywordValue,
            senderValue: retryResetState.senderValue,
            recipientValue: retryResetState.recipientValue,
            reason: retryResetState.reason || '',
          })
          expandedState = await ensureAdvancedSearchExpanded(normalizedAttempt)
        }
      }
      if (!expandedState.expanded && needsExpandedAdvancedSearch(normalizedAttempt)) {
        log('advancedSearchExpandFailed', {
          label: normalizedAttempt.label,
          keywordQuery: normalizedAttempt.keywordQuery,
          senderFieldValue: normalizedAttempt.senderFieldValue,
          recipientFieldValue: normalizedAttempt.recipientFieldValue,
          reason: expandedState.reason,
        })
        return false
      }
      const advancedSubmit = await submitAdvancedSearchForm(normalizedAttempt)
      if (advancedSubmit.submitted) {
        const routeState = await waitForSearchRouteApplied(normalizedAttempt)
        log('advancedSearchSubmit', {
          ...advancedSubmit,
          expandReason: expandedState.reason,
          routeApplied: !!routeState.applied,
          routeState,
        })
        if (routeState.applied) {
          return true
        }
        const directSubmit = await submitDirectSearchRoute(normalizedAttempt, 'advanced-submit-route-timeout')
        log('advancedSearchDirectFallback', directSubmit)
        return !!directSubmit.submitted
      }
      log('advancedSearchSubmitFailed', advancedSubmit)
      const directSubmit = await submitDirectSearchRoute(normalizedAttempt, 'advanced-submit-failed')
      log('advancedSearchDirectFallback', directSubmit)
      return !!directSubmit.submitted
    }
    const directSubmit = await submitDirectSearchRoute(normalizedAttempt, 'advanced-form-unavailable')
    log('advancedSearchDirectFallback', directSubmit)
    return !!directSubmit.submitted
  }

  const pickMessageRow = async (filters, candidateOffset) =>
    page.evaluate(
      ({ filters, candidateOffset, preferLatest, rowSelectors }) => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const includesText = (haystack, needle) => {
          const normalizedHaystack = normalize(haystack).toLowerCase()
          const normalizedTerms = normalize(needle)
            .split(/[,，;；|\n]+/g)
            .map((term) => normalize(term).toLowerCase())
            .filter(Boolean)
          if (normalizedTerms.length === 0) {
            return false
          }
          return normalizedTerms.some((term) => normalizedHaystack.includes(term))
        }
        const rowSelector = rowSelectors.join(',')
        const getCandidateRoot = (element) => {
          for (const selector of rowSelectors) {
            const closest = element.closest(selector)
            if (closest instanceof HTMLElement) {
              return closest
            }
          }
          return element
        }
        const getDOMIndex = (element) => {
          const rows = Array.from(document.querySelectorAll(rowSelector))
          const directIndex = rows.indexOf(element)
          if (directIndex >= 0) {
            return directIndex
          }
          const nestedIndex = rows.findIndex((row) => row instanceof HTMLElement && row.contains(element))
          return nestedIndex >= 0 ? nestedIndex : Number.MAX_SAFE_INTEGER
        }
        const parseTimeMs = (element, text) => {
          const parseLocalizedTimestamp = (value) => {
            const normalized = normalize(value)
            if (!normalized) {
              return 0
            }

            const timeMatches = Array.from(
              normalized.matchAll(
                /(?:(上午|下午|中午|凌晨|早上|晚上|晚间|am|pm)\s*)?(\d{1,2})[:：](\d{2})(?:\s*(上午|下午|中午|凌晨|早上|晚上|晚间|am|pm))?/gi,
              ),
            )
            if (timeMatches.length === 0) {
              return 0
            }

            const lastMatch = timeMatches[timeMatches.length - 1]
            const meridiem = String(lastMatch[1] || lastMatch[4] || '').toLowerCase()
            let hour = Number(lastMatch[2])
            const minute = Number(lastMatch[3])
            if (!Number.isFinite(hour) || !Number.isFinite(minute)) {
              return 0
            }
            if (meridiem) {
              if (/(下午|晚上|晚间|pm|中午)/i.test(meridiem) && hour < 12) {
                hour += 12
              } else if (/(上午|凌晨|早上|am)/i.test(meridiem) && hour === 12) {
                hour = 0
              }
            } else if (/(下午|晚上|晚间|pm|中午)/i.test(normalized) && hour < 12) {
              hour += 12
            } else if (/(上午|凌晨|早上|am)/i.test(normalized) && hour === 12) {
              hour = 0
            }

            const now = new Date()
            const dateMatch = normalized.match(/(\d{4})[年/-](\d{1,2})[月/-](\d{1,2})日?/)
            const baseDate = new Date(now.getTime())
            if (dateMatch) {
              baseDate.setFullYear(Number(dateMatch[1]), Number(dateMatch[2]) - 1, Number(dateMatch[3]))
            } else if (/昨天|yesterday/i.test(normalized)) {
              baseDate.setDate(baseDate.getDate() - 1)
            } else if (/前天/i.test(normalized)) {
              baseDate.setDate(baseDate.getDate() - 2)
            }

            baseDate.setHours(hour, minute, 0, 0)
            const parsed = baseDate.getTime()
            return Number.isFinite(parsed) ? parsed : 0
          }
          const timeElement = element.querySelector('time[datetime]')
          const datetime = timeElement ? timeElement.getAttribute('datetime') : ''
          const parsedDatetime = datetime ? Date.parse(datetime) : NaN
          if (Number.isFinite(parsedDatetime)) {
            return parsedDatetime
          }

          const timestampText = normalize(
            [
              element.getAttribute('datetime') || '',
              element.getAttribute('title') || '',
              element.getAttribute('aria-label') || '',
              timeElement ? timeElement.textContent || '' : '',
              text,
            ].join(' '),
          )
          const parsedTextTime = Date.parse(timestampText)
          if (Number.isFinite(parsedTextTime)) {
            return parsedTextTime
          }
          return parseLocalizedTimestamp(timestampText)
        }
        const seen = new Set()
        const candidates = []
        const pickToken = 'ant-automation-picked-row-' + Date.now() + '-' + Math.random().toString(16).slice(2)

        const isUsable = (el) => {
          if (!(el instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(el)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = el.getBoundingClientRect()
          if (rect.width < 120 || rect.height < 24) {
            return false
          }
          return true
        }

        for (const selector of rowSelectors) {
          const elements = Array.from(document.querySelectorAll(selector))
          for (const element of elements) {
            if (!(element instanceof HTMLElement) || !isUsable(element)) {
              continue
            }
            const root = getCandidateRoot(element)
            if (!(root instanceof HTMLElement) || seen.has(root) || !isUsable(root)) {
              continue
            }
            seen.add(root)

            const text = normalize(root.innerText || root.textContent || '')
            if (text.length < 8) {
              continue
            }

            const rect = root.getBoundingClientRect()
            const isMessageCandidate = root.matches(rowSelector)

            let score = 0
            let matchedSignals = 0
            const matchedBy = []
            const applySignal = (name, value, points) => {
              if (!value) {
                return
              }
              if (includesText(text, value)) {
                score += points
                matchedSignals += 1
                matchedBy.push(name)
              }
            }
            applySignal('searchQuery', filters.searchQuery, 70)
            applySignal('recipientQuery', filters.recipientQuery, 120)
            applySignal('subjectQuery', filters.subjectQuery, 180)
            applySignal('senderEmail', filters.senderEmail, 150)
            if (isMessageCandidate) {
              score += 35
            }
            if (rect.left < window.innerWidth * 0.8) {
              score += 10
            }
            if (rect.top > 70) {
              score += 10
            }
            if (rect.width > 220) {
              score += 10
            }
            if (rect.height <= 140) {
              score += 10
            }

            candidates.push({
              element: root,
              pickToken,
              text,
              score,
              matchedSignals,
              matchedBy,
              isMessageCandidate,
              domIndex: getDOMIndex(root),
              timeMs: parseTimeMs(root, text),
              top: rect.top,
              left: rect.left,
              clickX: Math.min(rect.right - 12, Math.max(rect.left + 120, rect.left + rect.width * 0.35)),
              clickY: rect.top + Math.min(rect.height - 8, Math.max(8, rect.height / 2)),
              clickRelativeX: Math.min(rect.width - 12, Math.max(120, rect.width * 0.35)),
              clickRelativeY: Math.min(rect.height - 8, Math.max(8, rect.height / 2)),
            })
          }
        }

        candidates.sort((a, b) => {
          if (a.top !== b.top) {
            return a.top - b.top
          }
          if (b.matchedSignals !== a.matchedSignals) {
            return b.matchedSignals - a.matchedSignals
          }
          if (b.score !== a.score) {
            return b.score - a.score
          }
          if (a.domIndex !== b.domIndex) {
            return a.domIndex - b.domIndex
          }
          return a.left - b.left
        })

        const index = Number.isFinite(Number(candidateOffset)) ? Math.max(0, Math.round(Number(candidateOffset))) : 0
        const picked = candidates[index]
        if (!picked) {
          return null
        }

        for (const candidate of candidates) {
          if (candidate.element && candidate.element.removeAttribute) {
            candidate.element.removeAttribute('data-ant-automation-picked-row')
          }
        }
        picked.element.setAttribute('data-ant-automation-picked-row', picked.pickToken)

        return {
          pickToken: picked.pickToken,
          text: picked.text,
          score: picked.score,
          matchedSignals: picked.matchedSignals,
          matchedBy: picked.matchedBy,
          isMessageCandidate: picked.isMessageCandidate,
          domIndex: picked.domIndex,
          timeMs: picked.timeMs,
          clickX: picked.clickX,
          clickY: picked.clickY,
          clickRelativeX: picked.clickRelativeX,
          clickRelativeY: picked.clickRelativeY,
          candidateOffset: index,
          candidateCount: candidates.length,
        }
      },
      { filters, candidateOffset, preferLatest, rowSelectors: messageRowSelectors },
    )

  const openPickedMessageRow = async (pickedRow) => {
    if (!pickedRow) {
      return { clicked: false, strategy: 'missing-row' }
    }
    await stabilizeMessageListSurface()
    const attempts = []
    const relativeX = Number(pickedRow.clickRelativeX)
    const relativeY = Number(pickedRow.clickRelativeY)
    if (pickedRow.pickToken) {
      const tokenSelector = '[data-ant-automation-picked-row="' + String(pickedRow.pickToken).replace(/"/g, '\\"') + '"]'
      const locator = page.locator(tokenSelector).first()
      if (await locator.isVisible().catch(() => false)) {
        await locator.scrollIntoViewIfNeeded().catch(() => {})
        const getMousePositionFromBox = async () => {
          const box = await locator.boundingBox().catch(() => null)
          if (!box) {
            return null
          }
          return {
            x: box.x + Math.min(box.width - 12, Math.max(120, box.width * 0.35)),
            y: box.y + Math.min(box.height - 8, Math.max(8, box.height / 2)),
          }
        }
        const position = {
          x: Number.isFinite(relativeX) ? Math.max(1, relativeX) : 120,
          y: Number.isFinite(relativeY) ? Math.max(1, relativeY) : 20,
        }
        attempts.push([
          'locator-position',
          () =>
            locator.click({
              timeout: Math.max(500, openMailTimeoutMs),
              position,
            }),
        ])
        attempts.push([
          'locator-force-center',
          () =>
            locator.click({
              timeout: Math.max(500, openMailTimeoutMs),
              force: true,
            }),
        ])
        attempts.push([
          'locator-dom-click',
          () =>
            locator.evaluate((element) => {
              const clickTarget = element.querySelector('a, button, [role="button"]') || element
              if (!(clickTarget instanceof HTMLElement)) {
                return false
              }
              if (typeof clickTarget.focus === 'function') {
                clickTarget.focus()
              }
              clickTarget.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, view: window }))
              clickTarget.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, view: window }))
              clickTarget.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window }))
              if (typeof clickTarget.click === 'function') {
                clickTarget.click()
              }
              return true
            }),
        ])
        attempts.push([
          'locator-enter',
          async () => {
            await locator.focus({ timeout: Math.max(500, openMailTimeoutMs) }).catch(() => {})
            await locator.press('Enter', { timeout: Math.max(500, openMailTimeoutMs) })
          },
        ])
        attempts.push([
          'locator-mouse-position',
          async () => {
            const mousePosition = await getMousePositionFromBox()
            if (!mousePosition) {
              throw new Error('missing-bounding-box')
            }
            await page.mouse.click(mousePosition.x, mousePosition.y)
          },
        ])
      }
    }
    if (Number.isFinite(Number(pickedRow.clickX)) && Number.isFinite(Number(pickedRow.clickY))) {
      attempts.push([
        'mouse-position',
        () => page.mouse.click(Number(pickedRow.clickX), Number(pickedRow.clickY)),
      ])
    }
    for (const [strategy, action] of attempts) {
      const clicked = await Promise.resolve()
        .then(() => action())
        .then(() => true)
        .catch(() => false)
      if (clicked) {
        return { clicked: true, strategy }
      }
    }
    return { clicked: false, strategy: 'all-failed' }
  }

  const extractOpenedMail = async () => {
    const baseMail = await page.evaluate(
      ({ maxChars }) => {
      const normalizeLines = (value) =>
        String(value || '')
          .replace(/\r/g, '')
          .split('\n')
          .map((line) => line.replace(/\s+/g, ' ').trim())
          .filter(Boolean)
          .join('\n')

      const visible = (el) => {
        if (!el) {
          return false
        }
        const style = window.getComputedStyle(el)
        if (!style || style.visibility === 'hidden' || style.display === 'none') {
          return false
        }
        const rect = el.getBoundingClientRect()
        if (rect.width < 240 || rect.height < 100) {
          return false
        }
        if (rect.bottom < 60 || rect.top > window.innerHeight) {
          return false
        }
        return true
      }

      const extractIframeText = (root) => {
        const iframeTexts = []
        const iframeElements = Array.from((root || document).querySelectorAll('iframe'))
        for (const frameElement of iframeElements) {
          if (!(frameElement instanceof HTMLIFrameElement) || !visible(frameElement)) {
            continue
          }
          try {
            const frameDocument = frameElement.contentDocument
            const frameBody = frameDocument && frameDocument.body
            const frameText = normalizeLines(frameBody ? frameBody.innerText || frameBody.textContent || '' : '')
            if (frameText.length >= 6) {
              iframeTexts.push(frameText)
            }
          } catch {}
        }
        return iframeTexts
      }
      const extractIframeSubjects = (root) => {
        const subjects = []
        const iframeElements = Array.from((root || document).querySelectorAll('iframe'))
        for (const frameElement of iframeElements) {
          if (!(frameElement instanceof HTMLIFrameElement) || !visible(frameElement)) {
            continue
          }
          const subject = normalizeLines(
            [
              frameElement.getAttribute('data-subject') || '',
              frameElement.getAttribute('aria-label') || '',
            ].join('\n'),
          )
          if (subject.length >= 4 && subject.length <= 240) {
            subjects.push(subject)
          }
        }
        return subjects
      }

      const selectors = [
        '[data-testid*="message-view"]',
        '[data-testid*="conversation-view"]',
        '[data-shortcut-target="message-container"]',
        '.message-container.is-opened',
        '[role="article"]',
        'main article',
      ]

      const seen = new Set()
      const candidates = []
      for (const selector of selectors) {
        const elements = Array.from(document.querySelectorAll(selector))
        for (const element of elements) {
          if (!(element instanceof HTMLElement) || seen.has(element) || !visible(element)) {
            continue
          }
          seen.add(element)
          const rect = element.getBoundingClientRect()
          const text = normalizeLines(element.innerText || element.textContent || '')
          if (text.length < 30) {
            continue
          }

          let score = text.length
          if (rect.left > window.innerWidth * 0.25) {
            score += 240
          }
          if (rect.width > window.innerWidth * 0.35) {
            score += 120
          }
          score += Math.round(rect.height)

          candidates.push({
            element,
            text,
            score,
          })
        }
      }

      candidates.sort((a, b) => b.score - a.score)
      if (!candidates[0]) {
        return null
      }
      const root = candidates[0].element
      const rootText = normalizeLines(root.innerText || root.textContent || '')
      const iframeTexts = Array.from(new Set([...extractIframeText(root), ...extractIframeText(document)]))
      const iframeSubjects = Array.from(new Set([...extractIframeSubjects(root), ...extractIframeSubjects(document)]))
      const contentText = [...iframeTexts, rootText].filter(Boolean).join('\n')

      let subject = iframeSubjects[0] || ''
      const headingCandidates = Array.from(root.querySelectorAll('h1, h2, h3'))
      if (!subject) {
        for (const heading of headingCandidates) {
          const text = normalizeLines(heading.textContent || '')
          if (text.length >= 4 && text.length <= 240) {
            subject = text
            break
          }
        }
      }
      if (!subject) {
        subject = (contentText.split('\n')[0] || '').slice(0, 240)
      }

      const slicedContentText = contentText.slice(0, maxChars)
      const lines = contentText.split('\n').filter(Boolean)
      const metaLines = lines.slice(0, 12).join('\n')
      const headerText = lines.slice(0, 20).join('\n')

      return {
        subject,
        contentText: slicedContentText,
        metaText: metaLines,
        headerText,
      }
      },
      { maxChars: maxBodyChars },
    )

    if (!baseMail) {
      return null
    }

    const normalizeFrameText = (value) =>
      String(value || '')
        .replace(/\r/g, '')
        .split('\n')
        .map((line) => line.replace(/\s+/g, ' ').trim())
        .filter(Boolean)
        .join('\n')
    const frameTexts = []
    for (const frame of page.frames()) {
      if (frame === page.mainFrame()) {
        continue
      }
      try {
        const frameText = normalizeFrameText(await frame.locator('body').innerText({ timeout: 250 }))
        if (frameText.length >= 6) {
          frameTexts.push(frameText)
        }
      } catch {}
    }
    const contentText = Array.from(new Set([...frameTexts, baseMail.contentText].filter(Boolean))).join('\n').slice(0, maxBodyChars)
    const lines = contentText.split('\n').filter(Boolean)
    return {
      ...baseMail,
      contentText,
      metaText: lines.slice(0, 12).join('\n') || baseMail.metaText,
      headerText: lines.slice(0, 20).join('\n') || baseMail.headerText,
    }
  }

  const getMessageListSnapshot = async () =>
    page
      .evaluate(({ selectors }) => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const isUsable = (el) => {
          if (!(el instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(el)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = el.getBoundingClientRect()
          return rect.width >= 120 && rect.height >= 24
        }
        const getCandidateRoot = (element) => {
          for (const selector of selectors || []) {
            const closest = element.closest(selector)
            if (closest instanceof HTMLElement) {
              return closest
            }
          }
          return element
        }
        const rows = []
        const seen = new Set()
        for (const selector of selectors || []) {
          for (const element of Array.from(document.querySelectorAll(selector))) {
            if (!(element instanceof HTMLElement) || !isUsable(element)) {
              continue
            }
            const root = getCandidateRoot(element)
            if (!(root instanceof HTMLElement) || seen.has(root) || !isUsable(root)) {
              continue
            }
            seen.add(root)
            const text = normalize(root.innerText || root.textContent || '')
            if (text.length >= 8) {
              rows.push(text)
            }
          }
        }
        return rows.slice(0, 8).join('\n').slice(0, 2000)
      }, { selectors: messageRowSelectors })
      .catch(() => '')

  const getMessageListState = async () => ({
    snapshot: await getMessageListSnapshot(),
    url: page.url(),
  })

  const waitForVisibleMessageCandidate = async (waitTimeoutMs = Math.max(500, Math.min(timeoutMs, 2500))) =>
    waitForPageSignal(
      ({ selectors }) => {
        const normalize = (value) =>
          String(value || '')
            .replace(/\s+/g, ' ')
            .trim()
        const isUsable = (el) => {
          if (!(el instanceof HTMLElement)) {
            return false
          }
          const style = window.getComputedStyle(el)
          if (!style || style.visibility === 'hidden' || style.display === 'none') {
            return false
          }
          const rect = el.getBoundingClientRect()
          return rect.width >= 120 && rect.height >= 24
        }
        const getCandidateRoot = (element) => {
          for (const selector of selectors || []) {
            const closest = element.closest(selector)
            if (closest instanceof HTMLElement) {
              return closest
            }
          }
          return element
        }
        const rows = []
        const seen = new Set()
        for (const selector of selectors || []) {
          for (const element of Array.from(document.querySelectorAll(selector))) {
            if (!(element instanceof HTMLElement) || !isUsable(element)) {
              continue
            }
            const root = getCandidateRoot(element)
            if (!(root instanceof HTMLElement) || seen.has(root) || !isUsable(root)) {
              continue
            }
            seen.add(root)
            const text = normalize(root.innerText || root.textContent || '')
            if (text.length >= 8) {
              rows.push(text)
            }
          }
        }
        if (rows.length === 0) {
          return false
        }
        return {
          count: rows.length,
          firstRowText: rows[0].slice(0, 240),
        }
      },
      { selectors: messageRowSelectors },
      waitTimeoutMs,
    )

  const waitForSearchResults = async (previousState, attempt) => {
    if (searchResultTimeoutMs <= 0) {
      return false
    }
    const previousSnapshot = normalizeLineText(previousState && previousState.snapshot).slice(0, 2000)
    const previousUrl = normalizeText(previousState && previousState.url)
    const keywordQuery = normalizeText(attempt && attempt.keywordQuery)
    const senderFieldValue = normalizeText(attempt && attempt.senderFieldValue)
    const recipientFieldValue = normalizeText(attempt && attempt.recipientFieldValue)
    const expectedQuery = normalizeText(keywordQuery || recipientFieldValue || senderFieldValue).toLowerCase()
    const expectedHashParams = [
      keywordQuery ? ['keyword', keywordQuery.toLowerCase()] : null,
      senderFieldValue ? ['from', senderFieldValue.toLowerCase()] : null,
      recipientFieldValue ? ['to', recipientFieldValue.toLowerCase()] : null,
    ].filter(Boolean)
    const startedAt = Date.now()
    return page
      .waitForFunction(
        ({ expectedQuery, previousSnapshot, previousUrl, expectedHashParams, searchSettleMs, selectors, startedAt }) => {
          const normalize = (value) =>
            String(value || '')
              .replace(/\s+/g, ' ')
              .trim()
          const isUsable = (el) => {
            if (!(el instanceof HTMLElement)) {
              return false
            }
            const style = window.getComputedStyle(el)
            if (!style || style.visibility === 'hidden' || style.display === 'none') {
              return false
            }
            const rect = el.getBoundingClientRect()
            return rect.width >= 120 && rect.height >= 24
          }
          const getCandidateRoot = (element) => {
            for (const selector of selectors || []) {
              const closest = element.closest(selector)
              if (closest instanceof HTMLElement) {
                return closest
              }
            }
            return element
          }
          const rows = []
          const seen = new Set()
          for (const selector of selectors || []) {
            for (const element of Array.from(document.querySelectorAll(selector))) {
              if (!(element instanceof HTMLElement) || !isUsable(element)) {
                continue
              }
              const root = getCandidateRoot(element)
              if (!(root instanceof HTMLElement) || seen.has(root) || !isUsable(root)) {
                continue
              }
              seen.add(root)
              const text = normalize(root.innerText || root.textContent || '')
              if (text.length >= 8) {
                rows.push(text)
              }
            }
          }
          const snapshot = rows.slice(0, 8).join('\n').slice(0, 2000)
          const href = window.location.href || ''
          const hashParams = new URLSearchParams(String(window.location.hash || '').replace(/^#/, ''))
          const hashMatched =
            expectedHashParams.length > 0 &&
            expectedHashParams.every(([key, value]) => normalize(hashParams.get(key)).toLowerCase() === value)
          const bodyText = normalize(document.body ? document.body.innerText || document.body.textContent || '' : '')
          const emptyStateVisible = /(找到 0 条结果|找不到您想要的结果|0 results|no results)/i.test(bodyText)
          const elapsedMs = Date.now() - Number(startedAt || 0)
          const snapshotChanged = !previousSnapshot || snapshot !== previousSnapshot
          const urlChanged = !!previousUrl && href !== previousUrl
          if (elapsedMs < Number(searchSettleMs || 0)) {
            return false
          }
          if (rows.length === 0 && !emptyStateVisible) {
            return false
          }
          if (emptyStateVisible && hashMatched) {
            return {
              reason: 'empty-state-visible',
              href,
              rowCount: rows.length,
              emptyStateVisible,
              hashMatched,
            }
          }
          if (rows.length > 0 && snapshotChanged) {
            const haystack = snapshot.toLowerCase()
            const queryMatched = !expectedQuery || haystack.includes(expectedQuery)
            if (queryMatched || hashMatched || urlChanged) {
              return {
                reason: queryMatched ? 'snapshot-query-matched' : hashMatched ? 'snapshot-hash-matched' : 'snapshot-url-changed',
                href,
                rowCount: rows.length,
                emptyStateVisible,
                hashMatched,
                snapshotChanged,
              }
            }
          }
          if (!previousSnapshot && rows.length > 0) {
            return {
              reason: 'rows-visible-no-previous-snapshot',
              href,
              rowCount: rows.length,
              emptyStateVisible,
              hashMatched,
            }
          }
          return false
        },
        { expectedQuery, previousSnapshot, previousUrl, expectedHashParams, searchSettleMs, selectors: messageRowSelectors, startedAt },
        { timeout: searchResultTimeoutMs },
      )
      .then((handle) => handle.jsonValue())
      .catch(() => false)
  }

  const getOpenedMailSnapshot = async () =>
    extractOpenedMail()
      .then((opened) => normalizeLineText([opened.subject, opened.metaText, opened.headerText].join('\n')).slice(0, 1200))
      .catch(() => '')

  const waitForOpenedMailChange = async (previousSnapshot) => {
    if (openMailTimeoutMs <= 0) {
      return false
    }
    const previous = normalizeLineText(previousSnapshot).slice(0, 1200)
    return page
      .waitForFunction(
        ({ previous }) => {
          const normalizeLines = (value) =>
            String(value || '')
              .replace(/\r/g, '')
              .split('\n')
              .map((line) => line.replace(/\s+/g, ' ').trim())
              .filter(Boolean)
              .join('\n')
          const visible = (el) => {
            if (!el) {
              return false
            }
            const style = window.getComputedStyle(el)
            if (!style || style.visibility === 'hidden' || style.display === 'none') {
              return false
            }
            const rect = el.getBoundingClientRect()
            return rect.width >= 240 && rect.height >= 100 && rect.bottom >= 60 && rect.top <= window.innerHeight
          }
          const iframeTexts = []
          for (const frameElement of Array.from(document.querySelectorAll('iframe'))) {
            if (!(frameElement instanceof HTMLIFrameElement) || !visible(frameElement)) {
              continue
            }
            try {
              const frameDocument = frameElement.contentDocument
              const frameBody = frameDocument && frameDocument.body
              const frameText = normalizeLines(frameBody ? frameBody.innerText || frameBody.textContent || '' : '')
              if (frameText.length >= 6) {
                iframeTexts.push(frameText)
              }
            } catch {}
          }
          const iframeText = iframeTexts.join('\n')
          if (/([^\d]|^)[0-9]{6}(?!\d)/.test(iframeText) && (!previous || !iframeText.includes(previous))) {
            return true
          }
          const selectors = [
            '[data-testid*="message-view"]',
            '[data-testid*="conversation-view"]',
            '[data-shortcut-target="message-container"]',
            '.message-container.is-opened',
            '[role="article"]',
            'main article',
          ]
          const candidates = []
          const seen = new Set()
          for (const selector of selectors) {
            for (const element of Array.from(document.querySelectorAll(selector))) {
              if (!(element instanceof HTMLElement) || seen.has(element) || !visible(element)) {
                continue
              }
              seen.add(element)
              const text = normalizeLines(element.innerText || element.textContent || '')
              if (text.length < 30) {
                continue
              }
              const rect = element.getBoundingClientRect()
              let score = text.length
              if (rect.left > window.innerWidth * 0.25) {
                score += 240
              }
              if (rect.width > window.innerWidth * 0.35) {
                score += 120
              }
              score += Math.round(rect.height)
              candidates.push({ text, score })
            }
          }
          candidates.sort((a, b) => b.score - a.score)
          const text = [iframeText, candidates[0] ? candidates[0].text : ''].filter(Boolean).join('\n').slice(0, 1200)
          if (!text) {
            return false
          }
          return !previous || text !== previous
        },
        { previous },
        { timeout: openMailTimeoutMs },
      )
      .then(() => true)
      .catch(() => false)
  }

  const waitForOpenedVerificationCode = async () => {
    const deadline = Date.now() + Math.max(200, openMailTimeoutMs)
    let latestMail = null
    while (Date.now() <= deadline) {
      latestMail = await extractOpenedMail().catch(() => null)
      if (latestMail) {
        const code = extractVerificationCode(latestMail.contentText, latestMail.metaText, latestMail.headerText)
        if (code) {
          return { mail: latestMail, verificationCode: code }
        }
      }
      await sleep(120)
    }
    return { mail: latestMail, verificationCode: '' }
  }

  let clickedRow = null
  let mail = null
  let senderInfo = { raw: '', name: '', email: '' }
  let recipientInfo = { raw: '', name: '', email: '' }
  let verificationCode = ''
  let signature = ''
  let mailboxName = ''
  let matchReport = { ok: true, mismatches: [] }
  let submittedSearchQuery = ''
  let searchedQueries = []
  let submittedSearchAttempts = []

  if (allowOpenedMailShortcut) {
    mail = await extractOpenedMail().catch(() => null)
  }
  if (mail) {
    senderInfo = extractMailbox([mail.headerText, mail.metaText, mail.contentText], ['从', '发件人', 'from', 'sender'])
    recipientInfo = extractMailbox([mail.headerText, mail.metaText, mail.contentText], ['收件人', 'to', 'recipient'])
    verificationCode = extractVerificationCode(mail.contentText, mail.metaText, mail.headerText)
    signature = extractSignature(mail.contentText)
    mailboxName = senderInfo.name || senderInfo.email
    matchReport = matchMailAgainstFilters(mail, senderInfo, recipientInfo, matchFilters)
    if (matchReport.ok && verificationCode) {
      clickedRow = {
        text: mail.headerText,
        score: 0,
        matchedSignals: 0,
        matchedBy: ['openedMail'],
        isMessageCandidate: true,
        domIndex: 0,
        timeMs: 0,
        candidateOffset: 0,
        candidateCount: 1,
      }
      submittedSearchQuery = searchQuery
      searchedQueries.push(searchQuery)
    } else {
      mail = null
    }
  }

  for (let searchPass = 0; searchPass < maxSearchPasses; searchPass += 1) {
    if (mail && matchReport.ok) {
      break
    }
    for (const [attemptIndex, attempt] of searchAttempts.entries()) {
    if (mail && matchReport.ok) {
      break
    }
    let firstAttemptEmptyRetryUsed = false
    const attemptFilters = buildAttemptMatchFilters(attempt)
    const query =
      normalizeText(attempt.keywordQuery) ||
      normalizeText(attempt.recipientFieldValue) ||
      normalizeText(attempt.senderFieldValue)
    let searchResultReady = false
    let visibleCandidateState = null
    for (let attemptSubmission = 0; attemptSubmission < 2; attemptSubmission += 1) {
      const isFirstAttemptEmptyRetry = attemptSubmission === 1
      if (isFirstAttemptEmptyRetry) {
        if (firstAttemptEmptyRetryDelayMs <= 0) {
          break
        }
        firstAttemptEmptyRetryUsed = true
        log('firstAttemptEmptyRetryWait', {
          pass: searchPass + 1,
          label: attempt.label,
          query,
          delayMs: firstAttemptEmptyRetryDelayMs,
        })
        await sleep(firstAttemptEmptyRetryDelayMs)
        log('firstAttemptEmptyRetryReload', {
          pass: searchPass + 1,
          label: attempt.label,
          query,
          url: page.url(),
        })
        await page.reload({ waitUntil: 'domcontentloaded', timeout: Math.min(timeoutMs, 15000) }).catch((error) => {
          log('firstAttemptEmptyRetryReloadFailed', String((error && error.message) || error || 'unknown'))
        })
        await waitForMailboxShell().catch(() => null)
      }

      const previousListState = await getMessageListState()
      await markSettingsDrawerProbeAction('search-attempt:start', {
        pass: searchPass + 1,
        label: attempt.label,
        query,
        keywordQuery: normalizeText(attempt.keywordQuery),
        senderFieldValue: normalizeText(attempt.senderFieldValue),
        recipientFieldValue: normalizeText(attempt.recipientFieldValue),
        firstAttemptEmptyRetry: isFirstAttemptEmptyRetry,
      })
      const searchSubmitted = await submitSearchQuery(attempt)
      if (!searchSubmitted) {
        break
      }
      submittedSearchQuery = query
      searchedQueries.push(query)
      submittedSearchAttempts.push({
        pass: searchPass + 1,
        label: attempt.label,
        query,
        keywordQuery: normalizeText(attempt.keywordQuery),
        senderFieldValue: normalizeText(attempt.senderFieldValue),
        recipientFieldValue: normalizeText(attempt.recipientFieldValue),
        firstAttemptEmptyRetry: isFirstAttemptEmptyRetry,
      })
      await markSettingsDrawerProbeAction('search-results:wait', {
        pass: searchPass + 1,
        label: attempt.label,
        query,
        firstAttemptEmptyRetry: isFirstAttemptEmptyRetry,
      })
      searchResultReady = await waitForSearchResults(previousListState, attempt)
      await flushSettingsDrawerProbe('after-search-results-wait', {
        label: attempt.label,
        query,
        pass: searchPass + 1,
        firstAttemptEmptyRetry: isFirstAttemptEmptyRetry,
        ready: !!searchResultReady,
      })
      if (!searchResultReady) {
        await page.waitForLoadState('networkidle', { timeout: Math.min(timeoutMs, 1200) }).catch(() => {})
        log('legacySearchWaitFallback', { query, waitAfterSearchMs, firstAttemptEmptyRetry: isFirstAttemptEmptyRetry })
        if (waitAfterSearchMs > 0) {
          await sleep(waitAfterSearchMs)
        }
      }
      log('searchResultSignal', {
        label: attempt.label,
        query,
        pass: searchPass + 1,
        keywordQuery: attempt.keywordQuery,
        senderFieldValue: attempt.senderFieldValue,
        recipientFieldValue: attempt.recipientFieldValue,
        firstAttemptEmptyRetry: isFirstAttemptEmptyRetry,
        ready: searchResultReady,
      })
      await stabilizeMessageListSurface()
      visibleCandidateState = await waitForVisibleMessageCandidate(
        Math.max(500, Math.min(timeoutMs, searchResultTimeoutMs + 1200)),
      )
      log('searchCandidateSignal', {
        label: attempt.label,
        query,
        pass: searchPass + 1,
        keywordQuery: attempt.keywordQuery,
        senderFieldValue: attempt.senderFieldValue,
        recipientFieldValue: attempt.recipientFieldValue,
        firstAttemptEmptyRetry: isFirstAttemptEmptyRetry,
        ready: !!visibleCandidateState,
        count: visibleCandidateState ? visibleCandidateState.count : 0,
        firstRowText: visibleCandidateState ? visibleCandidateState.firstRowText : '',
      })

      const isFirstSearchCombination = searchPass === 0 && attemptIndex === 0
      const isEmptySearchResult =
        !visibleCandidateState &&
        searchResultReady &&
        searchResultReady.reason === 'empty-state-visible' &&
        Number(searchResultReady.rowCount || 0) === 0
      if (!isFirstSearchCombination || !isEmptySearchResult || firstAttemptEmptyRetryUsed) {
        break
      }
    }

    const candidateIndex = 0
    await stabilizeMessageListSurface()
    const previousMailSnapshot = await getOpenedMailSnapshot()
    await markSettingsDrawerProbeAction('search-row:pick', {
      pass: searchPass + 1,
      label: attempt.label,
      query,
      candidateIndex,
    })
    clickedRow = await pickMessageRow(attemptFilters, candidateIndex)
    if (!clickedRow) {
      continue
    }

    await markSettingsDrawerProbeAction('search-row:open', {
      pass: searchPass + 1,
      label: attempt.label,
      query,
      candidateIndex,
    })
    const clickResult = await openPickedMessageRow(clickedRow)
    const clicked = clickResult.clicked
    await flushSettingsDrawerProbe('after-open-picked-row', {
      label: attempt.label,
      query,
      pass: searchPass + 1,
      candidateIndex,
      clicked,
      strategy: clickResult.strategy,
    })

    const openedMailReady = await waitForOpenedMailChange(previousMailSnapshot)
    if (!openedMailReady) {
      await page.waitForLoadState('networkidle', { timeout: Math.min(timeoutMs, 1200) }).catch(() => {})
      if (waitAfterOpenMs > 0) {
        await sleep(waitAfterOpenMs)
      }
    }
    log('openedMailSignal', {
      candidateIndex,
      clicked,
      ready: openedMailReady,
      strategy: clickResult.strategy,
    })
    const openedCodeResult = await waitForOpenedVerificationCode()
    mail = openedCodeResult.mail || (await extractOpenedMail())
    if (!mail) {
      matchReport = { ok: false, mismatches: ['邮件详情未打开'] }
      continue
    }
    senderInfo = extractMailbox([mail.headerText, mail.metaText, mail.contentText], ['从', '发件人', 'from', 'sender'])
    recipientInfo = extractMailbox([mail.headerText, mail.metaText, mail.contentText], ['收件人', 'to', 'recipient'])
    verificationCode = openedCodeResult.verificationCode || extractVerificationCode(mail.contentText, mail.metaText, mail.headerText)
    signature = extractSignature(mail.contentText)
    mailboxName = senderInfo.name || senderInfo.email
    matchReport = matchMailAgainstFilters(mail, senderInfo, recipientInfo, attemptFilters)

    log('candidateCheck', {
      searchQuery: query,
      pass: searchPass + 1,
      candidateIndex,
      matchedBy: clickedRow.matchedBy,
      subject: mail.subject,
      senderEmail: senderInfo.email,
      recipientEmail: recipientInfo.email,
      mismatches: matchReport.mismatches,
    })

    if (mail && matchReport.ok) {
      break
    }
    }
    break
  }

  if (searchedQueries.length === 0) {
    throw new Error('Proton 搜索未提交')
  }

  if (!clickedRow) {
    return {
      ok: false,
      summary: '未找到匹配邮件',
      error: '未找到符合搜索条件的邮件：' + searchQuery,
      recipientQuery,
      subjectQuery,
      senderEmail,
      searchQuery,
      searchQueries,
      searchedQueries,
      submittedSearchAttempts,
      inboxUrl,
    }
  }

  if (!mail || !matchReport.ok) {
    return {
      ok: false,
      summary: '未找到符合过滤条件的邮件',
      error:
        '已检查前 ' +
        maxCandidateChecks +
        ' 封候选邮件，仍未满足过滤条件。' +
        (matchReport.mismatches.length > 0 ? ' 最后一次不匹配：' + matchReport.mismatches.join('；') : ''),
      recipientQuery,
      subjectQuery,
      senderEmail,
      searchQuery,
      searchQueries,
      searchedQueries,
      submittedSearchAttempts,
      inboxUrl,
      matchedRowText: clickedRow.text,
      subject: mail ? mail.subject : '',
    }
  }

  if (!verificationCode) {
    return {
      ok: false,
      summary: '邮件已命中，但未提取到验证码',
      error:
        '已命中目标邮件，但正文里没有提取到 verificationCode。' +
        (mail && mail.subject ? ' 当前主题：' + mail.subject : ''),
      recipientQuery,
      subjectQuery,
      senderEmail,
      searchQuery,
      searchQueries,
      searchedQueries,
      submittedSearchAttempts,
      inboxUrl,
      matchedRowText: clickedRow ? clickedRow.text : '',
      subject: mail ? mail.subject : '',
      senderEmailDetected: senderInfo.email,
      recipientEmail: recipientInfo.email,
    }
  }

  let screenshotPath = ''
  if (captureScreenshot) {
    screenshotPath = artifact('proton-mail-first-message.png')
    await page.screenshot({ path: screenshotPath, fullPage: true }).catch(() => {})
  }

  log('searchQuery', searchQuery)
  log('searchQueries', searchQueries)
  log('submittedSearchQuery', submittedSearchQuery)
  log('submittedSearchAttempts', submittedSearchAttempts)
  log('recipientQuery', recipientQuery)
  log('clickedRow', clickedRow)
  log('subject', mail.subject)

  return {
    ok: true,
    summary: preferLatest ? '已返回最新命中邮件内容' : '已返回命中邮件内容',
    verificationCode,
    subject: mail.subject,
    mailboxName,
    senderName: senderInfo.name,
    senderEmail: senderInfo.email,
    recipientEmail: recipientInfo.email,
    signature,
    checkedCandidateCount: Number(clickedRow.candidateOffset || 0) + 1,
    submittedSearchAttempts,
  }
}
