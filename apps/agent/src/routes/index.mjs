import crypto from "node:crypto";
import os from "node:os";
import {
  captureAntProfileSessionBundle,
  bootstrapDesktopSession,
  checkAntRuntimeReachable,
  checkServerReachable,
  closeAntProfile,
  getAntProfileRuntime,
  getDesktopSharedLoginBindSession,
  heartbeatDeviceBridgeAgent,
  heartbeatDesktopDevice,
  launchAntProfile,
  listDesktopShopProfiles,
  listDesktopShops,
  navigateAntProfile,
  pullDeviceBridgeTasks,
  registerDeviceBridgeAgent,
  registerDesktopDevice,
  reportDeviceBridgeTask,
  reportOpenShopResult,
  requestDesktopSharedLoginBind,
  requestDesktopSharedLoginValidate,
  requestOpenShop,
  uploadDeviceBridgeTaskSessionBundle,
  upsertAntProfile
} from "../clients/index.mjs";
import {
  appendRunEvent,
  createAgentSessionId,
  getRun,
  getState,
  listRunEvents,
  listRuns,
  setDevice,
  setSession,
  setShops,
  upsertRun
} from "../state/index.mjs";

let heartbeatTimer = null;
let localBridgeTaskTimer = null;
const TERMINAL_RUN_STATUSES = new Set(["succeeded", "failed"]);
const LOCAL_BRIDGE_POLL_INTERVAL_MS = Number(process.env.LOCAL_BRIDGE_POLL_INTERVAL_MS || 3000);
const LOCAL_BRIDGE_WATCH_TIMEOUT_MS = Number(process.env.LOCAL_BRIDGE_WATCH_TIMEOUT_MS || 10 * 60 * 1000);
const LOCAL_BRIDGE_BIND_URL = process.env.LOCAL_BRIDGE_BIND_URL || "https://login.1688.com/member/signin.htm";
const LOCAL_BRIDGE_VALIDATE_URL = process.env.LOCAL_BRIDGE_VALIDATE_URL || "https://work.1688.com/";
const CDP_CONNECT_TIMEOUT_MS = Number(process.env.CDP_CONNECT_TIMEOUT_MS || 5000);
const CDP_RESPONSE_TIMEOUT_MS = Number(process.env.CDP_RESPONSE_TIMEOUT_MS || 30000);
const LOCAL_BRIDGE_SUCCESS_URL_PATTERNS = [
  "work.1688.com",
  "trade.1688.com",
  "air.1688.com",
  "seller.1688.com"
];
const runningLocalBridgeTasks = new Set();

function sendJson(res, statusCode, payload) {
  res.writeHead(statusCode, { "content-type": "application/json; charset=utf-8" });
  res.end(JSON.stringify(payload));
}

function ok(data) {
  return {
    code: 0,
    message: "ok",
    data
  };
}

