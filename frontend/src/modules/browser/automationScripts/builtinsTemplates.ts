import type { AutomationScriptType } from './definitions'
import { normalizeAutomationScriptTargetSelector } from './targets'

const DUAL_INSTANCE_DEFAULT_CODES = ["BUYER_001", "BUYER_002"] as const;
const DUAL_INSTANCE_DEFAULT_START_URLS = [
  "https://finance.sina.com.cn/",
  "https://map.baidu.com/",
] as const;

export function buildSelectorTemplate(type: AutomationScriptType): string {
  if (type === "launch-api") {
    return `{
  "code": "BUYER_001"
}`;
  }

  return "";
}

export function buildParamsTemplate(type: AutomationScriptType): string {
  if (type === "launch-api") {
    return `{
  "startUrls": ["https://example.com"],
  "skipDefaultStartUrls": true
}`;
  }

  return `{
  "url": "https://www.baidu.com",
  "keyword": "OpenAI",
  "timeoutMs": 30000,
  "waitAfterSearchMs": 1500,
  "captureScreenshot": true
}`;
}

export function buildScriptTemplate(type: AutomationScriptType): string {
  if (type === "launch-api") {
    return `export async function run({ baseUrl, apiKey, selector, params }) {
  const response = await fetch(\`\${baseUrl}/api/launch\`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(apiKey ? { 'X-Ant-Api-Key': apiKey } : {}),
    },
    body: JSON.stringify({
      selector,
      ...(params || {}),
    }),
  })

  if (!response.ok) {
    throw new Error(\`launch failed: \${response.status}\`)
  }

  return await response.json()
}`;
  }

  return `module.exports.run = async ({ useBrowser, browserFetch, selector, params, log, artifact }) => {
  const targetUrl =
    typeof params.url === 'string' && params.url.trim()
      ? params.url.trim()
      : 'https://www.baidu.com'
  const keyword =
    typeof params.keyword === 'string' && params.keyword.trim()
      ? params.keyword.trim()
      : 'OpenAI'
  const timeout =
    Number.isFinite(Number(params.timeoutMs)) && Number(params.timeoutMs) > 0
      ? Math.round(Number(params.timeoutMs))
      : 30000
  const waitAfterSearchMs =
    Number.isFinite(Number(params.waitAfterSearchMs)) && Number(params.waitAfterSearchMs) >= 0
      ? Math.round(Number(params.waitAfterSearchMs))
      : 1500

  const runtime = await useBrowser({
    selector,
    startUrls: params.startUrls || [targetUrl],
    skipDefaultStartUrls: true,
    url: targetUrl,
    timeoutMs: timeout,
    reuseCurrentPage: true,
  })
  const page = runtime.page

  const searchInput = page.locator('textarea[name="wd"], input[name="wd"]').first()
  await searchInput.waitFor({
    state: 'visible',
    timeout,
  })
  await searchInput.fill(keyword)
  await searchInput.press('Enter').catch(async () => {
    const submitButton = page.locator('#su, input[type="submit"]').first()
    await submitButton.click({ timeout })
  })
  await page.waitForURL(/wd=/, { timeout }).catch(() => {})

  if (waitAfterSearchMs > 0) {
    await page.waitForTimeout(waitAfterSearchMs)
  }

  if (params.captureScreenshot !== false) {
    await page.screenshot({
      path: artifact('baidu-search.png'),
      fullPage: true,
    })
  }

  const title = await page.title()
  let apiResult = null
  const apiUrl = typeof params.apiUrl === 'string' ? params.apiUrl.trim() : ''
  if (apiUrl) {
    const apiRequest = {
      url: apiUrl,
      method: params.apiBody === undefined ? 'GET' : 'POST',
      timeoutMs: timeout,
    }
    if (params.apiBody !== undefined) {
      apiRequest.json = params.apiBody
    }
    apiResult = await browserFetch(page, apiRequest)
  }
  log('keyword', keyword)
  log('title', title)

  return {
    ok: true,
    summary: \`已在百度搜索 \${keyword}\`,
    keyword,
    url: page.url(),
    title,
    apiResult,
  }
}`;
}

export function buildNotesTemplate(type: AutomationScriptType): string {
  if (type === "launch-api") {
    return "适合外部调度器或 HTTP 中台。脚本负责组装 selector 和 launch 参数，不直接接管页面。";
  }

  return "默认示例使用 useBrowser 启动并接管页面；需要调用站内接口时传 apiUrl/apiBody，会通过 browserFetch 在浏览器上下文发起请求。";
}

