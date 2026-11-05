import { loadAutomationScripts, saveAutomationScripts, type AutomationScriptRecord } from "./automationScripts";
import { getBindings, normalizeAutomationScriptRecord, sortScripts } from "./automationScriptApi.shared";

export async function fetchAutomationScripts(): Promise<
  AutomationScriptRecord[]
> {
  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptList) {
    const raw = (await bindings.AutomationScriptList()) || [];
    return sortScripts(
      Array.isArray(raw) ? raw.map(normalizeAutomationScriptRecord) : [],
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptList === "function") {
    const raw = (await goApp.AutomationScriptList()) || [];
    return sortScripts(
      Array.isArray(raw) ? raw.map(normalizeAutomationScriptRecord) : [],
    );
  }

  return loadAutomationScripts();
}

export async function saveAutomationScript(
  script: AutomationScriptRecord,
): Promise<AutomationScriptRecord> {
  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptSave) {
    return normalizeAutomationScriptRecord(
      await bindings.AutomationScriptSave(script),
    );
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptSave === "function") {
    return normalizeAutomationScriptRecord(
      await goApp.AutomationScriptSave(script),
    );
  }

  const current = loadAutomationScripts();
  const next = current.some((item) => item.id === script.id)
    ? current.map((item) => (item.id === script.id ? script : item))
    : [script, ...current];
  saveAutomationScripts(sortScripts(next));
  return script;
}

export async function deleteAutomationScript(scriptId: string): Promise<void> {
  const bindings: any = await getBindings();
  if (bindings?.AutomationScriptDelete) {
    await bindings.AutomationScriptDelete(scriptId);
    return;
  }

  const goApp = (window as any).go?.main?.App;
  if (typeof goApp?.AutomationScriptDelete === "function") {
    await goApp.AutomationScriptDelete(scriptId);
    return;
  }

  saveAutomationScripts(
    loadAutomationScripts().filter((item) => item.id !== scriptId),
  );
}
