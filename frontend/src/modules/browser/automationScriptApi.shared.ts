import {
  normalizeAutomationScriptPublicAPIConfig,
  normalizeAutomationScriptRecordPayload,
  normalizeAutomationScriptTargetConfig,
  type AutomationScriptRunInput,
  type AutomationScriptRunRecord,
  type AutomationScriptRecord,
} from "./automationScripts";

export const getBindings = async () => {
  try {
    return await import("../../wailsjs/go/main/App");
  } catch {
    return null;
  }
};

export function normalizeAutomationScriptRecord(payload: any): AutomationScriptRecord {
  const normalized = normalizeAutomationScriptRecordPayload(payload);
  if (normalized) {
    return normalized;
  }

  return {
    packageFormat: String(payload?.packageFormat || "ant-automation-script"),
    manifestVersion: Number(payload?.manifestVersion) || 1,
    id: String(payload?.id || ""),
    name: String(payload?.name || ""),
    description: String(payload?.description || ""),
    type: payload?.type === "launch-api" ? "launch-api" : "playwright-cdp",
    status:
      payload?.status === "ready" || payload?.status === "disabled"
        ? payload.status
        : "draft",
    entryFile: String(payload?.entryFile || "index.cjs"),
    tags: Array.isArray(payload?.tags)
      ? payload.tags
          .map((item: unknown) => String(item || "").trim())
          .filter(Boolean)
      : [],
    selectorText: String(payload?.selectorText || ""),
    paramsText: String(payload?.paramsText || ""),
    scriptText: String(payload?.scriptText || ""),
    notes: String(payload?.notes || ""),
    targetConfig: normalizeAutomationScriptTargetConfig(payload?.targetConfig),
    publicAPI: normalizeAutomationScriptPublicAPIConfig(payload?.publicAPI),
    source: {
      type: String(payload?.source?.type || ""),
      uri: String(payload?.source?.uri || ""),
      ref: String(payload?.source?.ref || ""),
      path: String(payload?.source?.path || ""),
      importedAt: String(payload?.source?.importedAt || ""),
    },
    createdAt: String(payload?.createdAt || ""),
    updatedAt: String(payload?.updatedAt || ""),
  };
}

export function sortScripts(
  items: AutomationScriptRecord[],
): AutomationScriptRecord[] {
  return [...items].sort(
    (left, right) =>
      new Date(right.updatedAt).getTime() - new Date(left.updatedAt).getTime(),
  );
}

export function normalizeAutomationScriptRunRecord(
  payload: any,
): AutomationScriptRunRecord {
  return {
    id: String(payload?.id || ""),
    scriptId: String(payload?.scriptId || ""),
    scriptName: String(payload?.scriptName || ""),
    scriptType: String(payload?.scriptType || ""),
    status:
      payload?.status === "success" || payload?.status === "running"
        ? payload.status
        : "failed",
    summary: String(payload?.summary || ""),
    error: String(payload?.error || ""),
    resultText: String(payload?.resultText || ""),
    logText: String(payload?.logText || ""),
    startedAt: String(payload?.startedAt || ""),
    finishedAt: String(payload?.finishedAt || ""),
    durationMs: Number(payload?.durationMs) || 0,
  };
}

export interface AutomationScriptExportResult {
  cancelled: boolean;
  format: string;
  message: string;
  path: string;
  fileCount: number;
}

export interface AutomationScriptImportIssue {
  path: string;
  message: string;
}

export interface AutomationScriptBatchImportResult {
  imported: AutomationScriptRecord[];
  failed: AutomationScriptImportIssue[];
  scanned: number;
}

export interface AutomationScriptPublicApiInvokeInput {
  url: string;
  method?: string;
  bodyText?: string;
  apiKey?: string;
  authHeader?: string;
  timeoutMs?: number;
}

export interface AutomationScriptPublicApiInvokeResult {
  ok: boolean;
  status: number;
  statusText: string;
  bodyText: string;
  bodyJson: unknown | null;
}

export function normalizeAutomationScriptPublicApiInvokeResult(
  payload: any,
): AutomationScriptPublicApiInvokeResult {
  return {
    ok: payload?.ok === true,
    status: Number(payload?.status) || 0,
    statusText: String(payload?.statusText || ""),
    bodyText: String(payload?.bodyText || ""),
    bodyJson: payload?.bodyJson ?? null,
  };
}

export function normalizeAutomationScriptRunInput(
  input: string | AutomationScriptRunInput,
): AutomationScriptRunInput {
  if (typeof input === "string") {
    return {
      scriptId: input,
      selectorText: "",
      targetInput: {},
      paramsText: "",
      useScriptSelector: true,
      useScriptParams: true,
      timeoutMs: 0,
      launchCode: "",
      startByCodeBeforeRun: false,
    };
  }

  return {
    scriptId: String(input?.scriptId || ""),
    selectorText: String(input?.selectorText || ""),
    targetInput:
      input?.targetInput && typeof input.targetInput === "object"
        ? { ...input.targetInput }
        : {},
    paramsText: String(input?.paramsText || ""),
    useScriptSelector: input?.useScriptSelector !== false,
    useScriptParams: input?.useScriptParams !== false,
    timeoutMs: Number.isFinite(Number(input?.timeoutMs))
      ? Math.round(Number(input?.timeoutMs))
      : 0,
    launchCode: String(input?.launchCode || "")
      .trim()
      .toUpperCase(),
    startByCodeBeforeRun: input?.startByCodeBeforeRun === true,
  };
}
