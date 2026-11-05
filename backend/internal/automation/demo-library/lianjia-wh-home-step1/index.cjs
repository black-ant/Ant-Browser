const fs = require('fs')

module.exports.run = async ({ launch, connect, openPage, selector, params, log, artifact }) => {
  const normalizeText = (value) => String(value == null ? '' : value).trim()
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
    const text = normalizeText(value) || 'https://wh.lianjia.com/'
    const parsed = new URL(text)
    if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') {
      throw new Error('targetUrl 必须是 http/https URL')
    }
    return parsed.toString()
  }
  const detectCaptcha = (title, bodyText, currentUrl) => {
    const haystack = [title, bodyText, currentUrl].join('\n')
    return /(captcha|CAPTCHA|人机验证|安全验证|验证中心|滑块|拖动滑块|请完成验证)/i.test(haystack)
  }
  const buildCookieHeader = (cookies) => cookies
    .filter((item) => item && item.name)
    .map((item) => item.name + '=' + item.value)
    .join('; ')

  const targetUrl = ensureURL(params.targetUrl)
  const timeoutMs = normalizeInt(params.timeoutMs, 60000, 1000, 180000)
  const waitAfterLoadMs = normalizeInt(params.waitAfterLoadMs, 2500, 0, 30000)
  const captureScreenshot = normalizeBool(params.captureScreenshot, true)
  const keepOpen = normalizeBool(params.keepOpen, true)

  log('step', 'S1_ENTER_LIANJIA_HOME')
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
  const bodyText = await page.locator('body').innerText({ timeout: Math.min(timeoutMs, 5000) }).catch(() => '')
  const bodyPreview = bodyText.replace(/\s+/g, ' ').trim().slice(0, 500)
  const captchaDetected = detectCaptcha(title, bodyPreview, currentUrl)
  const cookies = await page.context().cookies([targetUrl]).catch(() => [])
  const cookieHeader = buildCookieHeader(cookies)

  const cookiesPath = artifact('lianjia-wh-s1-cookies.json')
  fs.writeFileSync(cookiesPath, JSON.stringify(cookies, null, 2), 'utf8')
  const cookieHeaderPath = artifact('lianjia-wh-s1-cookie-header.txt')
  fs.writeFileSync(cookieHeaderPath, cookieHeader, 'utf8')

  let screenshotPath = ''
  if (captureScreenshot) {
    screenshotPath = artifact('lianjia-wh-s1-page.png')
    await page.screenshot({ path: screenshotPath, fullPage: true }).catch(() => {
      screenshotPath = ''
    })
  }

  log('status', status)
  log('currentUrl', currentUrl)
  log('title', title)
  log('captchaDetected', captchaDetected)
  log('cookieCount', cookies.length)
  log('cookiesPath', cookiesPath)

  if (!keepOpen && !page.isClosed()) {
    await page.close().catch(() => {})
  }

  return {
    ok: true,
    summary: captchaDetected
      ? '已进入链家武汉首页，但检测到验证码/安全验证页面，请在浏览器内人工完成验证。'
      : '已进入链家武汉首页，并导出当前 Cookie。',
    step: 'S1_ENTER_LIANJIA_HOME',
    targetUrl,
    currentUrl,
    status,
    title,
    captchaDetected,
    cookieCount: cookies.length,
    cookiesPath,
    cookieHeaderPath,
    screenshotPath,
    bodyPreview,
    keepOpen,
  }
}
