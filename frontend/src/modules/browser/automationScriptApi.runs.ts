import { type AutomationScriptRunInput, type AutomationScriptRunRecord } from "./automationScripts";
import { startBrowserInstanceByCode } from "./api/instances";
import { getBindings, normalizeAutomationScriptPublicApiInvokeResult, normalizeAutomationScriptRunInput, normalizeAutomationScriptRunRecord, type AutomationScriptPublicApiInvokeInput, type AutomationScriptPublicApiInvokeResult } from "./automationScriptApi.shared";

const AUTOMATION_RUN_LIST_TIMEOUT_MS = 8000;

function withAutomationRunListTimeout<T>(promise: Promise<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => {
      reject(new Error("调用记录加载超时"));
    }, AUTOMATION_RUN_LIST_TIMEOUT_MS);

    promise
      .then(resolve, reject)
      .finally(() => window.clearTimeout(timer));
  });
}

export async function runAutomationScript(
  input: string | AutomationScriptRunInput,
): Promise<AutomationScriptRunRecord> {
  const request = normalizeAutomationScriptRunInput(input);
  const { launchCode, startByCodeBeforeRun, ...bindingRequest } = request;

  if (startByCodeBeforeRun && launchCode) {
    const startedProfile = await startBrowserInstanceByCode(launchCode);
    if (!startedProfile) {
      throw new Error(`通过 Launch Code 启动实例失败: ${launchCode}`);
    }
  }

  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptRunWithOptions) {
    return normalizeAutomationScriptRunRecord(
      await bindings.AutomationScriptRunWithOptions(bindingRequest),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptRunWithOptions === "function") {
    return normalizeAutomationScriptRunRecord(
      await goApp.AutomationScriptRunWithOptions(bindingRequest),
    );
  }

  if (
    bindings?.AutomationScriptRun &&
    bindingRequest.useScriptSelector &&
    bindingRequest.useScriptParams
  ) {
    return normalizeAutomationScriptRunRecord(
      await bindings.AutomationScriptRun(bindingRequest.scriptId),
    );
  }

  if (
    typeof goApp?.AutomationScriptRun === "function" &&
    bindingRequest.useScriptSelector &&
    bindingRequest.useScriptParams
  ) {
    return normalizeAutomationScriptRunRecord(
      await goApp.AutomationScriptRun(bindingRequest.scriptId),
    );
  }

  const now = new Date().toISOString();
  return {
    id: `mock-run-${Date.now()}`,
    scriptId: bindingRequest.scriptId,
    scriptName: "",
    scriptType: "",
    status: "failed",
    summary: "当前环境未接入自动化脚本执行",
    error: "AutomationScriptRun binding is unavailable",
    resultText: "",
    logText: "",
    startedAt: now,
    finishedAt: now,
    durationMs: 0,
  };
}

export async function fetchAutomationScriptRuns(
  limit = 20,
): Promise<AutomationScriptRunRecord[]> {
  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptRunList) {
    const raw = (await withAutomationRunListTimeout(bindings.AutomationScriptRunList(limit))) || [];
    return Array.isArray(raw)
      ? raw.map(normalizeAutomationScriptRunRecord)
      : [];
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptRunList === "function") {
    const raw = (await withAutomationRunListTimeout(goApp.AutomationScriptRunList(limit))) || [];
    return Array.isArray(raw)
      ? raw.map(normalizeAutomationScriptRunRecord)
      : [];
  }

  return [];
}

export async function invokeAutomationScriptPublicApi(
  input: AutomationScriptPublicApiInvokeInput,
): Promise<AutomationScriptPublicApiInvokeResult> {
  const url = String(input?.url || "").trim();
  if (!url) {
    throw new Error("接口地址不能为空");
  }

  const method = String(input?.method || "POST").trim().toUpperCase() || "POST";
  const authHeader = String(input?.authHeader || "X-Ant-Api-Key").trim() || "X-Ant-Api-Key";
  const apiKey = String(input?.apiKey || "").trim();
  const bodyText = String(input?.bodyText || "").trim();
  const timeoutMs = Number.isFinite(Number(input?.timeoutMs))
    ? Math.max(1000, Math.round(Number(input?.timeoutMs)))
    : 0;

  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptInvokePublicAPI) {
    return normalizeAutomationScriptPublicApiInvokeResult(
      await bindings.AutomationScriptInvokePublicAPI({
        url,
        method,
        bodyText,
        apiKey,
        authHeader,
        timeoutMs,
      }),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptInvokePublicAPI === "function") {
    return normalizeAutomationScriptPublicApiInvokeResult(
      await goApp.AutomationScriptInvokePublicAPI({
        url,
        method,
        bodyText,
        apiKey,
        authHeader,
        timeoutMs,
      }),
    );
  }

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (apiKey) {
    headers[authHeader] = apiKey;
  }

  const response = await fetch(url, {
    method,
    headers,
    body: bodyText || "{}",
  });

  const rawText = await response.text();
  let bodyJson: unknown | null = null;
  if (rawText.trim()) {
    try {
      bodyJson = JSON.parse(rawText);
    } catch {
      bodyJson = null;
    }
  }

  return {
    ok: response.ok,
    status: response.status,
    statusText: response.statusText,
    bodyText: rawText,
    bodyJson,
  };
}
