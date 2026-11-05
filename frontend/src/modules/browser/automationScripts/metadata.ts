import {
  AUTOMATION_SCRIPT_STATUS_OPTIONS,
  AUTOMATION_SCRIPT_TYPE_OPTIONS,
  type AutomationScriptSource,
  type AutomationScriptStatus,
  type AutomationScriptType,
} from "./definitions";

export function normalizeAutomationScriptSource(
  source: unknown,
): AutomationScriptSource {
  if (!source || typeof source !== "object") {
    return {
      type: "",
      uri: "",
      ref: "",
      path: "",
      importedAt: "",
    };
  }

  const raw = source as Partial<AutomationScriptSource>;
  return {
    type: typeof raw.type === "string" ? raw.type.trim() : "",
    uri: typeof raw.uri === "string" ? raw.uri.trim() : "",
    ref: typeof raw.ref === "string" ? raw.ref.trim() : "",
    path: typeof raw.path === "string" ? raw.path.trim() : "",
    importedAt:
      typeof raw.importedAt === "string" ? raw.importedAt.trim() : "",
  };
}

export function getAutomationScriptSourceLabel(source: AutomationScriptSource): string {
  switch (source.type) {
    case "builtin":
      return "内置基线";
    case "local-file":
      return "本地文件";
    case "local-dir":
      return "本地目录";
    case "remote-url":
      return "远程 URL";
    case "git":
      return "Git";
    case "text":
      return "文本导入";
    case "manual":
      return "手动维护";
    default:
      return source.type || "未标记";
  }
}

export function canRefreshAutomationScriptSource(
  source: AutomationScriptSource,
): boolean {
  return (
    source.type === "builtin" ||
    source.type === "local-file" ||
    source.type === "local-dir" ||
    source.type === "remote-url" ||
    source.type === "git"
  );
}

export function getAutomationScriptRefreshLabel(
  source: AutomationScriptSource,
): string {
  if (source.type === "builtin") {
    return "恢复基线";
  }
  return source.type === "git" ? "重新拉取" : "重新导入";
}

export function getAutomationScriptTypeLabel(
  type: AutomationScriptType,
): string {
  return (
    AUTOMATION_SCRIPT_TYPE_OPTIONS.find((item) => item.value === type)?.label ||
    type
  );
}

export function getAutomationScriptStatusLabel(
  status: AutomationScriptStatus,
): string {
  return (
    AUTOMATION_SCRIPT_STATUS_OPTIONS.find((item) => item.value === status)
      ?.label || status
  );
}