async function readJsonBody(req) {
  const chunks = [];
  for await (const chunk of req) {
    chunks.push(chunk);
  }

  if (chunks.length === 0) {
    return {};
  }

  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

function deviceFingerprint() {
  const seed = `${os.hostname()}|${os.arch()}|${os.platform()}`;
  return crypto.createHash("sha256").update(seed).digest("hex");
}

function deviceName() {
  return `${os.hostname()}-${os.platform()}`;
}

function resolveFailureCode(error) {
  if (error?.failureCode) {
    return error.failureCode;
  }
  const message = error instanceof Error ? error.message : "open failed";
  const text = String(message).toLowerCase();
  if (text.includes("fetch failed") || text.includes("econnrefused") || text.includes("network")) {
    return "ANT_CONNECT_FAILED";
  }
  return "ANT_OPEN_FAILED";
}

function looksLikeLoginUrl(url) {
  const text = String(url || "").toLowerCase();
  if (!text) {
    return false;
  }
  return [
    "login.1688.com",
    "login.taobao.com",
    "member/signin",
    "passport",
    "signin"
  ].some((token) => text.includes(token));
}

function matchesUrlPatterns(url, patterns = []) {
  const text = String(url || "").toLowerCase();
  if (!text) {
    return false;
  }
  return patterns.some((pattern) => text.includes(String(pattern || "").toLowerCase()));
}

function isSuccessfulOpenUrl(currentUrl, launchContext = {}) {
  const url = String(currentUrl || "").trim();
  if (!url || url === "about:blank") {
    return false;
  }
  if (looksLikeLoginUrl(url) || matchesUrlPatterns(url, launchContext.loginUrlPatterns || [])) {
    return false;
  }
  if (matchesUrlPatterns(url, launchContext.successUrlPatterns || [])) {
    return true;
  }
  return false;
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isLocalBridgeSuccessUrl(currentUrl) {
  return matchesUrlPatterns(currentUrl, LOCAL_BRIDGE_SUCCESS_URL_PATTERNS) && !looksLikeLoginUrl(currentUrl);
}

function localBridgeTargetUrl(taskType) {
  return taskType === "bind" ? LOCAL_BRIDGE_BIND_URL : LOCAL_BRIDGE_VALIDATE_URL;
}

async function cdpTargets(debugPort) {
  const response = await fetch(`http://127.0.0.1:${debugPort}/json`);
  if (!response.ok) {
    throw new Error(`CDP targets unavailable (${response.status})`);
  }
  const targets = await response.json();
  return Array.isArray(targets) ? targets : [];
}

function scoreLocalBridgeTarget(target) {
  const url = String(target?.url || "").trim();
  if (!url || url === "about:blank" || url.startsWith("chrome://")) {
    return 0;
  }
  if (isLocalBridgeSuccessUrl(url)) {
    return 100;
  }
  if ((url.includes("1688.com") || url.includes("alibaba.com")) && !looksLikeLoginUrl(url)) {
    return 80;
  }
  if (looksLikeLoginUrl(url)) {
    return 40;
  }
  return 20;
}

async function selectCdpPageTarget(debugPort) {
  const targets = (await cdpTargets(debugPort)).filter((item) => item.type === "page" && item.webSocketDebuggerUrl);
  targets.sort((left, right) => scoreLocalBridgeTarget(right) - scoreLocalBridgeTarget(left));
  return targets[0] || null;
}

async function webSocketDataToText(data) {
  if (typeof data === "string") {
    return data;
  }
  if (data instanceof ArrayBuffer) {
    return Buffer.from(data).toString("utf8");
  }
  if (ArrayBuffer.isView(data)) {
    return Buffer.from(data.buffer, data.byteOffset, data.byteLength).toString("utf8");
  }
  if (data && typeof data.arrayBuffer === "function") {
    return Buffer.from(await data.arrayBuffer()).toString("utf8");
  }
  return Buffer.from(data || []).toString("utf8");
}

async function cdpCallWebSocket(wsUrl, method, params = {}, label = "CDP") {
  const ws = new WebSocket(wsUrl);
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`${label} WebSocket 连接超时`)), CDP_CONNECT_TIMEOUT_MS);
    ws.addEventListener("open", () => {
      clearTimeout(timer);
      resolve();
    }, { once: true });
    ws.addEventListener("error", () => {
      clearTimeout(timer);
      reject(new Error(`${label} WebSocket 连接失败`));
    }, { once: true });
  });

  try {
    const id = Date.now();
    const response = await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error(`CDP ${method} 响应超时`)), CDP_RESPONSE_TIMEOUT_MS);
      ws.addEventListener("message", async (event) => {
        try {
          const payload = JSON.parse((await webSocketDataToText(event.data)) || "{}");
          if (payload.id !== id) {
            return;
          }
          clearTimeout(timer);
          if (payload.error) {
            reject(new Error(payload.error.message || `CDP ${method} 执行失败`));
            return;
          }
          resolve(payload.result || {});
        } catch (error) {
          clearTimeout(timer);
          reject(error);
        }
      });
      ws.send(JSON.stringify({ id, method, params }));
    });
    return response;
  } finally {
    ws.close();
  }
}

async function cdpCall(debugPort, method, params = {}) {
  if (typeof WebSocket !== "function") {
    throw new Error("当前 Node 版本不支持 WebSocket，无法通过 CDP 操作 Maka Browser");
  }
  const target = await selectCdpPageTarget(debugPort);
  if (!target) {
    throw new Error("未找到 Maka Browser 可调试页面");
  }

  return cdpCallWebSocket(target.webSocketDebuggerUrl, method, params, "CDP");
}

async function cdpBrowserCall(debugPort, method, params = {}) {
  if (typeof WebSocket !== "function") {
    throw new Error("当前 Node 版本不支持 WebSocket，无法通过 CDP 操作 Maka Browser");
  }
  const versionResponse = await fetch(`http://127.0.0.1:${debugPort}/json/version`);
  if (!versionResponse.ok) {
    throw new Error(`CDP browser target unavailable (${versionResponse.status})`);
  }
  const version = await versionResponse.json();
  const wsUrl = String(version.webSocketDebuggerUrl || "").trim();
  if (!wsUrl) {
    throw new Error("未找到 Maka Browser 浏览器级调试地址");
  }

  return cdpCallWebSocket(wsUrl, method, params, "CDP Browser");
}

async function navigateAntPage(debugPort, targetUrl) {
  await cdpCall(debugPort, "Page.navigate", { url: targetUrl });
}

async function readAntCurrentUrl(debugPort) {
  const page = await selectCdpPageTarget(debugPort);
  return String(page?.url || "");
}

function readRuntimeValue(result) {
  return result?.result?.value ?? null;
}

async function captureAntSessionBundle(debugPort, platformCode, captureStartedAt) {
  let cookiesResult = null;
  try {
    cookiesResult = await cdpCall(debugPort, "Network.getAllCookies", {});
  } catch (error) {
    cookiesResult = await cdpBrowserCall(debugPort, "Storage.getCookies", {});
  }
  const cookies = Array.isArray(cookiesResult.cookies)
    ? cookiesResult.cookies.filter((cookie) => String(cookie.domain || "").endsWith("1688.com") || String(cookie.domain || "").endsWith("alibaba.com"))
    : [];
  const userAgentResult = await cdpCall(debugPort, "Runtime.evaluate", {
    expression: "window.navigator.userAgent",
    returnByValue: true
  });
  const storageResult = await cdpCall(debugPort, "Runtime.evaluate", {
    expression: `(() => ({
      origin: window.location.origin,
      localStorage: Object.fromEntries(Object.entries(window.localStorage)),
      sessionStorage: Object.fromEntries(Object.entries(window.sessionStorage))
    }))()`,
    returnByValue: true
  });
  const currentUrl = await readAntCurrentUrl(debugPort);
  const storage = readRuntimeValue(storageResult);
  return {
    platformCode: platformCode || "alibaba",
    capturedAt: new Date().toISOString(),
    captureStartedAt,
    lastObservedUrl: currentUrl,
    userAgent: String(readRuntimeValue(userAgentResult) || ""),
    cookies,
    storages: storage?.origin ? [storage] : []
  };
}

