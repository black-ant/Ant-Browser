import { toast } from "../../../shared/components";
import {
  applyAutomationScriptPublicAPIVariables,
  collectAutomationScriptPublicAPIVariableValues,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptRecord,
} from "../automationScripts";
import type { AutomationDemoSession } from "../demoSession";
import type { BrowserProfile } from "../types";
import type { ResultOutputEntry, RunVariableInputs, SelectableProfile } from "./AutomationScriptRunModal.types";

export function validateJsonObjectText(
  text: string,
  label: string,
  required: boolean,
): string {
  const normalized = text.trim();
  if (!normalized) {
    return required ? `${label}不能为空` : "";
  }

  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return `${label}必须是 JSON 对象`;
    }
    return "";
  } catch {
    return `${label}不是合法 JSON`;
  }
}

export function formatDateTime(value?: string): string {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString("zh-CN", { hour12: false });
}

export function formatDuration(durationMs?: number): string {
  if (!durationMs || durationMs <= 0) {
    return "-";
  }
  if (durationMs < 1000) {
    return `${durationMs} ms`;
  }
  return `${(durationMs / 1000).toFixed(2)} s`;
}

export function parseRunResultOutputs(resultText?: string): ResultOutputEntry[] {
  const normalized = String(resultText || "").trim();
  if (!normalized) {
    return [];
  }

  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return [];
    }

    const seen = new Set<string>();
    const outputs: ResultOutputEntry[] = [];

    const addOutput = (key: string, value: string) => {
      const path = value.trim();
      if (!path || seen.has(path)) {
        return;
      }
      seen.add(path);
      outputs.push({
        key,
        label: formatRunResultOutputLabel(key),
        path,
      });
    };

    const collectOutputs = (value: unknown, keyHint = "") => {
      if (!value) {
        return;
      }
      if (typeof value === "string") {
        if (/path$/i.test(keyHint)) {
          addOutput(keyHint, value);
        }
        return;
      }
      if (Array.isArray(value)) {
        if (keyHint === "artifacts") {
          value.forEach((item) => {
            if (typeof item === "string") {
              addOutput(keyHint, item);
            }
          });
          return;
        }
        value.forEach((item) => collectOutputs(item, keyHint));
        return;
      }
      if (typeof value !== "object") {
        return;
      }

      for (const [nestedKey, nestedValue] of Object.entries(
        value as Record<string, unknown>,
      )) {
        collectOutputs(nestedValue, nestedKey);
      }
    };

    collectOutputs(parsed);
    return outputs;
  } catch {
    return [];
  }
}

function formatRunResultOutputLabel(key: string): string {
  switch (key) {
    case "outputPath":
      return "输出文件";
    case "screenshotPath":
      return "截图文件";
    case "artifacts":
      return "导出文件";
    default:
      return key;
  }
}

export function formatRunResultOutputName(path: string): string {
  const segments = path.split(/[\\/]/).filter(Boolean);
  return segments[segments.length - 1] || path;
}

export function formatRunResultText(resultText?: string): string {
  const normalized = String(resultText || "").trim();
  if (!normalized) {
    return "";
  }

  try {
    return JSON.stringify(JSON.parse(normalized), null, 2);
  } catch {
    return resultText || "";
  }
}

export async function copyToClipboard(text: string, successMessage: string) {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(successMessage);
  } catch {
    toast.error("复制失败");
  }
}

export function buildDemoSelectorText(launchCode: string) {
  return JSON.stringify(
    {
      code: launchCode,
    },
    null,
    2,
  );
}

export function normalizeLaunchCode(value?: string): string {
  return String(value || "")
    .trim()
    .toUpperCase();
}

export function isPlaceholderSelectorText(text: string): boolean {
  const normalized = text.trim();
  if (!normalized) {
    return true;
  }

  try {
    const parsed = JSON.parse(normalized);
    const code =
      parsed && typeof parsed === "object" && !Array.isArray(parsed)
        ? String((parsed as Record<string, unknown>).code || "")
            .trim()
            .toUpperCase()
        : "";
    return !code || code === "BUYER_001" || code === "DEMO_ABC123";
  } catch {
    return false;
  }
}

function parseJsonObjectText(text: string): Record<string, unknown> {
  const normalized = text.trim();
  if (!normalized) {
    return {};
  }

  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return {};
    }
    return parsed as Record<string, unknown>;
  } catch {
    return {};
  }
}

function mergeJsonObjectValues(
  base: Record<string, unknown>,
  patch: Record<string, unknown>,
): Record<string, unknown> {
  const merged: Record<string, unknown> = { ...base };
  Object.entries(patch).forEach(([key, value]) => {
    const baseValue = merged[key];
    if (
      baseValue &&
      typeof baseValue === "object" &&
      !Array.isArray(baseValue) &&
      value &&
      typeof value === "object" &&
      !Array.isArray(value)
    ) {
      merged[key] = mergeJsonObjectValues(
        baseValue as Record<string, unknown>,
        value as Record<string, unknown>,
      );
      return;
    }
    merged[key] = value;
  });
  return merged;
}

export function buildPublicAPIVariableInputs(
  config: AutomationScriptPublicAPIConfig,
): RunVariableInputs {
  return collectAutomationScriptPublicAPIVariableValues(config);
}

