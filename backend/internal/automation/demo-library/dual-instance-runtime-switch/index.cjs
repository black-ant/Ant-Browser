export async function run({ baseUrl, apiKey, params, log }) {
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
            ["BUYER_001", "BUYER_002"][index] || "",
            ["https://finance.sina.com.cn/", "https://map.baidu.com/"][index] || [],
            defaultSkipDefaultStartUrls,
          ),
        )
        .filter(Boolean)
    : []

  if (browsers.length === 0) {
    browsers = [
      normalizeBrowserInput(
        { code: params.primaryCode, skipDefaultStartUrls: params.skipDefaultStartUrls },
        "BUYER_001",
        ["https://finance.sina.com.cn/"],
        defaultSkipDefaultStartUrls,
      ),
      normalizeBrowserInput(
        { code: params.secondaryCode, skipDefaultStartUrls: params.skipDefaultStartUrls },
        "BUYER_002",
        ["https://map.baidu.com/"],
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

  const post = async (requestPath, payload) => {
    const response = await fetch(`${baseUrl}${requestPath}`, {
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
      throw new Error(`${requestPath} failed: ${response.status} ${text}`)
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
    summary: `${browserCodes.length} 个浏览器已就绪：${browserCodes.join(" / ")}`,
    browserCodes,
    sessions,
  }
}
