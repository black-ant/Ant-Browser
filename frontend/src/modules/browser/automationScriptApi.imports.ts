import { importAutomationScript, type AutomationScriptRecord } from "./automationScripts";
import { getBindings, normalizeAutomationScriptRecord, type AutomationScriptBatchImportResult } from "./automationScriptApi.shared";

export async function importAutomationScriptFromLocalFile(): Promise<AutomationScriptRecord> {
  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptImportLocalFile) {
    return normalizeAutomationScriptRecord(
      await bindings.AutomationScriptImportLocalFile(),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptImportLocalFile === "function") {
    return normalizeAutomationScriptRecord(
      await goApp.AutomationScriptImportLocalFile(),
    );
  }

  throw new Error("当前环境不支持本地文件导入");
}

export async function importAutomationScriptFromText(
  text: string,
): Promise<AutomationScriptRecord> {
  const normalizedText = String(text || "").trim();
  if (!normalizedText) {
    throw new Error("导入内容不能为空");
  }

  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptImportText) {
    return normalizeAutomationScriptRecord(
      await bindings.AutomationScriptImportText(normalizedText),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptImportText === "function") {
    return normalizeAutomationScriptRecord(
      await goApp.AutomationScriptImportText(normalizedText),
    );
  }

  return importAutomationScript(normalizedText);
}

export async function importAutomationScriptFromLocalDirectory(): Promise<AutomationScriptRecord> {
  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptImportLocalDirectory) {
    return normalizeAutomationScriptRecord(
      await bindings.AutomationScriptImportLocalDirectory(),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptImportLocalDirectory === "function") {
    return normalizeAutomationScriptRecord(
      await goApp.AutomationScriptImportLocalDirectory(),
    );
  }

  throw new Error("当前环境不支持本地目录导入");
}

function normalizeAutomationScriptBatchImportResult(
  payload: any,
): AutomationScriptBatchImportResult {
  const imported = Array.isArray(payload?.imported)
    ? payload.imported.map(normalizeAutomationScriptRecord)
    : [];

  return {
    imported,
    failed: Array.isArray(payload?.failed)
      ? payload.failed.map((item: any) => ({
          path: String(item?.path || ""),
          message: String(item?.message || ""),
        }))
      : [],
    scanned:
      Number.isFinite(Number(payload?.scanned)) && Number(payload.scanned) > 0
        ? Math.round(Number(payload.scanned))
        : imported.length,
  };
}

export async function importAutomationScriptFromLocalLibrary(): Promise<AutomationScriptBatchImportResult> {
  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptImportLocalLibrary) {
    return normalizeAutomationScriptBatchImportResult(
      await bindings.AutomationScriptImportLocalLibrary(),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptImportLocalLibrary === "function") {
    return normalizeAutomationScriptBatchImportResult(
      await goApp.AutomationScriptImportLocalLibrary(),
    );
  }

  throw new Error("当前环境不支持本地脚本库导入");
}

export async function importAutomationScriptFromRemote(url: string): Promise<AutomationScriptRecord> {
  const normalizedURL = String(url || "").trim();
  if (!normalizedURL) {
    throw new Error("远程脚本地址不能为空");
  }

  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptImportRemote) {
    return normalizeAutomationScriptRecord(
      await bindings.AutomationScriptImportRemote(normalizedURL),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptImportRemote === "function") {
    return normalizeAutomationScriptRecord(
      await goApp.AutomationScriptImportRemote(normalizedURL),
    );
  }

  throw new Error("当前环境不支持远程脚本导入");
}

export async function importAutomationScriptFromGit(
  repoURL: string,
  ref = "",
  scriptPath = "",
): Promise<AutomationScriptRecord> {
  const normalizedRepoURL = String(repoURL || "").trim();
  if (!normalizedRepoURL) {
    throw new Error("Git 仓库地址不能为空");
  }

  const normalizedRef = String(ref || "").trim();
  const normalizedScriptPath = String(scriptPath || "").trim();

  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptImportGit) {
    return normalizeAutomationScriptRecord(
      await bindings.AutomationScriptImportGit(
        normalizedRepoURL,
        normalizedRef,
        normalizedScriptPath,
      ),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptImportGit === "function") {
    return normalizeAutomationScriptRecord(
      await goApp.AutomationScriptImportGit(
        normalizedRepoURL,
        normalizedRef,
        normalizedScriptPath,
      ),
    );
  }

  throw new Error("当前环境不支持 Git 脚本导入");
}

export async function refreshAutomationScript(
  scriptId: string,
): Promise<AutomationScriptRecord> {
  const normalizedScriptId = String(scriptId || "").trim();
  if (!normalizedScriptId) {
    throw new Error("脚本 ID 不能为空");
  }

  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptRefresh) {
    return normalizeAutomationScriptRecord(
      await bindings.AutomationScriptRefresh(normalizedScriptId),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptRefresh === "function") {
    return normalizeAutomationScriptRecord(
      await goApp.AutomationScriptRefresh(normalizedScriptId),
    );
  }

  throw new Error("当前环境不支持按来源重新导入");
}
