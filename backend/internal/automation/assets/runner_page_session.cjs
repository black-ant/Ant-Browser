const fs = require('fs');
const path = require('path');
const readline = require('readline');
const { normalizeTimeout, normalizePathUnderRoot, toSerializable } = require('./runner_shared.cjs');
const { createBrowserGateway } = require('./runner_browser.cjs');

// 常驻页面会话。
//
// 与 script 任务的区别：script 是「跑完即退」，这里连上 CDP 之后不退出，
// 转而从 stdin 逐行读取 NDJSON 指令并逐行回写结果。CDP 握手只做一次，
// 后续每条指令的开销降到一次进程间往返。
//
// 协议约定：
//   - stdout 只承载 NDJSON，每行一个对象；诊断信息一律走 stderr
//   - 就绪时先发 {"type":"ready",...}，之后每条指令回 {"id":N,...}
//   - 浏览器断开时发 {"type":"closed",...} 并退出，让 Go 侧回收会话
//   - 截图等大产物写盘，响应里只带路径，避免撑爆管道缓冲

const DEFAULT_TIMEOUT_MS = 30000;
const DEFAULT_SNAPSHOT_LIMIT = 200;
const ALLOWED_LOAD_STATES = new Set(['load', 'domcontentloaded', 'networkidle']);
const ALLOWED_WAIT_UNTIL = new Set(['load', 'domcontentloaded', 'networkidle', 'commit']);
const ALLOWED_ELEMENT_STATES = new Set(['visible', 'hidden', 'attached', 'detached']);

function isPlainObject(value) {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value));
}

// 页面内执行：采集可交互元素。
//
// 刻意不回传整棵 DOM——那对模型既贵又没用。只给「能点能填的东西」，
// 每项附一个可直接回传给 click/fill 的 selector，形成闭环。
function collectInteractiveElements(options) {
  const limit = options && options.limit ? options.limit : 200;
  const INTERACTIVE = 'a,button,input,select,textarea,summary,[role=button],[role=link],[role=checkbox],[role=radio],[role=tab],[role=menuitem],[contenteditable=""],[contenteditable=true],[onclick]';

  const cssEscape = (value) => {
    if (window.CSS && typeof window.CSS.escape === 'function') {
      return window.CSS.escape(value);
    }
    return String(value).replace(/[^a-zA-Z0-9_-]/g, (ch) => `\\${ch}`);
  };

  const isVisible = (el) => {
    const rect = el.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) {
      return false;
    }
    const style = window.getComputedStyle(el);
    return style.visibility !== 'hidden' && style.display !== 'none' && style.opacity !== '0';
  };

  // 选择器优先级：id > name > data-testid > 结构化 nth-of-type 路径。
  // 前三者稳定且短，最后是兜底，保证任何元素都有可回传的定位方式。
  const buildSelector = (el) => {
    if (el.id) {
      const byID = `#${cssEscape(el.id)}`;
      if (document.querySelectorAll(byID).length === 1) {
        return byID;
      }
    }
    for (const attr of ['data-testid', 'data-test-id', 'name', 'aria-label']) {
      const value = el.getAttribute(attr);
      if (value) {
        const candidate = `${el.tagName.toLowerCase()}[${attr}="${value.replace(/"/g, '\\"')}"]`;
        try {
          if (document.querySelectorAll(candidate).length === 1) {
            return candidate;
          }
        } catch {
          /* 属性值里可能有非法字符，忽略这个候选 */
        }
      }
    }

    const parts = [];
    let node = el;
    while (node && node.nodeType === 1 && parts.length < 6) {
      const tag = node.tagName.toLowerCase();
      if (tag === 'html' || tag === 'body') {
        break;
      }
      const parent = node.parentElement;
      if (!parent) {
        parts.unshift(tag);
        break;
      }
      const siblings = Array.from(parent.children).filter((child) => child.tagName === node.tagName);
      parts.unshift(siblings.length > 1 ? `${tag}:nth-of-type(${siblings.indexOf(node) + 1})` : tag);
      if (node.id) {
        parts.unshift(`#${cssEscape(node.id)}`);
        break;
      }
      node = parent;
    }
    return parts.join(' > ');
  };

  const accessibleName = (el) => {
    const candidates = [
      el.getAttribute('aria-label'),
      el.getAttribute('placeholder'),
      el.getAttribute('title'),
      el.tagName === 'IMG' ? el.getAttribute('alt') : '',
      el.tagName === 'INPUT' && el.type === 'submit' ? el.value : '',
      (el.innerText || el.textContent || '').trim().slice(0, 120),
      el.getAttribute('name'),
    ];
    for (const candidate of candidates) {
      const text = String(candidate || '').replace(/\s+/g, ' ').trim();
      if (text) {
        return text;
      }
    }
    return '';
  };

  const results = [];
  const seen = new Set();
  for (const el of document.querySelectorAll(INTERACTIVE)) {
    if (results.length >= limit) {
      break;
    }
    if (seen.has(el) || !isVisible(el)) {
      continue;
    }
    seen.add(el);

    const tag = el.tagName.toLowerCase();
    const entry = {
      role: el.getAttribute('role') || (tag === 'a' ? 'link' : tag === 'button' ? 'button' : tag),
      tag,
      name: accessibleName(el),
      selector: buildSelector(el),
    };
    if (tag === 'input' || tag === 'textarea' || tag === 'select') {
      entry.type = el.getAttribute('type') || tag;
      entry.value = typeof el.value === 'string' ? el.value.slice(0, 200) : '';
      if (el.disabled) entry.disabled = true;
      if (typeof el.checked === 'boolean' && (entry.type === 'checkbox' || entry.type === 'radio')) {
        entry.checked = el.checked;
      }
    }
    if (tag === 'a' && el.href) {
      entry.href = String(el.href).slice(0, 500);
    }
    results.push(entry);
  }

  return {
    url: location.href,
    title: document.title,
    elements: results,
    truncated: results.length >= limit,
  };
}

