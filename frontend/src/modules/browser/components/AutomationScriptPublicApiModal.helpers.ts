import { toast } from "../../../shared/components";
import type { AutomationScriptPublicApiInvokeResult } from "../automationScriptApi";
import {
  applyAutomationScriptPublicAPIVariables,
  buildAutomationScriptPublicAPIPath,
  buildAutomationScriptPublicAPIRequestExample,
  collectAutomationScriptPublicAPIVariableValues,
  readAutomationScriptPublicAPIParamObject,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptRecord,
} from "../automationScripts";
const INSTANCE_VARIABLE_NAMES = new Set([
  "code",
  "launchCode",
  "primaryCode",
  "secondaryCode",
]);

export function isInstanceVariableName(name: string): boolean {
  return INSTANCE_VARIABLE_NAMES.has(name.trim());
}

export function parseJSONText(
  text: string,
): { ok: boolean; value: unknown | null; error: string } {
  const sourceText = String(text || "").trim();
  if (!sourceText) {
    return { ok: true, value: null, error: "" };
  }

  try {
    return {
      ok: true,
      value: JSON.parse(sourceText),
      error: "",
    };
  } catch (error: unknown) {
    return {
      ok: false,
      value: null,
      error: error instanceof Error ? error.message : "JSON 解析失败",
    };
  }
}

export function safeParseJSONObject(text: string): Record<string, unknown> | null {
  const parsed = parseJSONText(text);
  if (!parsed.ok || !parsed.value || typeof parsed.value !== "object") {
    return null;
  }
  if (Array.isArray(parsed.value)) {
    return null;
  }
  return parsed.value as Record<string, unknown>;
}

function normalizeLaunchCode(value: unknown): string {
  return String(value || "").trim().toUpperCase();
}

export function readPublicApiDualTargetCode(bodyText: string, index: number): string {
  const body = safeParseJSONObject(bodyText);
  if (!body) return "";
  const param = readAutomationScriptPublicAPIParamObject(body);
  const browsers = Array.isArray(param.browsers)
    ? param.browsers
    : Array.isArray(body.browsers)
      ? body.browsers
      : [];
  const browser = browsers[index];
  if (!browser || typeof browser !== "object" || Array.isArray(browser)) {
    return "";
  }
  return normalizeLaunchCode(
    (browser as Record<string, unknown>).code ||
      (browser as Record<string, unknown>).launchCode,
  );
}

export function buildRequestBodyWithDualTargetCode(
  currentBodyText: string,
  fallbackBodyText: string,
  index: number,
  code: string,
): string {
  const sourceBody =
    safeParseJSONObject(currentBodyText) || safeParseJSONObject(fallbackBodyText) || {};
  const sourceParam = readAutomationScriptPublicAPIParamObject(sourceBody);
  const sourceBrowsers = Array.isArray(sourceParam.browsers)
    ? sourceParam.browsers
    : [];
  const nextBrowsers = [...sourceBrowsers];
  const currentBrowser = nextBrowsers[index];
  const nextBrowser =
    currentBrowser && typeof currentBrowser === "object" && !Array.isArray(currentBrowser)
      ? { ...(currentBrowser as Record<string, unknown>) }
      : {};
  nextBrowser.code = normalizeLaunchCode(code);
  delete nextBrowser.launchCode;
  nextBrowsers[index] = nextBrowser;

  const nextBody: Record<string, unknown> = {
    ...sourceBody,
    params: {
      ...sourceParam,
      browsers: nextBrowsers,
    },
  };
  delete nextBody.param;
  delete nextBody.browsers;

  return JSON.stringify(nextBody, null, 2);
}

export function buildCurlPreview(
  script: AutomationScriptRecord,
  config: AutomationScriptPublicAPIConfig,
  launchBaseUrl: string,
  apiAuthEnabled: boolean,
  apiAuthHeader: string,
): string {
  const lines = [
    `curl -X ${config.method} ${launchBaseUrl}${buildAutomationScriptPublicAPIPath(config.path)} \\`,
    `  -H "Content-Type: application/json" \\`,
  ];

  if (apiAuthEnabled && apiAuthHeader.trim()) {
    lines.push(`  -H "${apiAuthHeader}: <YOUR_API_KEY>" \\`);
  }

  const requestBody = applyAutomationScriptPublicAPIVariables(
    buildAutomationScriptPublicAPIRequestExample(script, config),
    config.variables,
    collectAutomationScriptPublicAPIVariableValues(config),
  ).bodyText
    .split("\n")
    .map((line, index, all) =>
      index === all.length - 1 ? `  -d '${line}'` : `  -d '${line}`,
    )
    .join("\n");

  lines.push(requestBody);
  return lines.join("\n");
}

export function formatInvokeResult(result: AutomationScriptPublicApiInvokeResult): string {
  if (result.bodyJson !== null) {
    try {
      return JSON.stringify(result.bodyJson, null, 2);
    } catch {
      // noop
    }
  }
  return result.bodyText.trim() || "(empty)";
}

export interface PublicApiOutputEntry {
  key: string;
  label: string;
  path: string;
}

export function parsePublicApiOutputEntries(result: AutomationScriptPublicApiInvokeResult | null): PublicApiOutputEntry[] {
  if (!result?.bodyJson || typeof result.bodyJson !== "object" || Array.isArray(result.bodyJson)) {
    return [];
  }

  const seen = new Set<string>();
  const outputs: PublicApiOutputEntry[] = [];
  const addOutput = (key: string, value: string) => {
    const path = value.trim();
    if (!path || seen.has(path)) {
      return;
    }
    seen.add(path);
    outputs.push({ key, label: formatPublicApiOutputLabel(key), path });
  };
  const collect = (value: unknown, keyHint = "") => {
    if (!value) {
      return;
    }
    if (typeof value === "string") {
      if (/path$/i.test(keyHint) || keyHint === "downloadAddress") {
        addOutput(keyHint, value);
      }
      return;
    }
    if (Array.isArray(value)) {
      value.forEach((item) => collect(item, keyHint));
      return;
    }
    if (typeof value !== "object") {
      return;
    }
    Object.entries(value as Record<string, unknown>).forEach(([key, nestedValue]) =>
      collect(nestedValue, key),
    );
  };

  collect(result.bodyJson);
  return outputs;
}

function formatPublicApiOutputLabel(key: string): string {
  switch (key) {
    case "outputPath":
    case "downloadPath":
    case "downloadAddress":
      return "导出文件";
    case "screenshotPath":
      return "截图文件";
    case "artifacts":
      return "导出文件";
    default:
      return key;
  }
}

export function formatPublicApiOutputName(path: string): string {
  const segments = path.split(/[\\/]/).filter(Boolean);
  return segments[segments.length - 1] || path;
}

export async function copyText(text: string, successMessage: string) {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(successMessage);
  } catch {
    toast.error("复制失败");
  }
}