async function readAntRuntimeCurrentUrl(profileId) {
  const runtime = await getAntProfileRuntime({ profileId });
  return String(runtime.currentUrl || "");
}

async function captureAntRuntimeSessionBundle(profileId, platformCode, captureStartedAt) {
  const result = await captureAntProfileSessionBundle({
    profileId,
    platformCode,
    captureStartedAt
  });
  if (!result?.sessionBundle) {
    throw new Error("Maka Browser Runtime 未返回共享会话数据");
  }
  return result.sessionBundle;
}

function currentAgentStatus() {
  const state = getState();
  if (!state.session) {
    return "degraded";
  }
  return "ready";
}

function countActiveRuns() {
  return listRuns({ limit: 200 }).filter((item) => !TERMINAL_RUN_STATUSES.has(item.status)).length;
}

function buildHealthSnapshot() {
  const state = getState();
  return {
    status: state.session ? "ready" : "degraded",
    agentStatus: currentAgentStatus(),
    sessionReady: Boolean(state.session),
    serverReachable: false,
    antRuntimeReachable: false,
    activeRunCount: countActiveRuns(),
    deviceId: state.device?.deviceId || null,
    deviceStatus: state.device?.status || null,
    user: state.session?.user || null
  };
}

function createRun(shopId, taskType = "open") {
  const runId = `run-${crypto.randomUUID()}`;
  const run = {
    runId,
    taskId: runId,
    shopId,
    taskType,
    status: "accepted",
    statusLabel: "accepted",
    startedAt: new Date().toISOString(),
    finishedAt: null,
    profileId: null,
    runtime: null,
    bindSessionId: null,
    manualActionRequired: false,
    challengeType: null,
    cancellable: false,
    failureCode: null,
    failureMessage: null
  };
  const next = upsertRun(run);
  appendRunEvent(runId, {
    eventId: `evt-${crypto.randomUUID()}`,
    stage: "accepted",
    message: "run accepted",
    createdAt: new Date().toISOString()
  });
  return next;
}

function patchRun(runId, patch) {
  const current = getRun(runId);
  if (!current) {
    return null;
  }
  return upsertRun({ ...current, ...patch });
}

function logRunEvent(runId, stage, message, details = {}) {
  appendRunEvent(runId, {
    eventId: `evt-${crypto.randomUUID()}`,
    stage,
    message,
    details,
    createdAt: new Date().toISOString()
  });
}

function mapBindSessionStatus(status) {
  switch (status) {
    case "launching":
      return "launching";
    case "awaiting_verification":
      return "awaiting_verification";
    case "capturing":
      return "capturing";
    case "completed":
      return "succeeded";
    case "failed":
    case "expired":
      return "failed";
    default:
      return "authorizing";
  }
}

function syncRunWithBindSession(runId, bindSession) {
  const nextStatus = mapBindSessionStatus(bindSession.status);
  const finishedAt =
    nextStatus === "succeeded" || nextStatus === "failed"
      ? bindSession.completedAt || new Date().toISOString()
      : null;
  return patchRun(runId, {
    status: nextStatus,
    statusLabel: bindSession.statusLabel,
    bindSessionId: bindSession.bindSessionId,
    manualActionRequired: Boolean(bindSession.manualActionRequired),
    challengeType: bindSession.challengeType || null,
    finishedAt,
    failureCode: nextStatus === "failed" ? bindSession.status.toUpperCase() : null,
    failureMessage: nextStatus === "failed" ? bindSession.message : null
  });
}

