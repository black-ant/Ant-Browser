const fs = require('fs');
const path = require('path');
const {
  normalizeTimeout,
  sleep,
  writeStream,
  closeBrowserConnection,
  buildConnectEndpoints,
  normalizePathUnderRoot,
  requestJSON,
  toSerializable,
} = require('./runner_shared.cjs');
const { normalizeOrigin, normalizePermissionList, normalizePageAPIRequest, executePageAPIRequest } = require('./runner_page_api.cjs');
const { loadScriptModule } = require('./runner_script_loader.cjs');

const ALLOWED_WAIT_UNTIL = new Set(['load', 'domcontentloaded', 'networkidle', 'commit']);

function getPageURL(page) {
  if (!page || typeof page.url !== 'function') {
    return '';
  }
  try {
    return String(page.url() || '').trim();
  } catch {
    return '';
  }
}

function normalizeComparableURL(value) {
  const text = String(value || '').trim();
  if (!text) {
    return '';
  }
  if (text === 'about:blank') {
    return text;
  }
  try {
    return new URL(text).toString();
  } catch {
    return text;
  }
}

function shouldReuseExistingPageByDefault(page, targetURL) {
  const currentURL = normalizeComparableURL(getPageURL(page));
  if (!currentURL || currentURL === 'about:blank') {
    return true;
  }
  const nextURL = normalizeComparableURL(targetURL);
  return nextURL !== '' && currentURL === nextURL;
}

function hasOpenPageIntent(options) {
  const openOptions = options && typeof options === 'object' && !Array.isArray(options) ? options : {};
  if (String(openOptions.url || '').trim()) {
    return true;
  }
  if (openOptions.permissions !== undefined) {
    return true;
  }
  if (typeof openOptions.permissionOrigin === 'string' && openOptions.permissionOrigin.trim()) {
    return true;
  }
  if (openOptions.reuseCurrentPage === true || openOptions.bringToFront === true) {
    return true;
  }
  return false;
}

