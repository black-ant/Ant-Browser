const fs = require('fs')
const path = require('path')

function normalizeText(value) {
  return String(value || '').trim()
}

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

function sanitizeFileName(value, fallback) {
  const normalized = normalizeText(value || fallback)
    .replace(/[\\/:*?"<>|]+/g, '-')
    .replace(/\s+/g, ' ')
    .trim()
  return normalized || fallback
}

function ensurePngExtension(fileName) {
  if (/\.(png|jpg|jpeg|webp)$/i.test(fileName)) {
    return fileName
  }
  return fileName + '.png'
}

function resolveOutputPath(outputDir, outputFileName) {
  const safeName = ensurePngExtension(sanitizeFileName(outputFileName, 'generated-image.png'))
  return path.join(outputDir || process.cwd(), safeName)
}

const DEFAULT_SELECTORS = {
  newSessionButton: '',
  promptInput: '#prompt-textarea[contenteditable="true"], textarea[name="prompt-textarea"]',
  sendButton: 'button[data-testid="send-button"], button[aria-label*="发送"], button.composer-submit-button-color',
  generatedImage: 'img[src*="/backend-api/estuary/content"], img[alt*="已生成图片"], img[src*="oaiusercontent"], img[src*="oaidalleapiprodscus"], img[alt*="生成"], img[alt*="image" i]',
  downloadButton: '',
}

function normalizeSelectors(value) {
  const selectors = value && typeof value === 'object' ? value : {}
  return {
    newSessionButton: normalizeText(selectors.newSessionButton),
    promptInput: normalizeText(selectors.promptInput) || DEFAULT_SELECTORS.promptInput,
    sendButton: normalizeText(selectors.sendButton) || DEFAULT_SELECTORS.sendButton,
    generatedImage: normalizeText(selectors.generatedImage) || DEFAULT_SELECTORS.generatedImage,
    downloadButton: normalizeText(selectors.downloadButton),
  }
}

function buildMissingSetup(selectors, pageUrl) {
  const missing = []
  if (!pageUrl) {
    missing.push('pageUrl')
  }
  for (const key of ['promptInput', 'generatedImage']) {
    if (!normalizeText(selectors[key])) {
      missing.push('selectors.' + key)
    }
  }
  return missing
}

async function detectLoginRequired(page) {
  const url = page.url()
  if (/auth\.openai\.com|\/auth\/login|email-verification/i.test(url)) {
    return { url, reason: 'auth_page' }
  }
  const bodyText = await page.locator('body').innerText({ timeout: 2000 }).catch(() => '')
  if (/requires you to be logged in|需要登录|登录以获取|登录以|log in|sign in|免费注册/i.test(bodyText)) {
    return { url, reason: 'login_required_text' }
  }
  return null
}

async function firstVisibleLocator(page, selector, timeoutMs, label) {
  const locator = page.locator(selector)
  const deadline = Date.now() + timeoutMs
  let lastCount = 0
  while (Date.now() < deadline) {
    lastCount = await locator.count().catch(() => 0)
    for (let index = 0; index < lastCount; index += 1) {
      const candidate = locator.nth(index)
      if (await candidate.isVisible().catch(() => false)) {
        return candidate
      }
    }
    await page.waitForTimeout(250)
  }
  throw new Error(`${label || 'selector'} 未找到可见元素：${selector}，匹配数量 ${lastCount}`)
}

async function clickWhenReady(page, selector, timeoutMs, label) {
  const locator = await firstVisibleLocator(page, selector, timeoutMs, label)
  await locator.click({ timeout: timeoutMs })
  return { step: label, selector }
}

async function fillPrompt(page, selector, prompt, timeoutMs) {
  const locator = await firstVisibleLocator(page, selector, timeoutMs, 'promptInput')
  const tagName = await locator.evaluate((element) => element.tagName.toLowerCase())
  const isContentEditable = await locator.evaluate((element) => element.isContentEditable)
  await locator.click({ timeout: timeoutMs })
  if (tagName === 'textarea' || tagName === 'input') {
    await locator.fill(prompt, { timeout: timeoutMs })
  } else if (isContentEditable) {
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+A' : 'Control+A')
    await page.keyboard.type(prompt, { delay: 8 })
  } else {
    await locator.fill(prompt, { timeout: timeoutMs })
  }
  return { step: 'input_prompt', selector, promptLength: prompt.length, inputMode: isContentEditable ? 'contenteditable' : tagName }
}

async function submitPrompt(page, selector, timeoutMs) {
  const normalizedSelector = normalizeText(selector)
  if (normalizedSelector) {
    const locator = await firstVisibleLocator(page, normalizedSelector, Math.min(timeoutMs, 15000), 'sendButton')
    await locator.click({ timeout: Math.min(timeoutMs, 15000) })
    return { step: 'submit_prompt', selector: normalizedSelector, submitMode: 'button' }
  }

  await page.keyboard.press('Enter')
  return { step: 'submit_prompt', selector: '', submitMode: 'keyboard-enter' }
}

async function collectImageSources(page, selector) {
  const locator = page.locator(selector)
  const count = await locator.count().catch(() => 0)
  const sources = new Set()
  for (let index = 0; index < count; index += 1) {
    const source = await locator.nth(index).evaluate((element) => {
      if (element.tagName.toLowerCase() !== 'img') {
        return ''
      }
      return element.currentSrc || element.src || ''
    }).catch(() => '')
    if (source) {
      sources.add(source)
    }
  }
  return sources
}

async function findFreshVisibleImage(page, selector, existingSources) {
  const locator = page.locator(selector)
  const count = await locator.count().catch(() => 0)
  for (let index = 0; index < count; index += 1) {
    const candidate = locator.nth(index)
    if (!(await candidate.isVisible().catch(() => false))) {
      continue
    }
    const imageInfo = await candidate.evaluate((element) => {
      const tagName = element.tagName.toLowerCase()
      const src = tagName === 'img' ? element.currentSrc || element.src || '' : ''
      const rect = element.getBoundingClientRect()
      return {
        tagName,
        src,
        width: Math.round(rect.width),
        height: Math.round(rect.height),
      }
    }).catch(() => null)
    if (!imageInfo || !imageInfo.src || existingSources.has(imageInfo.src)) {
      continue
    }
    const marker = `ant-gateway-generated-${Date.now()}-${index}`
    await candidate.evaluate((element, value) => {
      element.setAttribute('data-ant-gateway-image-candidate', value)
    }, marker).catch(() => {})
    return { ...imageInfo, marker }
  }
  return null
}

async function waitForGeneratedImage(page, selector, timeoutMs, onProgress, existingSources = new Set()) {
  const locator = page.locator(selector)
  const deadline = Date.now() + timeoutMs
  const startedAt = Date.now()
  let lastProgressAt = 0
  const loginRequiredPattern = /requires you to be logged in|需要登录|登录以获取|登录以|log in|sign in/i
  while (Date.now() < deadline) {
    const matchedCount = await locator.count().catch(() => 0)
    const imageInfo = await findFreshVisibleImage(page, selector, existingSources)
    if (imageInfo) {
      return { step: 'wait_image', selector, imageInfo }
    }
    const now = Date.now()
    if (typeof onProgress === 'function' && now - lastProgressAt >= 10000) {
      lastProgressAt = now
      onProgress({
        elapsedMs: now - startedAt,
        remainingMs: Math.max(0, deadline - now),
        matchedCount,
        firstVisible: false,
      })
    }
    const bodyText = await page.locator('body').innerText({ timeout: 1000 }).catch(() => '')
    if (loginRequiredPattern.test(bodyText)) {
      return {
        step: 'wait_image',
        ok: false,
        status: 'needs_login',
        summary: 'ChatGPT 当前未登录，页面拒绝生成图片。',
      }
    }
    await page.waitForTimeout(1500)
  }
  return {
    step: 'wait_image',
    selector,
    ok: false,
    status: 'timeout',
    summary: '等待新的图片生成结果超时。',
  }
}

function fileSize(pathname) {
  try {
    return fs.statSync(pathname).size
  } catch {
    return 0
  }
}

async function downloadByButton(page, selector, timeoutMs) {
  const downloadPromise = page.waitForEvent('download', { timeout: timeoutMs })
  await page.locator(selector).first().click({ timeout: timeoutMs })
  return downloadPromise
}

async function downloadByImageURL(page, imageSelector, timeoutMs, outputPath, candidateMarker) {
  const selector = normalizeText(candidateMarker)
    ? `[data-ant-gateway-image-candidate="${candidateMarker}"]`
    : imageSelector
  const locator = await firstVisibleLocator(page, selector, timeoutMs, 'generatedImage')
  const imageUrl = await locator.evaluate((element) => {
    if (element.tagName.toLowerCase() !== 'img') {
      return ''
    }
    return element.currentSrc || element.src || ''
  })
  if (!imageUrl) {
    throw new Error('已找到生成结果，但没有可下载的图片 URL。请补充 selectors.downloadButton 或确认 generatedImage 指向 img。')
  }

  const result = await page.evaluate(async (targetUrl) => {
    const response = await fetch(targetUrl, { credentials: 'include' })
    if (!response.ok) {
      return {
        ok: false,
        status: response.status,
        statusText: response.statusText,
        contentType: response.headers.get('content-type') || '',
        bytes: [],
      }
    }
    const arrayBuffer = await response.arrayBuffer()
    return {
      ok: true,
      status: response.status,
      statusText: response.statusText,
      contentType: response.headers.get('content-type') || '',
      bytes: Array.from(new Uint8Array(arrayBuffer)),
    }
  }, imageUrl)
  if (!result.ok || result.bytes.length === 0) {
    throw new Error(`图片 URL 下载失败：HTTP ${result.status || 'unknown'} ${result.statusText || ''}`.trim())
  }
  const buffer = Buffer.from(result.bytes)
  fs.writeFileSync(outputPath, buffer)
  return { source: 'image-url', url: imageUrl, contentType: result.contentType, bytes: buffer.length }
}

async function captureScreenshotIfNeeded(page, enabled, outputDir, label) {
  if (!enabled) {
    return ''
  }
  const screenshotPath = path.join(outputDir || process.cwd(), `web-image-generate-download-${label}.png`)
  await page.screenshot({ path: screenshotPath, fullPage: true }).catch(() => {})
  return screenshotPath
}

exports.run = async function run({ useBrowser, params = {}, artifact, artifactsDir, log }) {
  const pageUrl = normalizeText(params.pageUrl || params.url) || 'https://chatgpt.com/'
  const prompt = normalizeText(params.prompt) || 'A cinematic chrome ant browser mascot, premium product lighting'
  const selectors = normalizeSelectors(params.selectors)
  const timeoutMs = normalizeInt(params.timeoutMs, 300000, 5000, 900000)
  const waitAfterLoadMs = normalizeInt(params.waitAfterLoadMs, 1200, 0, 30000)
  const settleMs = normalizeInt(params.settleMs, 2500, 0, 60000)
  const captureScreenshot = Boolean(params.captureScreenshot)
  const outputFileName = ensurePngExtension(sanitizeFileName(params.outputFileName, 'generated-image.png'))
  const outputPath = typeof artifact === 'function'
    ? artifact(outputFileName)
    : resolveOutputPath(artifactsDir, outputFileName)
  const steps = []
  const writeLog = (event, details = {}) => {
    if (typeof log === 'function') {
      log(`web-image-generate-download:${event}`, details)
    }
  }

  const missing = buildMissingSetup(selectors, pageUrl)
  if (missing.length > 0) {
    writeLog('setup-missing', { missing, pageUrl })
    return {
      ok: false,
      status: 'needs_page_info',
      summary: '网页图片生成缺少必要页面配置。',
      missing,
      expectedFlow: [
        'open_page',
        'create_new_session',
        'send_image_prompt',
        'wait_image_generated',
        'download_image',
      ],
    }
  }

  writeLog('start', {
    pageUrl,
    promptLength: prompt.length,
    outputFileName,
    timeoutMs,
    waitAfterLoadMs,
    settleMs,
    captureScreenshot,
    hasDownloadButton: Boolean(normalizeText(selectors.downloadButton)),
    selectors: {
      hasNewSessionButton: Boolean(normalizeText(selectors.newSessionButton)),
      hasPromptInput: Boolean(normalizeText(selectors.promptInput)),
      hasSendButton: Boolean(normalizeText(selectors.sendButton)),
      hasGeneratedImage: Boolean(normalizeText(selectors.generatedImage)),
      hasDownloadButton: Boolean(normalizeText(selectors.downloadButton)),
    },
  })

  if (typeof useBrowser !== 'function') {
    throw new Error('automation runtime does not provide useBrowser')
  }

  writeLog('open-page:start', { pageUrl, waitUntil: 'domcontentloaded', reuseCurrentPage: true })
  const { page } = await useBrowser({
    url: pageUrl,
    waitUntil: 'domcontentloaded',
    timeoutMs,
    reuseCurrentPage: true,
    bringToFront: true,
  })
  if (waitAfterLoadMs > 0) {
    writeLog('open-page:wait-after-load', { waitAfterLoadMs })
    await page.waitForTimeout(waitAfterLoadMs)
  }
  steps.push({ step: 'open_page', url: pageUrl, currentUrl: page.url() })
  writeLog('open-page:done', { requestedUrl: pageUrl, currentUrl: page.url() })

  writeLog('login-check:start', { currentUrl: page.url() })
  const loginRequired = await detectLoginRequired(page)
  if (loginRequired) {
    writeLog('login-check:failed', loginRequired)
    const screenshotPath = await captureScreenshotIfNeeded(page, true, path.dirname(outputPath), 'needs-login')
    writeLog('screenshot:capture', { reason: 'needs_login', screenshotPath })
    return {
      ok: false,
      status: 'needs_login',
      summary: '目标实例未登录 ChatGPT，无法生成图片。',
      screenshotPath,
      steps: [...steps, { step: 'check_login', ok: false, ...loginRequired }],
    }
  }
  writeLog('login-check:passed', { currentUrl: page.url() })

  if (normalizeText(selectors.newSessionButton)) {
    writeLog('new-session:start', { selector: selectors.newSessionButton })
    const step = await clickWhenReady(page, selectors.newSessionButton, timeoutMs, 'create_new_session')
    steps.push(step)
    writeLog('new-session:done', step)
  } else {
    const step = { step: 'create_new_session', skipped: true, reason: 'selectors.newSessionButton is empty' }
    steps.push(step)
    writeLog('new-session:skipped', step)
  }

  const existingImageSources = await collectImageSources(page, selectors.generatedImage)

  writeLog('prompt-input:start', { selector: selectors.promptInput, promptLength: prompt.length })
  const promptStep = await fillPrompt(page, selectors.promptInput, prompt, timeoutMs)
  steps.push(promptStep)
  writeLog('prompt-input:done', promptStep)

  writeLog('submit:start', { selector: selectors.sendButton || '', mode: normalizeText(selectors.sendButton) ? 'button' : 'keyboard-enter' })
  const submitStep = await submitPrompt(page, selectors.sendButton, timeoutMs)
  steps.push(submitStep)
  writeLog('submit:done', submitStep)

  writeLog('wait-image:start', { selector: selectors.generatedImage, timeoutMs })
  const generatedImageStep = await waitForGeneratedImage(page, selectors.generatedImage, timeoutMs, (progress) => {
    writeLog('wait-image:progress', progress)
  }, existingImageSources)
  steps.push(generatedImageStep)
  writeLog('wait-image:done', generatedImageStep)
  if (generatedImageStep && generatedImageStep.ok === false) {
    writeLog('wait-image:failed', generatedImageStep)
    const screenshotPath = await captureScreenshotIfNeeded(page, true, path.dirname(outputPath), generatedImageStep.status || 'failed')
    writeLog('screenshot:capture', { reason: generatedImageStep.status || 'failed', screenshotPath })
    return {
      ok: false,
      status: generatedImageStep.status || 'failed',
      summary: generatedImageStep.summary || '图片未生成。',
      screenshotPath,
      steps,
    }
  }

  if (settleMs > 0) {
    writeLog('settle:start', { settleMs })
    await page.waitForTimeout(settleMs)
    writeLog('settle:done', { settleMs })
  }

  let downloadInfo
  if (normalizeText(selectors.downloadButton)) {
    writeLog('download:start', { mode: 'download-button', selector: selectors.downloadButton, outputPath })
    const download = await downloadByButton(page, selectors.downloadButton, timeoutMs)
    await download.saveAs(outputPath)
    downloadInfo = {
      source: 'download-button',
      suggestedFilename: download.suggestedFilename(),
    }
  } else {
    writeLog('download:start', { mode: 'image-url', selector: selectors.generatedImage, outputPath })
    downloadInfo = await downloadByImageURL(page, selectors.generatedImage, timeoutMs, outputPath, generatedImageStep.imageInfo && generatedImageStep.imageInfo.marker)
  }
  const downloadStep = { step: 'download_image', outputPath, bytes: fileSize(outputPath), ...downloadInfo }
  steps.push(downloadStep)
  writeLog('download:done', downloadStep)

  writeLog('screenshot:optional-start', { enabled: captureScreenshot })
  const screenshotPath = await captureScreenshotIfNeeded(page, captureScreenshot, path.dirname(outputPath), 'done')
  writeLog('screenshot:optional-done', { enabled: captureScreenshot, screenshotPath })
  writeLog('completed', { outputPath, bytes: fileSize(outputPath), screenshotPath, stepCount: steps.length })

  return {
    ok: true,
    status: 'completed',
    summary: '图片已生成并下载。',
    outputPath,
    downloadAddress: outputPath,
    screenshotPath,
    steps,
  }
}