async function trackBindSessionRun({ runId, accessToken, serverBaseUrl, bindSessionId }) {
  for (let attempt = 0; attempt < 300; attempt += 1) {
    const current = getRun(runId);
    if (!current || TERMINAL_RUN_STATUSES.has(current.status)) {
      return;
    }
    const snapshot = await getDesktopSharedLoginBindSession({
      accessToken,
      serverBaseUrl,
      bindSessionId
    });
    const bindSession = snapshot.data;
    syncRunWithBindSession(runId, bindSession);
    logRunEvent(runId, bindSession.status, bindSession.message, {
      bindSessionId,
      sessionType: bindSession.sessionType
    });
    if (bindSession.status === "completed" || bindSession.status === "failed" || bindSession.status === "expired") {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
}

function ensureAgentReady(state, res) {
  if (!state.session || !state.device) {
    sendJson(res, 400, {
      code: 51001,
      message: "agent not ready",
      data: {}
    });
    return false;
  }
  return true;
}

function acceptedTaskResponse(run) {
  return {
    taskId: run.taskId,
    runId: run.runId,
    status: "accepted",
    shopId: run.shopId,
    taskType: run.taskType
  };
}

async function performHeartbeat() {
  const state = getState();
  if (!state.session || !state.device?.deviceId) {
    return;
  }

  try {
    const antRuntimeReachable = await checkAntRuntimeReachable();
    const heartbeat = await heartbeatDesktopDevice({
      accessToken: state.session.accessToken,
      serverBaseUrl: state.session.serverBaseUrl,
      deviceId: state.device.deviceId,
      antRuntimeReachable
    });

    setDevice({
      ...state.device,
      status: heartbeat.data.deviceStatus || state.device.status,
      lastHeartbeatAt: new Date().toISOString()
    });
  } catch {
    // 心跳失败不抛出，避免中断本地主流程。
  }
}

function startHeartbeatLoop(intervalSec) {
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer);
    heartbeatTimer = null;
  }

  const intervalMs = Math.max(15, Number(intervalSec || 60)) * 1000;
  heartbeatTimer = setInterval(() => {
    void performHeartbeat();
  }, intervalMs);
}

async function registerAndHeartbeatDeviceBridge(state) {
  const fingerprint = deviceFingerprint();
  const name = deviceName();
  await registerDeviceBridgeAgent({
    accessToken: state.session.accessToken,
    serverBaseUrl: state.session.serverBaseUrl,
    deviceFingerprint: fingerprint,
    deviceName: name
  });
  await heartbeatDeviceBridgeAgent({
    accessToken: state.session.accessToken,
    serverBaseUrl: state.session.serverBaseUrl,
    deviceFingerprint: fingerprint,
    deviceName: name
  });
}

async function reportLocalBridgeTask(state, taskId, status, payload = {}) {
  await reportDeviceBridgeTask({
    accessToken: state.session.accessToken,
    serverBaseUrl: state.session.serverBaseUrl,
    taskId,
    status,
    message: payload.message || "",
    errorMessage: payload.errorMessage || "",
    failureCode: payload.failureCode || "",
    result: payload.result || {},
    attempts: payload.attempts || (status === "completed" ? 8 : 3)
  });
}

async function resolveShopForLocalBridgeTask(state, shopId) {
  const shops = await listDesktopShops({
    accessToken: state.session.accessToken,
    serverBaseUrl: state.session.serverBaseUrl
  });
  setShops(shops.data.items || []);
  return (shops.data.items || []).find((item) => item.shopId === shopId) || {
    shopId,
    shopName: shopId,
    platformCode: "alibaba"
  };
}

async function closeLocalBridgeProfileAfterCredentialTask(run, task, profileId) {
  if (task.taskType !== "bind" && task.taskType !== "validate") {
    return;
  }

  try {
    const result = await closeAntProfile({ profileId });
    logRunEvent(run.runId, "closed", "local bridge controlled browser closed", {
      taskId: task.id,
      profileId,
      closed: Boolean(result?.closed)
    });
  } catch (error) {
    logRunEvent(run.runId, "close_failed", "local bridge controlled browser close failed", {
      taskId: task.id,
      profileId,
      message: error instanceof Error ? error.message : String(error)
    });
  }
}