async function runPageSession(payload, chromium) {
  const defaultTimeout = normalizeTimeout(payload.defaultTimeoutMs, DEFAULT_TIMEOUT_MS);
  const artifactDir = String(payload.artifactDir || '').trim();

  // stdout 必须严格串行，否则并发写会把两行 JSON 交错在一起。
  let writeChain = Promise.resolve();
  const emit = (message) => {
    writeChain = writeChain.then(
      () =>
        new Promise((resolve) => {
          process.stdout.write(`${JSON.stringify(message)}\n`, () => resolve());
        })
    );
    return writeChain;
  };
  const diagnose = (text) => {
    try {
      process.stderr.write(`${text}\n`);
    } catch {
      /* stderr 不可用时无需处理 */
    }
  };

  const gateway = createBrowserGateway(payload, chromium, { defaultTimeout });
  const session = await gateway.launch({});
  const connection = await gateway.connect(session, { timeoutMs: payload.connectTimeoutMs });
  const { browser, context } = await gateway.resolveConnectionContext(connection);

  const state = { page: connection.page || null };

  const livePages = () => context.pages().filter((page) => !page.isClosed());

  const activePage = async () => {
    if (state.page && !state.page.isClosed()) {
      return state.page;
    }
    const pages = livePages();
    state.page = pages.length > 0 ? pages[0] : await context.newPage();
    return state.page;
  };

  const describePage = async (page) => {
    if (!page || page.isClosed()) {
      return { url: '', title: '' };
    }
    let title = '';
    try {
      title = await page.title();
    } catch {
      /* 导航过程中取 title 可能失败，不影响主结果 */
    }
    return { url: page.url(), title };
  };

  const locatorFor = async (args) => {
    const page = await activePage();
    const selector = String((args && args.selector) || '').trim();
    if (!selector) {
      throw new Error('selector is required');
    }
    let scope = page;
    const frame = String((args && args.frameSelector) || '').trim();
    if (frame) {
      const frameLocator = page.frameLocator(frame);
      scope = frameLocator;
    }
    const locator = scope.locator(selector);
    const index = Number(args && args.index);
    return Number.isFinite(index) && index >= 0 ? locator.nth(index) : locator.first();
  };

  const timeoutOf = (args) => normalizeTimeout(args && args.timeoutMs, defaultTimeout);

  const actions = {
    goto: async (args) => {
      const page = await activePage();
      const url = String((args && args.url) || '').trim();
      if (!url) {
        throw new Error('url is required');
      }
      const waitUntil = ALLOWED_WAIT_UNTIL.has(String((args && args.waitUntil) || '').trim())
        ? String(args.waitUntil).trim()
        : 'domcontentloaded';
      const response = await page.goto(url, { waitUntil, timeout: timeoutOf(args) });
      return {
        ...(await describePage(page)),
        status: response ? response.status() : 0,
      };
    },

    back: async (args) => {
      const page = await activePage();
      await page.goBack({ timeout: timeoutOf(args) });
      return describePage(page);
    },

    forward: async (args) => {
      const page = await activePage();
      await page.goForward({ timeout: timeoutOf(args) });
      return describePage(page);
    },

    reload: async (args) => {
      const page = await activePage();
      await page.reload({ timeout: timeoutOf(args) });
      return describePage(page);
    },

    click: async (args) => {
      const locator = await locatorFor(args);
      const options = { timeout: timeoutOf(args) };
      if (args && args.button) options.button = String(args.button);
      if (args && args.clickCount) options.clickCount = Number(args.clickCount);
      if (args && args.force === true) options.force = true;
      await locator.click(options);
      return describePage(await activePage());
    },

    dblclick: async (args) => {
      const locator = await locatorFor(args);
      await locator.dblclick({ timeout: timeoutOf(args) });
      return describePage(await activePage());
    },

    hover: async (args) => {
      const locator = await locatorFor(args);
      await locator.hover({ timeout: timeoutOf(args) });
      return describePage(await activePage());
    },

    // fill 支持一次填多个字段：表单场景下省掉大量往返。
    fill: async (args) => {
      const timeout = timeoutOf(args);
      const entries = Array.isArray(args && args.fields)
        ? args.fields
        : [{ selector: args && args.selector, value: args && args.value, frameSelector: args && args.frameSelector }];

      const filled = [];
      for (const entry of entries) {
        if (!isPlainObject(entry)) {
          throw new Error('each field must be an object with selector and value');
        }
        const locator = await locatorFor({ ...entry, timeoutMs: timeout });
        await locator.fill(String(entry.value == null ? '' : entry.value), { timeout });
        filled.push(String(entry.selector || '').trim());
      }
      return { filled, ...(await describePage(await activePage())) };
    },

    type: async (args) => {
      const locator = await locatorFor(args);
      const timeout = timeoutOf(args);
      await locator.click({ timeout });
      await locator.pressSequentially(String((args && args.text) || ''), {
        delay: Number((args && args.delayMs) || 0) || undefined,
        timeout,
      });
      return describePage(await activePage());
    },

    press: async (args) => {
      const key = String((args && args.key) || '').trim();
      if (!key) {
        throw new Error('key is required');
      }
      const timeout = timeoutOf(args);
      if (args && String(args.selector || '').trim()) {
        const locator = await locatorFor(args);
        await locator.press(key, { timeout });
      } else {
        const page = await activePage();
        await page.keyboard.press(key);
      }
      return describePage(await activePage());
    },

    selectOption: async (args) => {
      const locator = await locatorFor(args);
      const raw = args && args.values !== undefined ? args.values : args && args.value;
      const values = Array.isArray(raw) ? raw.map(String) : [String(raw == null ? '' : raw)];
      const selected = await locator.selectOption(values, { timeout: timeoutOf(args) });
      return { selected, ...(await describePage(await activePage())) };
    },

    check: async (args) => {
      const locator = await locatorFor(args);
      await locator.check({ timeout: timeoutOf(args) });
      return describePage(await activePage());
    },

    uncheck: async (args) => {
      const locator = await locatorFor(args);
      await locator.uncheck({ timeout: timeoutOf(args) });
      return describePage(await activePage());
    },

    scroll: async (args) => {
      const page = await activePage();
      if (args && String(args.selector || '').trim()) {
        const locator = await locatorFor(args);
        await locator.scrollIntoViewIfNeeded({ timeout: timeoutOf(args) });
        return describePage(page);
      }
      const dx = Number((args && args.deltaX) || 0);
      const dy = Number((args && args.deltaY) || 0);
      await page.mouse.wheel(Number.isFinite(dx) ? dx : 0, Number.isFinite(dy) ? dy : 0);
      return describePage(page);
    },

    waitFor: async (args) => {
      const page = await activePage();
      const timeout = timeoutOf(args);
      const selector = String((args && args.selector) || '').trim();
      const url = String((args && args.url) || '').trim();
      const loadState = String((args && args.loadState) || '').trim();

      if (selector) {
        const requested = String((args && args.state) || 'visible').trim();
        const elementState = ALLOWED_ELEMENT_STATES.has(requested) ? requested : 'visible';
        const locator = await locatorFor({ ...args, timeoutMs: timeout });
        await locator.waitFor({ state: elementState, timeout });
      } else if (url) {
        await page.waitForURL(url, { timeout });
      } else if (loadState) {
        await page.waitForLoadState(ALLOWED_LOAD_STATES.has(loadState) ? loadState : 'load', { timeout });
      } else {
        await page.waitForTimeout(timeout);
      }
      return describePage(page);
    },

    snapshot: async (args) => {
      const page = await activePage();
      const limit = Number((args && args.limit) || DEFAULT_SNAPSHOT_LIMIT);
      const snapshot = await page.evaluate(collectInteractiveElements, {
        limit: Number.isFinite(limit) && limit > 0 ? Math.min(limit, 500) : DEFAULT_SNAPSHOT_LIMIT,
      });
      if (args && args.includeText === true) {
        snapshot.text = String(await page.innerText('body').catch(() => '')).slice(
          0,
          Number((args && args.textLimit) || 5000)
        );
      }
      return snapshot;
    },

    extract: async (args) => {
      const timeout = timeoutOf(args);
      const mode = String((args && args.mode) || 'text').trim() || 'text';
      const locator = await locatorFor({ ...args, timeoutMs: timeout });
      const all = args && args.all === true;
      const attribute = String((args && args.attribute) || '').trim();

      const readOne = async (target) => {
        if (mode === 'html') return target.innerHTML({ timeout });
        if (mode === 'attribute') {
          if (!attribute) {
            throw new Error('attribute is required when mode is attribute');
          }
          return target.getAttribute(attribute, { timeout });
        }
        return target.innerText({ timeout });
      };

      if (!all) {
        return { mode, value: await readOne(locator) };
      }

      // all 模式要重新取整组 locator：locatorFor 已经收敛到 first()/nth()。
      const page = await activePage();
      const group = page.locator(String(args.selector).trim());
      const count = await group.count();
      const values = [];
      for (let i = 0; i < count; i += 1) {
        values.push(await readOne(group.nth(i)));
      }
      return { mode, count, values };
    },

    evaluate: async (args) => {
      const page = await activePage();
      const expression = String((args && args.expression) || '').trim();
      if (!expression) {
        throw new Error('expression is required');
      }
      const hasArg = args && Object.prototype.hasOwnProperty.call(args, 'arg');
      const value = hasArg ? await page.evaluate(expression, args.arg) : await page.evaluate(expression);
      return { value: toSerializable(value) };
    },

    screenshot: async (args) => {
      if (!artifactDir) {
        throw new Error('artifactDir is not configured');
      }
      const page = await activePage();
      const type = String((args && args.type) || 'jpeg').trim() === 'png' ? 'png' : 'jpeg';
      const fileName = `screenshot-${Date.now()}-${Math.round(process.hrtime()[1] / 1000)}.${type}`;
      const targetPath = normalizePathUnderRoot(artifactDir, fileName);
      fs.mkdirSync(path.dirname(targetPath), { recursive: true });

      const options = { path: targetPath, type, timeout: timeoutOf(args) };
      if (type === 'jpeg') {
        const quality = Number((args && args.quality) || 60);
        options.quality = Number.isFinite(quality) ? Math.min(Math.max(Math.round(quality), 1), 100) : 60;
      }
      if (args && args.fullPage === true) {
        options.fullPage = true;
      }

      if (args && String(args.selector || '').trim()) {
        const locator = await locatorFor(args);
        await locator.screenshot(options);
      } else {
        await page.screenshot(options);
      }

      return {
        path: targetPath,
        mimeType: type === 'png' ? 'image/png' : 'image/jpeg',
        ...(await describePage(page)),
      };
    },

    tabs: async (args) => {
      const op = String((args && args.op) || 'list').trim() || 'list';
      const pages = livePages();

      if (op === 'list') {
        const items = [];
        for (let i = 0; i < pages.length; i += 1) {
          items.push({ index: i, active: pages[i] === state.page, ...(await describePage(pages[i])) });
        }
        return { count: items.length, items };
      }

      if (op === 'new') {
        const page = await context.newPage();
        state.page = page;
        const url = String((args && args.url) || '').trim();
        if (url) {
          await page.goto(url, { waitUntil: 'domcontentloaded', timeout: timeoutOf(args) });
        }
        return { index: livePages().indexOf(page), ...(await describePage(page)) };
      }

      const index = Number(args && args.index);
      if (!Number.isFinite(index) || index < 0 || index >= pages.length) {
        throw new Error(`tab index ${String(args && args.index)} is out of range (0-${pages.length - 1})`);
      }

      if (op === 'select') {
        state.page = pages[index];
        await state.page.bringToFront().catch(() => {});
        return { index, ...(await describePage(state.page)) };
      }

      if (op === 'close') {
        const target = pages[index];
        await target.close();
        if (state.page === target) {
          state.page = null;
        }
        const remaining = livePages();
        return { closed: index, count: remaining.length };
      }

      throw new Error(`unsupported tabs op: ${op}`);
    },
  };

  let shuttingDown = false;
  const shutdown = async (reason, code) => {
    if (shuttingDown) {
      return;
    }
    shuttingDown = true;
    await emit({ type: 'closed', reason });
    try {
      await gateway.closeAll();
    } catch (error) {
      diagnose(`close browser connection failed: ${error && error.message ? error.message : String(error)}`);
    }
    process.exit(code);
  };

  // 浏览器被用户关掉、实例被停掉时 CDP 会断。主动播报一次让 Go 侧立刻回收，
  // 而不是等到下一条指令超时才发现。
  browser.on('disconnected', () => {
    void shutdown('browser disconnected', 0);
  });

  await emit({
    type: 'ready',
    session: connection.session || session,
    page: await describePage(await activePage()),
  });

  const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });

  // 指令串行执行：同一个 page 上并发操作没有意义，且回包顺序会乱。
  let queue = Promise.resolve();

  const handleLine = async (line) => {
    const trimmed = line.trim();
    if (!trimmed) {
      return;
    }

    let command = null;
    try {
      command = JSON.parse(trimmed);
    } catch (error) {
      await emit({ id: 0, ok: false, error: `invalid command json: ${error.message}` });
      return;
    }

    const id = Number(command && command.id) || 0;
    const action = String((command && command.action) || '').trim();

    if (action === 'close') {
      await emit({ id, ok: true, result: {} });
      await shutdown('closed by request', 0);
      return;
    }

    const handler = actions[action];
    if (!handler) {
      await emit({ id, ok: false, error: `unsupported page action: ${action}` });
      return;
    }

    try {
      const result = await handler(isPlainObject(command.args) ? command.args : {});
      await emit({ id, ok: true, result: toSerializable(result) });
    } catch (error) {
      await emit({ id, ok: false, error: error && error.message ? error.message : String(error) });
    }
  };

  rl.on('line', (line) => {
    queue = queue.then(() => handleLine(line)).catch((error) => {
      diagnose(`page session command failed: ${error && error.message ? error.message : String(error)}`);
    });
  });

  // stdin 关闭意味着 Go 侧已经放弃这个会话，直接收摊。
  rl.on('close', () => {
    void queue.then(() => shutdown('stdin closed', 0));
  });

  await new Promise(() => {});
}

module.exports = {
  runPageSession,
  collectInteractiveElements,
};