export function buildDualInstanceRuntimeParamsText(
  codes = [...DUAL_INSTANCE_DEFAULT_CODES],
): string {
  return `{
  "browsers": [
    {
      "code": "${codes[0] || DUAL_INSTANCE_DEFAULT_CODES[0]}",
      "skipDefaultStartUrls": true,
      "startUrls": ["${DUAL_INSTANCE_DEFAULT_START_URLS[0]}"]
    },
    {
      "code": "${codes[1] || DUAL_INSTANCE_DEFAULT_CODES[1]}",
      "skipDefaultStartUrls": true,
      "startUrls": ["${DUAL_INSTANCE_DEFAULT_START_URLS[1]}"]
    }
  ],
  "timeoutMs": 45000
}`;
}

export function buildDualInstanceRuntimeScriptText(): string {
  return `export async function run({ baseUrl, apiKey, params, log }) {
  const normalizeCode = (value, fallback) =>
    String(value || fallback || "").trim().toUpperCase()
  const normalizeStringArray = (value) =>
    Array.isArray(value)
      ? value
          .map((item) => String(item || "").trim())
          .filter(Boolean)
      : []
  const normalizeBrowserInput = (value, fallbackCode, fallbackStartUrls, defaultSkip) => {
    const raw = value && typeof value === "object" ? value : {}
    const code = normalizeCode(raw.code || raw.launchCode, fallbackCode)
    if (!code) {
      return null
    }
    const startUrls = normalizeStringArray(raw.startUrls)
    const fallbackUrls = normalizeStringArray(fallbackStartUrls)
    const launchArgs = normalizeStringArray(raw.launchArgs)

    return {
      code,
      skipDefaultStartUrls:
        raw.skipDefaultStartUrls !== undefined
          ? raw.skipDefaultStartUrls !== false
          : defaultSkip,
      startUrls: startUrls.length > 0 ? startUrls : fallbackUrls,
      launchArgs,
    }
  }

  const timeoutMs = Number.isFinite(Number(params.timeoutMs))
    ? Math.max(1000, Math.round(Number(params.timeoutMs)))
    : 45000
  const defaultSkipDefaultStartUrls = params.skipDefaultStartUrls !== false

  let browsers = Array.isArray(params.browsers)
    ? params.browsers
        .map((item, index) =>
          normalizeBrowserInput(
            item,
            ${JSON.stringify([...DUAL_INSTANCE_DEFAULT_CODES])}[index] || "",
            ${JSON.stringify([...DUAL_INSTANCE_DEFAULT_START_URLS])}[index] || [],
            defaultSkipDefaultStartUrls,
          ),
        )
        .filter(Boolean)
    : []

  if (browsers.length === 0) {
    browsers = [
      normalizeBrowserInput(
        { code: params.primaryCode, skipDefaultStartUrls: params.skipDefaultStartUrls },
        ${JSON.stringify(DUAL_INSTANCE_DEFAULT_CODES[0])},
        ${JSON.stringify([DUAL_INSTANCE_DEFAULT_START_URLS[0]])},
        defaultSkipDefaultStartUrls,
      ),
      normalizeBrowserInput(
        { code: params.secondaryCode, skipDefaultStartUrls: params.skipDefaultStartUrls },
        ${JSON.stringify(DUAL_INSTANCE_DEFAULT_CODES[1])},
        ${JSON.stringify([DUAL_INSTANCE_DEFAULT_START_URLS[1]])},
        defaultSkipDefaultStartUrls,
      ),
    ].filter(Boolean)
  }

  if (browsers.length === 0) {
    throw new Error("params.browsers 不能为空")
  }

  const headers = {
    "Content-Type": "application/json",
    ...(apiKey ? { "X-Ant-Api-Key": apiKey } : {}),
  }

  const post = async (path, payload) => {
    const response = await fetch(\`\${baseUrl}\${path}\`, {
      method: "POST",
      headers,
      body: JSON.stringify(payload),
    })
    const text = await response.text()
    let body = text
    try {
      body = text ? JSON.parse(text) : null
    } catch {
      body = text
    }
    if (!response.ok) {
      throw new Error(\`\${path} failed: \${response.status} \${text}\`)
    }
    return body
  }

  const sessions = []

  for (const browser of browsers) {
    const sessionResult = await post("/api/runtime/session", {
      selector: { code: browser.code, matchMode: "unique" },
      skipDefaultStartUrls: browser.skipDefaultStartUrls,
      ...(browser.startUrls.length > 0 ? { startUrls: browser.startUrls } : {}),
      ...(browser.launchArgs.length > 0 ? { launchArgs: browser.launchArgs } : {}),
      timeoutMs,
    })

    sessions.push(sessionResult)
  }

  const browserCodes = browsers.map((item) => item.code)
  log("browserCodes", browserCodes)

  return {
    ok: true,
    summary: \`\${browserCodes.length} 个浏览器已就绪：\${browserCodes.join(" / ")}\`,
    browserCodes,
    sessions,
  }
}`;
}

