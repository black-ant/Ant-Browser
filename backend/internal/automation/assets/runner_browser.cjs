const { normalizeTimeout, sleep, closeBrowserConnection, buildConnectEndpoints, requestJSON } = require('./runner_shared.cjs');

// 浏览器接入网关：把「调 Launch API 起实例」和「connectOverCDP 接管」这两步
// 收敛到一处，供 script 任务和常驻 page session 共用。
//
// 抽出来的理由：两种任务类型必须用完全相同的接入语义（同样的候选端点顺序、
// 同样的重试窗口），否则脚本能连上、会话连不上这类问题会非常难查。

const LAUNCH_BODY_KEYS = [
  'code',
  'key',
  'profileId',
  'profileName',
  'keyword',
  'keywords',
  'tag',
  'tags',
  'groupId',
  'matchMode',
  'proxyId',
  'proxyConfig',
  'launchArgs',
  'startUrls',
  'skipDefaultStartUrls',
];

function buildLaunchRequestBody(defaultSelector, options) {
  const launchOptions = options && typeof options === 'object' ? options : {};
  const body = {};

  for (const key of LAUNCH_BODY_KEYS) {
    if (Object.prototype.hasOwnProperty.call(launchOptions, key)) {
      body[key] = launchOptions[key];
    }
  }

  const selector =
    launchOptions.selector &&
    typeof launchOptions.selector === 'object' &&
    !Array.isArray(launchOptions.selector)
      ? launchOptions.selector
      : defaultSelector;
  if (selector && typeof selector === 'object' && !Array.isArray(selector) && Object.keys(selector).length > 0) {
    body.selector = selector;
  }

  if (!Object.prototype.hasOwnProperty.call(body, 'skipDefaultStartUrls')) {
    body.skipDefaultStartUrls = true;
  }

  return body;
}

// createBrowserGateway 返回一组共享同一份连接登记表的接入函数。
// defaultTimeout 是 connect 的默认等待窗口，调用方各自决定（脚本取 params.timeoutMs）。
function createBrowserGateway(payload, chromium, options = {}) {
  const defaultTimeout = normalizeTimeout(options.defaultTimeout, 30000);
  const selector = payload && payload.selector && typeof payload.selector === 'object' ? payload.selector : {};
  const connectedBrowsers = new Set();

  const launchHeaders = {};
  if (payload && payload.launchAuthHeader && payload.launchAuthValue) {
    launchHeaders[payload.launchAuthHeader] = payload.launchAuthValue;
  }

  const launch = async (launchOptions = {}) => {
    const body = buildLaunchRequestBody(selector, launchOptions);

    const response = await requestJSON(
      'POST',
      `${String((payload && payload.launchBaseUrl) || '').replace(/\/$/, '')}/api/launch`,
      body,
      launchHeaders
    );

    if (!(response.status >= 200 && response.status < 300) || response.body.ok === false) {
      const errorText =
        (response.body && response.body.error && String(response.body.error).trim()) ||
        `launch api returned http ${response.status}`;
      throw new Error(errorText);
    }

    return response.body;
  };

  const connect = async (session = {}, connectOptions = {}) => {
    const normalizedOptions =
      connectOptions && typeof connectOptions === 'object' && !Array.isArray(connectOptions)
        ? connectOptions
        : {};
    const endpoints = buildConnectEndpoints(payload, session);
    if (endpoints.length === 0) {
      throw new Error(
        `launch session does not contain a valid cdp endpoint (cdpUrl=${String(
          session && session.cdpUrl ? session.cdpUrl : ''
        )}, debugPort=${String(session && session.debugPort ? session.debugPort : '')})`
      );
    }

    const connectTimeout = normalizeTimeout(normalizedOptions.timeoutMs, defaultTimeout);
    const deadline = Date.now() + connectTimeout;
    let lastError = null;

    while (Date.now() <= deadline) {
      for (const endpoint of endpoints) {
        const remaining = deadline - Date.now();
        if (remaining <= 0) {
          break;
        }

        try {
          const browser = await chromium.connectOverCDP(endpoint, {
            timeout: Math.max(1000, Math.min(remaining, connectTimeout)),
          });
          connectedBrowsers.add(browser);
          const context = browser.contexts()[0] || null;
          const page = context && context.pages().length > 0 ? context.pages()[0] : null;
          return {
            browser,
            context,
            page,
            session: {
              ...session,
              cdpUrl: endpoint,
            },
          };
        } catch (error) {
          lastError = error;
        }
      }

      if (Date.now() >= deadline) {
        break;
      }

      await sleep(Math.min(500, Math.max(100, deadline - Date.now())));
    }

    const lastMessage =
      lastError && lastError.message ? lastError.message : String(lastError || 'unknown error');
    throw new Error(
      `cdp endpoint is not ready after ${connectTimeout} ms (endpoints: ${endpoints.join(', ')}): ${lastMessage}`
    );
  };

  const resolveConnectionContext = async (connection) => {
    const browser = connection && connection.browser ? connection.browser : null;
    if (!browser) {
      throw new Error('browser connection is unavailable');
    }

    const context =
      connection.context ||
      browser.contexts()[0] ||
      (typeof browser.newContext === 'function' ? await browser.newContext() : null);
    if (!context) {
      throw new Error('browser context is unavailable');
    }

    return {
      browser,
      context,
    };
  };

  const closeAll = async () => {
    await Promise.all(Array.from(connectedBrowsers, (browser) => closeBrowserConnection(browser)));
    connectedBrowsers.clear();
  };

  return {
    selector,
    launch,
    connect,
    resolveConnectionContext,
    connectedBrowsers,
    closeAll,
  };
}

module.exports = {
  buildLaunchRequestBody,
  createBrowserGateway,
};