export function buildParamsTextFromPublicAPIRequest(
  config: AutomationScriptPublicAPIConfig,
  values: RunVariableInputs,
  fallbackParamsText: string,
): { paramsText: string; missingRequired: string[]; usedVariables: string[] } {
  const resolvedBody = applyAutomationScriptPublicAPIVariables(
    config.requestBodyText,
    config.variables,
    values,
  );
  const body = parseJsonObjectText(resolvedBody.bodyText);
  const fallbackParams = parseJsonObjectText(fallbackParamsText);
  const requestParams =
    config.requestMode === "params-only"
      ? body
      : body.params && typeof body.params === "object" && !Array.isArray(body.params)
        ? (body.params as Record<string, unknown>)
        : {};
  const params =
    Object.keys(requestParams).length > 0
      ? mergeJsonObjectValues(fallbackParams, requestParams)
      : fallbackParams;

  return {
    paramsText: JSON.stringify(params, null, 2),
    missingRequired: resolvedBody.missingRequired,
    usedVariables: resolvedBody.usedVariables,
  };
}

export function isCodeOnlySelectorForLaunchCode(
  text: string,
  launchCode: string,
): boolean {
  const normalizedCode = normalizeLaunchCode(launchCode);
  const normalizedText = text.trim();
  if (!normalizedCode || !normalizedText) {
    return false;
  }

  try {
    const parsed = JSON.parse(normalizedText);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return false;
    }

    const entries = Object.entries(parsed as Record<string, unknown>).filter(
      ([, value]) => {
        if (value == null) {
          return false;
        }
        if (typeof value === "string") {
          return value.trim() !== "";
        }
        if (Array.isArray(value)) {
          return value.length > 0;
        }
        return true;
      },
    );
    if (entries.length !== 1 || entries[0]?.[0] !== "code") {
      return false;
    }

    return normalizeLaunchCode(String(entries[0][1] || "")) === normalizedCode;
  } catch {
    return false;
  }
}

export function resolveInitialSelectorText(
  script: AutomationScriptRecord,
  demoSession: AutomationDemoSession,
): string {
  if (
    script.targetConfig.mode !== "manual" &&
    script.targetConfig.mode !== "existing"
  ) {
    return "";
  }
  if (script.targetConfig.mode === "existing") {
    const selectorCode = normalizeLaunchCode(script.targetConfig.selector.code);
    if (selectorCode) {
      return buildDemoSelectorText(selectorCode);
    }
  }
  const currentSelectorText = String(script.selectorText || "");
  if (
    script.type === "playwright-cdp" &&
    isPlaceholderSelectorText(currentSelectorText) &&
    demoSession.launchCode
  ) {
    return buildDemoSelectorText(demoSession.launchCode);
  }
  return currentSelectorText;
}

export function resolveRunnableSelectorText(
  script: AutomationScriptRecord,
  currentSelectorText: string,
  demoSession: AutomationDemoSession,
): string {
  if (
    script.targetConfig.mode !== "manual" &&
    script.targetConfig.mode !== "existing"
  ) {
    return currentSelectorText;
  }
  if (
    script.type === "playwright-cdp" &&
    isPlaceholderSelectorText(currentSelectorText) &&
    demoSession.launchCode
  ) {
    return buildDemoSelectorText(demoSession.launchCode);
  }
  return currentSelectorText;
}

export function resolveSelectorLaunchCode(text: string): string {
  const normalized = text.trim();
  if (!normalized) {
    return "";
  }

  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return "";
    }

    return String((parsed as Record<string, unknown>).code || "")
      .trim()
      .toUpperCase();
  } catch {
    return "";
  }
}

export function filterSelectableProfiles(profiles: BrowserProfile[]): SelectableProfile[] {
  return profiles
    .flatMap((profile) => {
      const launchCode = normalizeLaunchCode(profile.launchCode);
      if (!launchCode) {
        return [];
      }
      return [
        {
          ...profile,
          launchCode,
        },
      ];
    })
    .sort((left, right) => {
      if (left.running !== right.running) {
        return left.running ? -1 : 1;
      }
      return left.profileName.localeCompare(right.profileName, "zh-CN");
    });
}

export function resolvePreferredProfileId(
  profiles: SelectableProfile[],
  preferredProfileId: string,
  preferredLaunchCode: string,
): string {
  const normalizedProfileId = String(preferredProfileId || "").trim();
  const normalizedCode = normalizeLaunchCode(preferredLaunchCode);
  if (!normalizedProfileId && !normalizedCode) {
    return "";
  }

  if (normalizedProfileId) {
    const matchedByID = profiles.find(
      (profile) => profile.profileId === normalizedProfileId,
    );
    if (matchedByID) {
      return matchedByID.profileId;
    }
  }

  const matchedByCode = profiles.find(
    (profile) => normalizeLaunchCode(profile.launchCode) === normalizedCode,
  );
  if (matchedByCode) {
    return matchedByCode.profileId;
  }

  return "";
}

export function buildSelectableProfileOptions(profiles: SelectableProfile[]) {
  return profiles.map((profile) => ({
    value: profile.profileId,
    label: `${profile.launchCode} · ${profile.profileName} · ${formatSelectableProfileStatus(profile)}`,
  }));
}

function formatSelectableProfileStatus(profile: SelectableProfile): string {
  if (profile.running && profile.debugReady && profile.debugPort > 0) {
    return "可连接";
  }
  if (profile.running) {
    return "启动中";
  }
  return "未启动，执行时自动启动";
}

export function sortTemplateProfiles(profiles: BrowserProfile[]) {
  return [...profiles].sort((left, right) =>
    left.profileName.localeCompare(right.profileName, "zh-CN"),
  );
}

export function buildTemplateProfileOptions(profiles: BrowserProfile[]) {
  return profiles.map((profile) => ({
    value: profile.profileId,
    label: [profile.launchCode || "", profile.profileName || profile.profileId]
      .filter(Boolean)
      .join(" · "),
  }));
}