export function normalizeDualInstanceRuntimeParamsText(text: string): string {
  const fallback = buildDualInstanceRuntimeParamsText();

  try {
    const parsed = JSON.parse(text);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return fallback;
    }

    const raw = parsed as Record<string, unknown>;
    const topLevelSkipDefaultStartUrls = raw.skipDefaultStartUrls !== false;
    const rawBrowsers = Array.isArray(raw.browsers) ? raw.browsers : [];
    const browsers = rawBrowsers
      .map((item, index) => {
        if (!item || typeof item !== "object") {
          return null;
        }
        const entry = item as Record<string, unknown>;
        const code = normalizeAutomationScriptTargetSelector({
          code:
            typeof entry.code === "string"
              ? entry.code
              : typeof entry.launchCode === "string"
                ? entry.launchCode
                : "",
        }).code;
        if (!code) {
          return null;
        }

        const startUrls = Array.isArray(entry.startUrls)
          ? entry.startUrls
              .map((value) => String(value || "").trim())
              .filter(Boolean)
          : [];
        const launchArgs = Array.isArray(entry.launchArgs)
          ? entry.launchArgs
              .map((value) => String(value || "").trim())
              .filter(Boolean)
          : [];

        const fallbackStartUrls = DUAL_INSTANCE_DEFAULT_START_URLS[index]
          ? [DUAL_INSTANCE_DEFAULT_START_URLS[index]]
          : [];

        return {
          code: code || DUAL_INSTANCE_DEFAULT_CODES[index] || "",
          skipDefaultStartUrls:
            entry.skipDefaultStartUrls !== undefined
              ? entry.skipDefaultStartUrls !== false
              : topLevelSkipDefaultStartUrls,
          startUrls: startUrls.length > 0 ? startUrls : fallbackStartUrls,
          ...(launchArgs.length > 0 ? { launchArgs } : {}),
        };
      })
      .filter(
        (
          item,
        ): item is {
          code: string;
          skipDefaultStartUrls: boolean;
          startUrls: string[];
          launchArgs?: string[];
        } => item !== null,
      );

    const legacyCodes = [
      normalizeAutomationScriptTargetSelector({
        code: typeof raw.primaryCode === "string" ? raw.primaryCode : "",
      }).code,
      normalizeAutomationScriptTargetSelector({
        code: typeof raw.secondaryCode === "string" ? raw.secondaryCode : "",
      }).code,
    ].filter(Boolean);

    const normalizedBrowsers =
      browsers.length > 0
        ? browsers
        : legacyCodes.length > 0
          ? legacyCodes.map((code, index) => ({
              code,
              skipDefaultStartUrls: topLevelSkipDefaultStartUrls,
              startUrls: DUAL_INSTANCE_DEFAULT_START_URLS[index]
                ? [DUAL_INSTANCE_DEFAULT_START_URLS[index]]
                : [],
            }))
          : DUAL_INSTANCE_DEFAULT_CODES.map((code, index) => ({
              code,
              skipDefaultStartUrls: true,
              startUrls: DUAL_INSTANCE_DEFAULT_START_URLS[index]
                ? [DUAL_INSTANCE_DEFAULT_START_URLS[index]]
                : [],
            }));

    const timeoutMs =
      Number.isFinite(Number(raw.timeoutMs)) && Number(raw.timeoutMs) > 0
        ? Math.round(Number(raw.timeoutMs))
        : 45000;

    return JSON.stringify(
      {
        browsers: normalizedBrowsers,
        timeoutMs,
      },
      null,
      2,
    );
  } catch {
    return fallback;
  }
}
