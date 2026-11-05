import { exportAutomationScript, type AutomationScriptRecord } from "./automationScripts";
import { getBindings, type AutomationScriptExportResult } from "./automationScriptApi.shared";

function normalizeAutomationScriptExportResult(
  payload: any,
): AutomationScriptExportResult {
  return {
    cancelled: payload?.cancelled === true,
    format: String(payload?.format || ""),
    message: String(payload?.message || ""),
    path: String(payload?.path || ""),
    fileCount: Number(payload?.fileCount) || 0,
  };
}

function buildAutomationTemplateFallbackFilename(script: AutomationScriptRecord): string {
  const normalizedName = String(script.name || "")
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, "-")
    .replace(/^-+|-+$/g, "");

  return `${normalizedName || "automation-script"}-template.json`;
}

function downloadAutomationTemplate(
  filename: string,
  content: string,
): AutomationScriptExportResult {
  const blob = new Blob([content], { type: "application/json;charset=utf-8" });
  const url = URL.createObjectURL(blob);

  try {
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = filename;
    anchor.click();
  } finally {
    URL.revokeObjectURL(url);
  }

  return {
    cancelled: false,
    format: "json",
    message: "模板已导出",
    path: filename,
    fileCount: 1,
  };
}

export async function exportAutomationScriptTemplate(
  scriptId: string,
  fallbackScript?: AutomationScriptRecord,
): Promise<AutomationScriptExportResult> {
  const normalizedScriptId = String(scriptId || "").trim();
  if (!normalizedScriptId) {
    throw new Error("脚本 ID 不能为空");
  }

  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptExport) {
    return normalizeAutomationScriptExportResult(
      await bindings.AutomationScriptExport(normalizedScriptId),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptExport === "function") {
    return normalizeAutomationScriptExportResult(
      await goApp.AutomationScriptExport(normalizedScriptId),
    );
  }

  if (fallbackScript && typeof document !== "undefined") {
    return downloadAutomationTemplate(
      buildAutomationTemplateFallbackFilename(fallbackScript),
      exportAutomationScript(fallbackScript),
    );
  }

  throw new Error("当前环境不支持脚本模板导出");
}

export async function exportAutomationScriptZip(
  scriptId: string,
): Promise<AutomationScriptExportResult> {
  const normalizedScriptId = String(scriptId || "").trim();
  if (!normalizedScriptId) {
    throw new Error("脚本 ID 不能为空");
  }

  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptExportZip) {
    return normalizeAutomationScriptExportResult(
      await bindings.AutomationScriptExportZip(normalizedScriptId),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptExportZip === "function") {
    return normalizeAutomationScriptExportResult(
      await goApp.AutomationScriptExportZip(normalizedScriptId),
    );
  }

  throw new Error("当前环境不支持 ZIP 脚本包导出");
}

export async function exportAutomationScriptsBatchZip(
  scriptIds: string[],
): Promise<AutomationScriptExportResult> {
  const normalizedScriptIds = Array.from(
    new Set(
      scriptIds
        .map((scriptId) => String(scriptId || "").trim())
        .filter(Boolean),
    ),
  );
  if (normalizedScriptIds.length === 0) {
    throw new Error("请先勾选要导出的脚本");
  }

  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptExportBatchZip) {
    return normalizeAutomationScriptExportResult(
      await bindings.AutomationScriptExportBatchZip(normalizedScriptIds),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptExportBatchZip === "function") {
    return normalizeAutomationScriptExportResult(
      await goApp.AutomationScriptExportBatchZip(normalizedScriptIds),
    );
  }

  if (normalizedScriptIds.length === 1) {
    return exportAutomationScriptZip(normalizedScriptIds[0]);
  }

  throw new Error("当前环境不支持批量 ZIP 脚本包导出");
}

export async function exportAutomationScriptDirectory(
  scriptId: string,
): Promise<AutomationScriptExportResult> {
  const normalizedScriptId = String(scriptId || "").trim();
  if (!normalizedScriptId) {
    throw new Error("脚本 ID 不能为空");
  }

  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptExportDirectory) {
    return normalizeAutomationScriptExportResult(
      await bindings.AutomationScriptExportDirectory(normalizedScriptId),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptExportDirectory === "function") {
    return normalizeAutomationScriptExportResult(
      await goApp.AutomationScriptExportDirectory(normalizedScriptId),
    );
  }

  throw new Error("当前环境不支持目录脚本包导出");
}