async function runAntLocalBridgeTask(state, task) {
  const run = createRun(task.shopId, task.taskType);
  patchRun(run.runId, {
    taskId: task.id,
    status: "authorizing",
    statusLabel: "authorizing"
  });
  logRunEvent(run.runId, "authorizing", "local bridge task pulled", { taskId: task.id });

  try {
    await reportLocalBridgeTask(state, task.id, "running", {
      message: "本机 Agent 正在调用 Maka Browser"
    });
    if (!(await checkAntRuntimeReachable())) {
      throw new Error("Maka Browser Runtime 不可达");
    }

    const shop = await resolveShopForLocalBridgeTask(state, task.shopId);
    const platformCode = shop.platformCode || "alibaba";
    const profileId = `${platformCode}:${task.shopId}`;
    await upsertAntProfile({
      profileId,
      shopId: task.shopId,
      platformCode,
      profileName: shop.shopName || task.shopId,
      userDataDir: `${process.cwd()}/.runtime/profiles/${profileId.replaceAll(":", "__")}`
    });

    const targetUrl = localBridgeTargetUrl(task.taskType);
    const existingBundle = task.payload?.sessionBundle || null;
    const launchResult = await launchAntProfile({
      profileId,
      headless: false,
      targetUrl,
      sessionBundle: existingBundle
    });
    const runtime = launchResult.debugPort ? launchResult : await getAntProfileRuntime({ profileId });
    const debugPort = Number(runtime.debugPort || 0);
    if (!debugPort) {
      await navigateAntProfile({ profileId, targetUrl });
    }
    await reportLocalBridgeTask(state, task.id, "awaiting_user_input", {
      message: task.taskType === "open"
        ? "Maka Browser 已打开店铺后台，正在确认页面状态"
        : "Maka Browser 已打开登录页，请在本机完成登录或验证",
      result: {
        profileId,
        debugPort,
        targetUrl
      }
    });

    const captureStartedAt = new Date().toISOString();
    const deadline = Date.now() + LOCAL_BRIDGE_WATCH_TIMEOUT_MS;
    let lastObservedUrl = "";
    let lastReportedObservedUrl = "";
    while (Date.now() < deadline) {
      lastObservedUrl = debugPort ? await readAntCurrentUrl(debugPort) : await readAntRuntimeCurrentUrl(profileId);
      if (lastObservedUrl && lastObservedUrl !== lastReportedObservedUrl) {
        lastReportedObservedUrl = lastObservedUrl;
        await reportLocalBridgeTask(state, task.id, "awaiting_user_input", {
          message: "Maka Browser 已打开登录页，请在本机完成登录或验证",
          result: {
            profileId,
            debugPort,
            targetUrl,
            lastObservedUrl
          }
        }).catch((error) => {
          logRunEvent(run.runId, "awaiting_user_input_report_failed", "local bridge URL report failed", {
            taskId: task.id,
            message: error instanceof Error ? error.message : String(error)
          });
        });
      }
      if (isLocalBridgeSuccessUrl(lastObservedUrl)) {
        const sessionBundle =
          task.taskType === "bind" || task.taskType === "validate"
            ? await captureAntRuntimeSessionBundle(profileId, platformCode, captureStartedAt)
            : null;
        const sessionBundleUpload = sessionBundle
          ? await uploadDeviceBridgeTaskSessionBundle({
              accessToken: state.session.accessToken,
              serverBaseUrl: state.session.serverBaseUrl,
              taskId: task.id,
              sessionBundle
            })
          : null;
        await reportLocalBridgeTask(state, task.id, "completed", {
          message: task.taskType === "open" ? "Maka Browser 已进入目标后台" : "Maka Browser 已完成共享登录凭据更新",
          result: {
            profileId,
            debugPort,
            lastObservedUrl,
            ...(sessionBundleUpload
              ? {
                  sessionBundleUploadId: sessionBundleUpload.uploadId,
                  sessionBundleUpload: {
                    chunkCount: sessionBundleUpload.chunkCount,
                    compressedBytes: sessionBundleUpload.compressedBytes
                  }
                }
              : {})
          },
          attempts: 10
        });
        await closeLocalBridgeProfileAfterCredentialTask(run, task, profileId);
        patchRun(run.runId, {
          status: "succeeded",
          statusLabel: "succeeded",
          finishedAt: new Date().toISOString(),
          profileId,
          runtime: { debugPort, currentUrl: lastObservedUrl }
        });
        logRunEvent(run.runId, "succeeded", "local bridge task succeeded", { taskId: task.id });
        return;
      }
      await delay(2000);
    }

    await reportLocalBridgeTask(state, task.id, "failed", {
      message: "本机操作超时",
      errorMessage: looksLikeLoginUrl(lastObservedUrl) ? "等待人工登录超时" : "未进入目标后台页面",
      failureCode: looksLikeLoginUrl(lastObservedUrl) ? "challenge_timeout" : "target_url_not_reached",
      result: { profileId, debugPort, lastObservedUrl }
    });
    patchRun(run.runId, {
      status: "failed",
      statusLabel: "failed",
      finishedAt: new Date().toISOString(),
      failureCode: "LOCAL_BRIDGE_TIMEOUT",
      failureMessage: "本机操作超时"
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    await reportLocalBridgeTask(state, task.id, "failed", {
      message: "本机桥接执行失败",
      errorMessage: message,
      failureCode: "unknown"
    }).catch(() => {});
    patchRun(run.runId, {
      status: "failed",
      statusLabel: "failed",
      finishedAt: new Date().toISOString(),
      failureCode: "LOCAL_BRIDGE_FAILED",
      failureMessage: message
    });
    logRunEvent(run.runId, "failed", "local bridge task failed", { taskId: task.id, message });
  }
}

async function pullAndRunLocalBridgeTasks() {
  const state = getState();
  if (!state.session || !state.device) {
    return;
  }
  try {
    await heartbeatDeviceBridgeAgent({
      accessToken: state.session.accessToken,
      serverBaseUrl: state.session.serverBaseUrl,
      deviceFingerprint: deviceFingerprint(),
      deviceName: deviceName()
    });
  } catch {
    await registerAndHeartbeatDeviceBridge(state);
  }
  const payload = await pullDeviceBridgeTasks({
    accessToken: state.session.accessToken,
    serverBaseUrl: state.session.serverBaseUrl,
    deviceFingerprint: deviceFingerprint(),
    limit: 1
  });
  for (const task of payload.data?.items || payload.items || []) {
    if (!task?.id || runningLocalBridgeTasks.has(task.id)) {
      continue;
    }
    runningLocalBridgeTasks.add(task.id);
    void runAntLocalBridgeTask(state, task).finally(() => {
      runningLocalBridgeTasks.delete(task.id);
    });
  }
}

function startLocalBridgeTaskLoop() {
  if (localBridgeTaskTimer) {
    clearInterval(localBridgeTaskTimer);
    localBridgeTaskTimer = null;
  }
  void pullAndRunLocalBridgeTasks().catch(() => {});
  localBridgeTaskTimer = setInterval(() => {
    void pullAndRunLocalBridgeTasks().catch(() => {});
  }, Math.max(1000, LOCAL_BRIDGE_POLL_INTERVAL_MS));
}

export async function handleLocalRequest(req, res) {
  const url = new URL(req.url || "/", "http://127.0.0.1");
  const { pathname } = url;

  if (req.method === "GET" && pathname === "/health") {
    sendJson(res, 200, ok({ status: "ready" }));
    return;
  }

  try {
    if (req.method === "POST" && pathname === "/local/session/bootstrap") {
      const payload = await readJsonBody(req);
      const agentSessionId = createAgentSessionId();

      const bootstrapResult = await bootstrapDesktopSession({
        accessToken: payload.accessToken,
        serverBaseUrl: payload.serverBaseUrl
      });

      const registerResult = await registerDesktopDevice({
        accessToken: payload.accessToken,
        serverBaseUrl: payload.serverBaseUrl,
        deviceFingerprint: deviceFingerprint(),
        deviceName: deviceName()
      });

      setSession({
        agentSessionId,
        accessToken: payload.accessToken,
        refreshToken: payload.refreshToken || null,
        user: payload.user,
        serverBaseUrl: payload.serverBaseUrl,
        heartbeatIntervalSec: bootstrapResult.data.devicePolicy?.heartbeatIntervalSec || 60
      });

      setDevice(registerResult.data.device);
      await performHeartbeat();
      startHeartbeatLoop(bootstrapResult.data.devicePolicy?.heartbeatIntervalSec || 60);
      await registerAndHeartbeatDeviceBridge(getState()).catch(() => {});
      startLocalBridgeTaskLoop();

      sendJson(
        res,
        200,
        ok({
          agentSessionId,
          deviceStatus: getState().device?.status || registerResult.data.device.status,
          agentVersion: "0.1.0"
        })
      );
      return;
    }

    if (req.method === "GET" && pathname === "/local/health") {
      const state = getState();
      const snapshot = buildHealthSnapshot();
      snapshot.serverReachable = state.session
        ? await checkServerReachable(state.session.serverBaseUrl)
        : false;
      snapshot.antRuntimeReachable = await checkAntRuntimeReachable();
      sendJson(res, 200, ok(snapshot));
      return;
    }

    if (req.method === "GET" && pathname === "/local/device/status") {
      const state = getState();
      const serverReachable = state.session
        ? await checkServerReachable(state.session.serverBaseUrl)
        : false;
      const antRuntimeReachable = await checkAntRuntimeReachable();

      sendJson(
        res,
        200,
        ok({
          agentStatus: currentAgentStatus(),
          deviceStatus: state.device?.status || "unregistered",
          deviceId: state.device?.deviceId || null,
          user: state.session?.user || null,
          serverReachable,
          antRuntimeReachable,
          lastHeartbeatAt: state.device?.lastHeartbeatAt || null
        })
      );
      return;
    }

    if (req.method === "GET" && pathname === "/local/shops") {
      const state = getState();
      if (state.session) {
        const shops = await listDesktopShops({
          accessToken: state.session.accessToken,
          serverBaseUrl: state.session.serverBaseUrl
        });
        setShops(shops.data.items || []);
      }

      const next = getState();
      sendJson(
        res,
        200,
        ok({
          items: next.shops.items,
          syncedAt: next.shops.syncedAt || new Date().toISOString()
        })
      );
      return;
    }

    if (req.method === "GET" && pathname === "/local/shop-profiles") {
      const state = getState();
      if (!ensureAgentReady(state, res)) return;

      const profiles = await listDesktopShopProfiles({
        accessToken: state.session.accessToken,
        serverBaseUrl: state.session.serverBaseUrl
      });
      sendJson(
        res,
        200,
        ok({
          items: profiles.data.items || [],
          syncedAt: new Date().toISOString()
        })
      );
      return;
    }

    const openContextMatch = pathname.match(/^\/local\/shops\/([^/]+)\/open-context$/);
    if (req.method === "POST" && openContextMatch) {
      const state = getState();
      if (!ensureAgentReady(state, res)) return;

      const shopId = decodeURIComponent(openContextMatch[1]);
      const openResult = await requestOpenShop({
        accessToken: state.session.accessToken,
        serverBaseUrl: state.session.serverBaseUrl,
        shopId,
        deviceId: state.device.deviceId
      });
      sendJson(res, 200, ok(openResult.data));
      return;
    }

    const openMatch = pathname.match(/^\/local\/shops\/([^/]+)\/open$/);
    if (req.method === "POST" && openMatch) {
      const state = getState();
      if (!ensureAgentReady(state, res)) return;

      const shopId = decodeURIComponent(openMatch[1]);
      const run = createRun(shopId, "open");
      patchRun(run.runId, { status: "authorizing", statusLabel: "authorizing" });
      logRunEvent(run.runId, "authorizing", "authorizing with server");

      let openRequestId = null;
      try {
        const openResult = await requestOpenShop({
          accessToken: state.session.accessToken,
          serverBaseUrl: state.session.serverBaseUrl,
          shopId,
          deviceId: state.device.deviceId
        });
        openRequestId = openResult.data.openRequestId;
        const profileId = openResult.data.profile.profileId;
        const antReachable = await checkAntRuntimeReachable();
        if (!antReachable) {
          const runtimeError = new Error("ant runtime unavailable");
          runtimeError.failureCode = "ANT_RUNTIME_UNAVAILABLE";
          throw runtimeError;
        }

        patchRun(run.runId, {
          status: "upserting_profile",
          statusLabel: "upserting_profile",
          profileId
        });
        logRunEvent(run.runId, "upserting_profile", "profile upsert requested", {
          profileId
        });
        try {
          await upsertAntProfile({
            profileId,
            shopId,
            platformCode: openResult.data.shop.platformCode,
            profileName: openResult.data.shop.shopName,
            userDataDir: `${process.cwd()}/.runtime/profiles/${profileId.replaceAll(":", "__")}`
          });
        } catch (error) {
          const upsertError = new Error(error instanceof Error ? error.message : "profile upsert failed");
          upsertError.failureCode = "ANT_PROFILE_UPSERT_FAILED";
          throw upsertError;
        }

        patchRun(run.runId, {
          status: "launching",
          statusLabel: "launching",
          profileId
        });
        logRunEvent(run.runId, "launching", "profile launch requested", {
          profileId
        });

        let launchResult = null;
        try {
          const launchContext = openResult.data.launchContext || {};
          launchResult = await launchAntProfile({
            profileId,
            headless: false,
            targetUrl: launchContext.targetUrl || null,
            sessionBundle: launchContext.sessionBundle || null
          });
          if (!isSuccessfulOpenUrl(launchResult.currentUrl, launchContext)) {
            const reloginError = new Error("未能打开目标店铺后台，请先执行更新凭据后重试");
            reloginError.failureCode = "ANT_OPEN_FAILED";
            throw reloginError;
          }
        } catch (error) {
          const launchError = new Error(error instanceof Error ? error.message : "ant open failed");
          launchError.failureCode = error?.failureCode || "ANT_OPEN_FAILED";
          throw launchError;
        }

        await reportOpenShopResult({
          accessToken: state.session.accessToken,
          serverBaseUrl: state.session.serverBaseUrl,
          openRequestId,
          deviceId: state.device.deviceId,
          status: "succeeded",
          runtime: {
            pid: launchResult.pid || 0,
            debugPort: launchResult.debugPort || 0
          }
        });

        patchRun(run.runId, {
          status: "succeeded",
          statusLabel: "succeeded",
          finishedAt: new Date().toISOString(),
          runtime: {
            pid: launchResult.pid || 0,
            debugPort: launchResult.debugPort || 0,
            currentUrl: launchResult.currentUrl || null,
            pageTitle: launchResult.pageTitle || null,
            targetUrl: launchResult.targetUrl || null
          }
        });
        logRunEvent(run.runId, "succeeded", "shop open succeeded");
      } catch (error) {
        const message = error instanceof Error ? error.message : "open failed";
        const failureCode = resolveFailureCode(error);
        if (openRequestId) {
          try {
            await reportOpenShopResult({
              accessToken: state.session.accessToken,
              serverBaseUrl: state.session.serverBaseUrl,
              openRequestId,
              deviceId: state.device.deviceId,
              status: "failed",
              failureCode,
              failureMessage: message
            });
          } catch {
            // 回传失败不覆盖本地状态。
          }
        }
        patchRun(run.runId, {
          status: "failed",
          statusLabel: "failed",
          finishedAt: new Date().toISOString(),
          failureCode,
          failureMessage: message
        });
        logRunEvent(run.runId, "failed", "shop open failed", { message });
      }

      sendJson(res, 200, ok({ runId: run.runId, taskId: run.taskId, status: "accepted", shopId }));
      return;
    }

    const openReportMatch = pathname.match(/^\/local\/open-requests\/([^/]+)\/report$/);
    if (req.method === "POST" && openReportMatch) {
      const state = getState();
      if (!ensureAgentReady(state, res)) return;

      const openRequestId = decodeURIComponent(openReportMatch[1]);
      const payload = await readJsonBody(req);
      await reportOpenShopResult({
        accessToken: state.session.accessToken,
        serverBaseUrl: state.session.serverBaseUrl,
        openRequestId,
        deviceId: state.device.deviceId,
        status: payload.status,
        runtime: payload.runtime || null,
        failureCode: payload.failureCode || null,
        failureMessage: payload.failureMessage || null
      });
      sendJson(res, 200, ok({ openRequestId, reported: true }));
      return;
    }

    const bindMatch = pathname.match(/^\/local\/shops\/([^/]+)\/bind$/);
    if (req.method === "POST" && bindMatch) {
      const state = getState();
      if (!ensureAgentReady(state, res)) return;
      const shopId = decodeURIComponent(bindMatch[1]);
      const run = createRun(shopId, "bind");
      patchRun(run.runId, { status: "authorizing", statusLabel: "authorizing" });
      logRunEvent(run.runId, "authorizing", "bind task accepted");
      void (async () => {
        try {
          const result = await requestDesktopSharedLoginBind({
            accessToken: state.session.accessToken,
            serverBaseUrl: state.session.serverBaseUrl,
            shopId
          });
          const bindSession = result.data.bindSession;
          syncRunWithBindSession(run.runId, bindSession);
          logRunEvent(run.runId, bindSession.status, bindSession.message, {
            bindSessionId: bindSession.bindSessionId,
            sessionType: bindSession.sessionType
          });
          if (!TERMINAL_RUN_STATUSES.has(getRun(run.runId)?.status || "")) {
            await trackBindSessionRun({
              runId: run.runId,
              accessToken: state.session.accessToken,
              serverBaseUrl: state.session.serverBaseUrl,
              bindSessionId: bindSession.bindSessionId
            });
          }
        } catch (error) {
          const message = error instanceof Error ? error.message : "bind failed";
          patchRun(run.runId, {
            status: "failed",
            statusLabel: "failed",
            finishedAt: new Date().toISOString(),
            failureCode: "BIND_FAILED",
            failureMessage: message
          });
          logRunEvent(run.runId, "failed", "bind task failed", { message });
        }
      })();
      sendJson(res, 200, ok(acceptedTaskResponse(run)));
      return;
    }

    const validateMatch = pathname.match(/^\/local\/shops\/([^/]+)\/validate$/);
    if (req.method === "POST" && validateMatch) {
      const state = getState();
      if (!ensureAgentReady(state, res)) return;
      const shopId = decodeURIComponent(validateMatch[1]);
      const run = createRun(shopId, "validate");
      patchRun(run.runId, { status: "authorizing", statusLabel: "authorizing" });
      logRunEvent(run.runId, "authorizing", "validate task accepted");
      void (async () => {
        try {
          const result = await requestDesktopSharedLoginValidate({
            accessToken: state.session.accessToken,
            serverBaseUrl: state.session.serverBaseUrl,
            shopId
          });
          const bindSession = result.data.bindSession;
          syncRunWithBindSession(run.runId, bindSession);
          logRunEvent(run.runId, bindSession.status, bindSession.message, {
            bindSessionId: bindSession.bindSessionId,
            sessionType: bindSession.sessionType
          });
          if (!TERMINAL_RUN_STATUSES.has(getRun(run.runId)?.status || "")) {
            await trackBindSessionRun({
              runId: run.runId,
              accessToken: state.session.accessToken,
              serverBaseUrl: state.session.serverBaseUrl,
              bindSessionId: bindSession.bindSessionId
            });
          }
        } catch (error) {
          const message = error instanceof Error ? error.message : "validate failed";
          patchRun(run.runId, {
            status: "failed",
            statusLabel: "failed",
            finishedAt: new Date().toISOString(),
            failureCode: "VALIDATE_FAILED",
            failureMessage: message
          });
          logRunEvent(run.runId, "failed", "validate task failed", { message });
        }
      })();
      sendJson(res, 200, ok(acceptedTaskResponse(run)));
      return;
    }

    const runMatch = pathname.match(/^\/local\/runs\/([^/]+)$/);
    if (req.method === "GET" && runMatch) {
      const run = getRun(decodeURIComponent(runMatch[1]));
      if (!run) {
        sendJson(res, 404, { code: 404, message: "run not found", data: {} });
        return;
      }
      sendJson(res, 200, ok(run));
      return;
    }

    const runEventsMatch = pathname.match(/^\/local\/runs\/([^/]+)\/events$/);
    if (req.method === "GET" && runEventsMatch) {
      const runId = decodeURIComponent(runEventsMatch[1]);
      const run = getRun(runId);
      if (!run) {
        sendJson(res, 404, { code: 404, message: "run not found", data: {} });
        return;
      }
      const limit = Number(url.searchParams.get("limit") || 50);
      const items = listRunEvents(runId, limit);
      sendJson(
        res,
        200,
        ok({
          runId,
          items,
          total: items.length
        })
      );
      return;
    }

    if (req.method === "GET" && pathname === "/local/runs") {
      const limit = Number(url.searchParams.get("limit") || 20);
      const status = url.searchParams.get("status") || "";
      const shopId = url.searchParams.get("shopId") || "";
      const failureCode = url.searchParams.get("failureCode") || "";
      const items = listRuns({ limit, status, shopId, failureCode });
      sendJson(
        res,
        200,
        ok({
          items,
          total: items.length
        })
      );
      return;
    }

    const taskCancelMatch = pathname.match(/^\/local\/tasks\/([^/]+)\/cancel$/);
    if (req.method === "POST" && taskCancelMatch) {
      const taskId = decodeURIComponent(taskCancelMatch[1]);
      const run = getRun(taskId);
      if (!run) {
        sendJson(res, 404, { code: 404, message: "run not found", data: {} });
        return;
      }
      sendJson(
        res,
        200,
        ok({
          taskId,
          runId: run.runId,
          accepted: false,
          cancellable: false,
          status: run.status,
          reason: TERMINAL_RUN_STATUSES.has(run.status)
            ? "task already terminal"
            : "task cancel not supported yet"
        })
      );
      return;
    }

    sendJson(res, 404, { code: 404, message: "not found", data: {} });
  } catch (error) {
    sendJson(res, 500, {
      code: 500,
      message: error instanceof Error ? error.message : "internal error",
      data: {}
    });
  }
}
