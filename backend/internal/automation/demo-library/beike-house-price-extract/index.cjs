const fs = require('fs')

module.exports.run = async ({ launch, connect, openPage, selector, params, log, artifact }) => {
  const normalizeText = (value) => String(value == null ? '' : value).replace(/\s+/g, ' ').trim()
  const normalizeInt = (value, fallback, min, max) => {
    const parsed = Number(value)
    if (!Number.isFinite(parsed)) return fallback
    const rounded = Math.round(parsed)
    if (rounded < min) return min
    if (rounded > max) return max
    return rounded
  }
  const normalizeBool = (value, fallback) => {
    if (value === true || value === false) return value
    if (typeof value === 'string') {
      const normalized = value.trim().toLowerCase()
      if (['1', 'true', 'yes', 'on'].includes(normalized)) return true
      if (['0', 'false', 'no', 'off'].includes(normalized)) return false
    }
    return fallback
  }
  const ensureURL = (value) => {
    const text = normalizeText(value) || 'https://wx.ke.com/ershoufang/103147107668.html'
    const parsed = new URL(text)
    if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') {
      throw new Error('targetUrl 必须是 http/https URL')
    }
    return parsed.toString()
  }
  const detectCaptcha = (title, bodyText, currentUrl) => {
    const haystack = [title, bodyText, currentUrl].join('\n')
    return /(captcha|CAPTCHA|人机验证|安全验证|验证中心|滑块|拖动滑块|请完成验证|访问异常)/i.test(haystack)
  }
  const firstText = async (page, selectors, timeoutMs) => {
    for (const css of selectors) {
      const text = await page.locator(css).first().innerText({ timeout: timeoutMs }).catch(() => '')
      const normalized = normalizeText(text)
      if (normalized) return normalized
    }
    return ''
  }
  const firstAttribute = async (page, selectors, attribute, timeoutMs) => {
    for (const css of selectors) {
      const value = await page.locator(css).first().getAttribute(attribute, { timeout: timeoutMs }).catch(() => '')
      const normalized = normalizeText(value)
      if (normalized) return normalized
    }
    return ''
  }
  const extractHouseCodeFromURL = (value) => {
    const parsed = new URL(value)
    const matched = parsed.pathname.match(/\/ershoufang\/(\d+)\.html/i)
    return matched ? matched[1] : ''
  }
  const csvEscape = (value) => '"' + String(value == null ? '' : value).replace(/"/g, '""') + '"'

  const targetUrl = ensureURL(params.targetUrl)
  const timeoutMs = normalizeInt(params.timeoutMs, 60000, 1000, 180000)
  const waitAfterLoadMs = normalizeInt(params.waitAfterLoadMs, 2500, 0, 30000)
  const captureScreenshot = normalizeBool(params.captureScreenshot, true)
  const keepOpen = normalizeBool(params.keepOpen, true)
  const fieldTimeoutMs = Math.min(timeoutMs, 5000)

  log('step', 'BEIKE_HOUSE_PRICE_EXTRACT')
  log('targetUrl', targetUrl)

  const session = await launch({
    selector,
    startUrls: [targetUrl],
    skipDefaultStartUrls: true,
  })
  const connection = await connect(session)
  const opened = await openPage(connection, {
    reuseCurrentPage: true,
    bringToFront: true,
    timeoutMs,
  })
  const page = opened.page

  const response = await page.goto(targetUrl, {
    waitUntil: 'domcontentloaded',
    timeout: timeoutMs,
  })
  await page.waitForLoadState('networkidle', { timeout: Math.min(timeoutMs, 10000) }).catch(() => {})
  if (waitAfterLoadMs > 0) {
    await page.waitForTimeout(waitAfterLoadMs)
  }

  const title = await page.title().catch(() => '')
  const currentUrl = page.url()
  const status = response ? response.status() : 0
  const bodyText = await page.locator('body').innerText({ timeout: fieldTimeoutMs }).catch(() => '')
  const bodyPreview = normalizeText(bodyText).slice(0, 500)
  const captchaDetected = detectCaptcha(title, bodyPreview, currentUrl)

  const totalPriceValue = await firstText(page, [
    '.price .total',
    '.price span.total',
    '.sellDetailHeader .price .total',
  ], fieldTimeoutMs)
  const totalPriceUnit = await firstText(page, [
    '.price .unit span',
    '.price span.unit span',
    '.sellDetailHeader .price .unit span',
  ], fieldTimeoutMs)
  const unitPriceValue = await firstText(page, [
    '.unitPrice .unitPriceValue',
    '.price .unitPriceValue',
  ], fieldTimeoutMs)
  const unitPriceUnit = await firstText(page, [
    '.unitPrice i',
    '.price .unitPrice i',
  ], fieldTimeoutMs)
  const houseCodeFromDOM = await firstAttribute(page, [
    '[data-housecode]',
    '.tax[data-housecode]',
  ], 'data-housecode', fieldTimeoutMs)
  const houseCode = houseCodeFromDOM || extractHouseCodeFromURL(currentUrl) || extractHouseCodeFromURL(targetUrl)

  const missingFields = []
  if (!totalPriceValue) missingFields.push('totalPrice.value')
  if (!totalPriceUnit) missingFields.push('totalPrice.unit')
  if (!unitPriceValue) missingFields.push('unitPrice.value')
  if (!unitPriceUnit) missingFields.push('unitPrice.unit')

  const extractedAt = new Date().toISOString()
  const record = {
    source: 'beike',
    step: 'BEIKE_HOUSE_PRICE_EXTRACT',
    extractedAt,
    targetUrl,
    currentUrl,
    status,
    title,
    captchaDetected,
    houseCode,
    totalPrice: {
      value: totalPriceValue,
      unit: totalPriceUnit,
      text: totalPriceValue && totalPriceUnit ? totalPriceValue + totalPriceUnit : '',
    },
    unitPrice: {
      value: unitPriceValue,
      unit: unitPriceUnit,
      text: unitPriceValue && unitPriceUnit ? unitPriceValue + unitPriceUnit : '',
    },
    missingFields,
    bodyPreview,
  }

  const recordPath = artifact('beike-house-price-record.json')
  fs.writeFileSync(recordPath, JSON.stringify(record, null, 2), 'utf8')
  const csvPath = artifact('beike-house-price-record.csv')
  const csvHeader = [
    'extractedAt',
    'targetUrl',
    'currentUrl',
    'houseCode',
    'totalPriceValue',
    'totalPriceUnit',
    'unitPriceValue',
    'unitPriceUnit',
    'status',
    'title',
  ].join(',')
  const csvRow = [
    extractedAt,
    targetUrl,
    currentUrl,
    houseCode,
    totalPriceValue,
    totalPriceUnit,
    unitPriceValue,
    unitPriceUnit,
    status,
    title,
  ].map(csvEscape).join(',')
  fs.writeFileSync(csvPath, csvHeader + '\n' + csvRow + '\n', 'utf8')

  let screenshotPath = ''
  if (captureScreenshot) {
    screenshotPath = artifact('beike-house-price-page.png')
    await page.screenshot({ path: screenshotPath, fullPage: true }).catch(() => {
      screenshotPath = ''
    })
  }

  log('status', status)
  log('currentUrl', currentUrl)
  log('title', title)
  log('captchaDetected', captchaDetected)
  log('houseCode', houseCode)
  log('totalPrice', record.totalPrice.text)
  log('unitPrice', record.unitPrice.text)
  log('missingFields', missingFields.join(', '))
  log('recordPath', recordPath)

  if (!keepOpen && !page.isClosed()) {
    await page.close().catch(() => {})
  }

  return {
    ok: !captchaDetected && missingFields.length === 0,
    summary: captchaDetected
      ? '已打开贝壳页面，但检测到验证码/安全验证，未确认可提取价格。'
      : missingFields.length > 0
        ? '已打开贝壳页面，但价格字段提取不完整：' + missingFields.join(', ')
        : '已提取总价 ' + record.totalPrice.text + '，单价 ' + record.unitPrice.text + '。',
    ...record,
    recordPath,
    csvPath,
    screenshotPath,
    keepOpen,
  }
}