function buildLaunchRequestBody(defaultSelector, options) {
  const launchOptions = options && typeof options === 'object' ? options : {};
  const body = {};

  for (const key of [
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
  ]) {
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

async function runScriptTask(payload, chromium) {
  const scriptModule = await loadScriptModule(payload.scriptPath);
  if (!scriptModule || typeof scriptModule.run !== 'function') {
    throw new Error('script must export run()');
  }

  const logs = [];
  const artifacts = [];
  const connectedBrowsers = new Set();
  const selector = payload.selector && typeof payload.selector === 'object' ? payload.selector : {};
  const params = payload.params && typeof payload.params === 'object' ? payload.params : {};
  const timeout = normalizeTimeout(params.timeoutMs, 30000);
  const startedAt = new Date().toISOString();

  const log = (...entries) => {
    logs.push({
      time: new Date().toISOString(),
      values: entries.map((entry) => toSerializable(entry)),
    });
  };

  const artifact = (name) => {
    const fileName = String(name || '').trim() || `artifact-${Date.now()}`;
    const targetPath = normalizePathUnderRoot(payload.artifactDir, fileName);
    fs.mkdirSync(path.dirname(targetPath), { recursive: true });
    artifacts.push(targetPath);
    return targetPath;
  };

  const launchHeaders = {};
  if (payload.launchAuthHeader && payload.launchAuthValue) {
    launchHeaders[payload.launchAuthHeader] = payload.launchAuthValue;
  }

  const launch = async (options = {}) => {
    const body = buildLaunchRequestBody(selector, options);

    const response = await requestJSON(
      'POST',
      `${String(payload.launchBaseUrl || '').replace(/\/$/, '')}/api/launch`,
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

  const connect = async (session = {}, options = {}) => {
    const connectOptions =
      options && typeof options === 'object' && !Array.isArray(options) ? options : {};
    const endpoints = buildConnectEndpoints(payload, session);
    if (endpoints.length === 0) {
      throw new Error(
        `launch session does not contain a valid cdp endpoint (cdpUrl=${String(
          session && session.cdpUrl ? session.cdpUrl : ''
        )}, debugPort=${String(session && session.debugPort ? session.debugPort : '')})`
      );
    }

    const connectTimeout = normalizeTimeout(connectOptions.timeoutMs, timeout);
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

  const grantPermissions = async (target, options = {}) => {
    const permissionOptions =
      options && typeof options === 'object' && !Array.isArray(options) ? options : {};
    const permissions = normalizePermissionList(permissionOptions.permissions);
    const origin = normalizeOrigin(permissionOptions.origin);

    let context = null;
    if (target && typeof target.grantPermissions === 'function') {
      context = target;
    } else if (target && typeof target === 'object') {
      context = target.context || null;
      if (!context && target.browser) {
        const resolved = await resolveConnectionContext(target);
        context = resolved.context;
      }
    }

    if (!context) {
      return {
        applied: false,
        permissions,
        origin,
        reason: 'browser context is unavailable',
      };
    }
    if (!origin) {
      return {
        applied: false,
        permissions,
        origin: '',
        reason: 'origin is required',
      };
    }
    if (permissions.length === 0) {
      return {
        applied: false,
        permissions,
        origin,
        reason: 'permissions are required',
      };
    }
    if (typeof context.grantPermissions !== 'function') {
      return {
        applied: false,
        permissions,
        origin,
        reason: 'grantPermissions is unavailable',
      };
    }

    try {
      await context.grantPermissions(permissions, { origin });
      return {
        applied: true,
        permissions,
        origin,
        strategy: 'grantPermissions',
      };
    } catch (error) {
      return {
        applied: false,
        permissions,
        origin,
        reason: error && error.message ? error.message : String(error),
      };
    }
  };

  const openPage = async (connection, options = {}) => {
    const openOptions =
      options && typeof options === 'object' && !Array.isArray(options) ? options : {};
    const { browser, context } = await resolveConnectionContext(connection);
    const shouldReuseCurrentPage = openOptions.reuseCurrentPage === true;
    const hasReuseCurrentPageOption = Object.prototype.hasOwnProperty.call(
      openOptions,
      'reuseCurrentPage'
    );
    const targetURL = String(openOptions.url || '').trim();

    let page = null;
    const currentPage = connection && connection.page ? connection.page : null;
    if (
      currentPage &&
      typeof currentPage.isClosed === 'function' &&
      !currentPage.isClosed() &&
      (shouldReuseCurrentPage ||
        (!hasReuseCurrentPageOption && shouldReuseExistingPageByDefault(currentPage, targetURL)))
    ) {
      page = currentPage;
    }
    if (!page && targetURL) {
      page = await context.newPage();
    }

    if (page && typeof page.bringToFront === 'function' && openOptions.bringToFront !== false) {
      await page.bringToFront().catch(() => {});
    }

    const permissionResult =
      openOptions.permissions !== undefined
        ? await grantPermissions(context, {
            origin:
              typeof openOptions.permissionOrigin === 'string' && openOptions.permissionOrigin.trim()
                ? openOptions.permissionOrigin
                : openOptions.url,
            permissions: openOptions.permissions,
          })
        : {
            applied: false,
            permissions: [],
            origin: '',
            reason: '',
          };

    if (targetURL) {
      const waitUntil = ALLOWED_WAIT_UNTIL.has(String(openOptions.waitUntil || '').trim())
        ? String(openOptions.waitUntil).trim()
        : 'domcontentloaded';
      await page.goto(targetURL, {
        waitUntil,
        timeout: normalizeTimeout(openOptions.timeoutMs, timeout),
      });
    }

    return {
      browser,
      context,
      page,
      permissionResult,
      reusedPage: page === (connection && connection.page ? connection.page : null),
    };
  };

  const resolvePageTarget = (target) => {
    if (target && typeof target.evaluate === 'function') {
      return target;
    }
    if (target && target.page && typeof target.page.evaluate === 'function') {
      return target.page;
    }
    throw new Error('page api target must be a Playwright page or an object containing page');
  };

  const callPageAPI = async (target, urlOrRequest, options = {}) => {
    const page = resolvePageTarget(target);
    const request = normalizePageAPIRequest(urlOrRequest, options);
    const response = await page.evaluate(executePageAPIRequest, request);

    if (request.throwOnError && (!response || response.ok !== true)) {
      const status = response && response.status ? response.status : 0;
      const message =
        (response && typeof response.error === 'string' && response.error.trim()) ||
        (status ? `page api returned http ${status}` : 'page api request failed');
      throw new Error(message);
    }

    return response;
  };

  const browserFetch = callPageAPI;
  const pageAPI = callPageAPI;

  const useBrowser = async (options = {}) => {
    const runOptions = options && typeof options === 'object' && !Array.isArray(options) ? options : {};
    const launchOptions =
      runOptions.launch && typeof runOptions.launch === 'object' && !Array.isArray(runOptions.launch)
        ? runOptions.launch
        : runOptions;
    const connectOptions =
      runOptions.connect && typeof runOptions.connect === 'object' && !Array.isArray(runOptions.connect)
        ? runOptions.connect
        : {};
    const openOptions =
      runOptions.open && typeof runOptions.open === 'object' && !Array.isArray(runOptions.open)
        ? runOptions.open
        : {
            url: runOptions.url,
            waitUntil: runOptions.waitUntil,
            timeoutMs: runOptions.timeoutMs,
            permissions: runOptions.permissions,
            permissionOrigin: runOptions.permissionOrigin,
            reuseCurrentPage: runOptions.reuseCurrentPage,
            bringToFront: runOptions.bringToFront,
          };

    const session = await launch(launchOptions);
    const connection = await connect(session, connectOptions);
    const opened = hasOpenPageIntent(openOptions)
      ? await openPage(connection, openOptions)
      : {
          browser: connection.browser,
          context: connection.context,
          page: connection.page || null,
          permissionResult: {
            applied: false,
            permissions: [],
            origin: '',
            reason: '',
          },
          reusedPage: Boolean(connection.page),
        };
    return {
      session,
      connection,
      ...opened,
    };
  };

  const api = {
    chromium,
    launch,
    connect,
    grantPermissions,
    openPage,
    useBrowser,
    callPageAPI,
    pageAPI,
    browserFetch,
    selector,
    params,
    log,
    artifact,
    artifactsDir: payload.artifactDir || '',
  };

  try {
    const rawResult = await scriptModule.run(api);
    const normalizedResult = toSerializable(rawResult);
    const ok = !(normalizedResult && typeof normalizedResult === 'object' && normalizedResult.ok === false);
    const summary =
      normalizedResult &&
      typeof normalizedResult === 'object' &&
      typeof normalizedResult.summary === 'string'
        ? normalizedResult.summary.trim()
        : ok
          ? '脚本执行完成'
          : '脚本执行失败';
    const error =
      normalizedResult &&
      typeof normalizedResult === 'object' &&
      typeof normalizedResult.error === 'string'
        ? normalizedResult.error.trim()
        : '';

    return {
      ok,
      summary,
      error,
      title:
        normalizedResult &&
        typeof normalizedResult === 'object' &&
        typeof normalizedResult.title === 'string'
          ? normalizedResult.title
          : '',
      url:
        normalizedResult &&
        typeof normalizedResult === 'object' &&
        typeof normalizedResult.url === 'string'
          ? normalizedResult.url
          : '',
      startedAt,
      finishedAt: new Date().toISOString(),
      isolatedPage: false,
      logs,
      artifacts: Array.from(new Set(artifacts)),
      result: normalizedResult,
    };
  } catch (error) {
    return {
      ok: false,
      summary: '脚本执行失败',
      error: error && error.message ? error.message : String(error),
      title: '',
      url: '',
      startedAt,
      finishedAt: new Date().toISOString(),
      isolatedPage: false,
      logs,
      artifacts: Array.from(new Set(artifacts)),
      result: null,
    };
  } finally {
    await Promise.all(Array.from(connectedBrowsers, (browser) => closeBrowserConnection(browser)));
  }
}

async function main() {
  const payloadPath = process.argv[2];
  if (!payloadPath) {
    throw new Error('payload path is required');
  }

  const payload = JSON.parse(fs.readFileSync(payloadPath, 'utf8'));
  const runtimeDir = path.resolve(String(payload.runtimeDir || ''));
  if (!runtimeDir) {
    throw new Error('runtimeDir is required');
  }

  const { chromium } = require(path.join(runtimeDir, 'node_modules', 'playwright-core'));
  const taskType = String(payload.taskType || 'script').trim() || 'script';
  if (taskType !== 'script') {
    throw new Error(`unsupported automation task type: ${taskType}`);
  }

  const result = await runScriptTask(payload, chromium);
  await writeStream(process.stdout, JSON.stringify(result));
  process.exit(0);
}

main().catch(async (error) => {
  const message = error && error.message ? error.message : String(error);
  try {
    await writeStream(process.stderr, message);
  } finally {
    process.exit(1);
  }
});
