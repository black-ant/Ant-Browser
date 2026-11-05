import { toast } from "../../../shared/components";
import {
  findAutomationTargetProfile,
  prepareAutomationScriptPublicAPIConfigForSave,
  resolveAutomationScriptPublicAPIConfig,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptRecord,
  type AutomationScriptType,
} from "../automationScripts";
import type { BrowserProfile } from "../types";

export type ImportMode =
  | "local"
  | "git";
export const DUAL_INSTANCE_SCRIPT_ID = "dual-instance-runtime-switch";

export type DualLaunchCodes = {
  primaryCode: string;
  secondaryCode: string;
};

export type AutomationCardPresentation = {
  key: string;
  title: string;
  versionLabel: string;
  scriptId?: string;
  scriptType: AutomationScriptType;
  modeLabel: string;
  codeDisplay: string;
  primaryActionLabel: string;
  primaryActionText: string;
  primaryActionSuccessMessage: string;
  secondaryActionLabel: string;
  secondaryActionText: string;
  secondaryActionSuccessMessage: string;
  modeToneClass: string;
  publicAPIEnabled: boolean;
  railClassName: string;
};

function getAutomationCardRailClass(seed: string): string {
  const palette = [
    "bg-[#8aa0b3]",
    "bg-[#8da79b]",
    "bg-[#929ab1]",
    "bg-[#aa9a8e]",
    "bg-[#8b9a9c]",
  ];

  let hash = 0;
  for (const char of seed) {
    hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  }

  return palette[hash % palette.length];
}

function normalizeText(value?: string): string {
  return String(value || "").trim();
}

function formatMinuteTimestamp(value?: string): string {
  const normalized = normalizeText(value);
  if (!normalized) {
    return "";
  }

  const date = new Date(normalized);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  const year = String(date.getFullYear());
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day} ${hour}:${minute}`;
}

function resolveScriptVersionLabel(script: AutomationScriptRecord): string {
  const versionSource =
    normalizeText(script.source.importedAt) ||
    normalizeText(script.updatedAt) ||
    normalizeText(script.createdAt);
  const formatted = formatMinuteTimestamp(versionSource);
  return formatted ? `V ${formatted}` : "V -";
}

function normalizeCode(value?: string): string {
  return normalizeText(value).toUpperCase();
}

function resolveTargetCode(
  selector: AutomationScriptRecord["targetConfig"]["selector"],
  profiles: BrowserProfile[],
): string {
  const matched = findAutomationTargetProfile(selector, profiles);
  return normalizeCode(matched?.launchCode || selector.code);
}

export async function copyToClipboard(text: string, successMessage: string) {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(successMessage);
  } catch {
    toast.error("复制失败");
  }
}

function parseJSONObjectText(text?: string): Record<string, unknown> | null {
  const normalized = normalizeText(text);
  if (!normalized) {
    return null;
  }

  try {
    const parsed = JSON.parse(normalized);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
  } catch {
    return null;
  }

  return null;
}

function buildSelectorPayload(
  selector: AutomationScriptRecord["targetConfig"]["selector"],
  profiles: BrowserProfile[],
): Record<string, unknown> | null {
  const matched = findAutomationTargetProfile(selector, profiles);
  const payload: Record<string, unknown> = {};

  const code = normalizeCode(matched?.launchCode || selector.code);
  const profileId = normalizeText(matched?.profileId || selector.profileId);
  const profileName = normalizeText(
    matched?.profileName || selector.profileName,
  );
  const groupId = normalizeText(selector.groupId);

  if (code) {
    payload.code = code;
  }
  if (profileId) {
    payload.profileId = profileId;
  }
  if (profileName) {
    payload.profileName = profileName;
  }
  if (groupId) {
    payload.groupId = groupId;
  }
  if (selector.keywords.length > 0) {
    payload.keywords = [...selector.keywords];
  }
  if (selector.tags.length > 0) {
    payload.tags = [...selector.tags];
  }

  return Object.keys(payload).length > 0 ? payload : null;
}

function buildAutomationRequestPayload(
  script: AutomationScriptRecord,
  profiles: BrowserProfile[],
): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    scriptId: script.id,
  };
  const params = parseJSONObjectText(script.paramsText);

  switch (script.targetConfig.mode) {
    case "existing":
    case "rotate": {
      const selector = buildSelectorPayload(script.targetConfig.selector, profiles);
      if (selector) {
        payload.selector = selector;
      }
      break;
    }
    case "create":
      payload.useScriptSelector = true;
      break;
    default: {
      const selector = parseJSONObjectText(script.selectorText);
      if (selector && Object.keys(selector).length > 0) {
        payload.selector = selector;
      } else if (script.type === "playwright-cdp") {
        payload.selector = { code: "YOUR_CODE" };
      }
      break;
    }
  }

  if (params && Object.keys(params).length > 0) {
    payload.params = params;
  } else {
    payload.useScriptParams = true;
  }

  return payload;
}

export function buildAutomationRequestPayloadText(
  payload: Record<string, unknown>,
): string {
  return JSON.stringify(payload, null, 2);
}

export function buildAutomationRunCurlDemo(options: {
  launchBaseUrl: string;
  apiAuthEnabled: boolean;
  apiAuthHeader: string;
  payload: Record<string, unknown>;
}): string {
  const authHeader = buildCurlAuthHeaderLine(
    options.apiAuthEnabled,
    options.apiAuthHeader,
  );
  return `curl -X POST ${options.launchBaseUrl}/api/automation/scripts/run \\
  -H "Content-Type: application/json" \\
${authHeader}  -d '${buildAutomationRequestPayloadText(options.payload)}'`;
}

function buildAutomationCardMode(
  script: AutomationScriptRecord,
): "skill" | "api-sim" {
  return script.type === "playwright-cdp" ? "skill" : "api-sim";
}

function getAutomationModeLabel(type: AutomationScriptType): string {
  return type === "playwright-cdp" ? "脚本模式" : "接口模式";
}

function getAutomationModeToneClass(type: AutomationScriptType): string {
  return type === "playwright-cdp"
    ? "bg-[var(--color-text-primary)]"
    : "bg-[var(--color-text-secondary)]";
}

function buildAutomationSkillPrompt(
  script: AutomationScriptRecord,
  payload: Record<string, unknown>,
) {
  const lines = [
    "使用 ant-chrome-openclaw skill。",
    `执行预置脚本 ${script.id}（${script.name}）。`,
  ];

  if (Object.prototype.hasOwnProperty.call(payload, "selector")) {
    lines.push(`selector: ${JSON.stringify(payload.selector)}`);
  } else if (payload.useScriptSelector) {
    lines.push("selector: 使用脚本默认值。");
  }

  if (Object.prototype.hasOwnProperty.call(payload, "params")) {
    lines.push(`params: ${JSON.stringify(payload.params)}`);
  } else if (payload.useScriptParams) {
    lines.push("params: 使用脚本默认值。");
  }

  return lines.join("\n");
}

function buildAutomationCodeDisplay(
  script: AutomationScriptRecord,
  profiles: BrowserProfile[],
  dualLaunchCodes: DualLaunchCodes,
): string {
  if (script.id === DUAL_INSTANCE_SCRIPT_ID) {
    return `${dualLaunchCodes.primaryCode} / ${dualLaunchCodes.secondaryCode}`;
  }

  switch (script.targetConfig.mode) {
    case "existing": {
      return resolveTargetCode(script.targetConfig.selector, profiles) || "运行时传入";
    }
    case "create": {
      return (
        resolveTargetCode(script.targetConfig.templateSelector, profiles) ||
        "运行时传入"
      );
    }
    case "rotate": {
      const code = resolveTargetCode(script.targetConfig.selector, profiles);
      if (code) {
        return code;
      }
      const selector = script.targetConfig.selector;
      const hasFilter = Boolean(
        normalizeText(selector.profileId) ||
          normalizeText(selector.profileName) ||
          normalizeText(selector.groupId) ||
          selector.keywords.length > 0 ||
          selector.tags.length > 0,
      );
      return hasFilter ? "条件匹配" : "运行时传入";
    }
    default: {
      const selector = parseJSONObjectText(script.selectorText);
      const directCode = normalizeCode(
        typeof selector?.code === "string" ? selector.code : "",
      );
      const launchCode = normalizeCode(
        typeof selector?.launchCode === "string" ? selector.launchCode : "",
      );
      return directCode || launchCode || "运行时传入";
    }
  }
}

export function buildAutomationCardPresentation(options: {
  script: AutomationScriptRecord;
  profiles: BrowserProfile[];
  launchBaseUrl: string;
  apiAuthEnabled: boolean;
  apiAuthHeader: string;
  dualLaunchCodes: DualLaunchCodes;
  dualInstanceRunPayload: Record<string, unknown>;
  dualInstanceRunPayloadText: string;
  dualInstanceRunCurlDemo: string;
}): AutomationCardPresentation {
  const { script } = options;
  const isDualInstanceScript = script.id === DUAL_INSTANCE_SCRIPT_ID;
  const requestPayload = isDualInstanceScript
    ? options.dualInstanceRunPayload
    : buildAutomationRequestPayload(script, options.profiles);
  const requestPayloadText = isDualInstanceScript
    ? options.dualInstanceRunPayloadText
    : buildAutomationRequestPayloadText(requestPayload);
  const requestCurlDemo = isDualInstanceScript
    ? options.dualInstanceRunCurlDemo
    : buildAutomationRunCurlDemo({
        launchBaseUrl: options.launchBaseUrl,
        apiAuthEnabled: options.apiAuthEnabled,
        apiAuthHeader: options.apiAuthHeader,
        payload: requestPayload,
      });
  const cardMode = buildAutomationCardMode(script);
  const resolvedPublicAPI = resolveAutomationScriptPublicAPIConfig(script);

  return {
    key: script.id,
    title: script.name,
    versionLabel: resolveScriptVersionLabel(script),
    scriptId: script.id,
    scriptType: script.type,
    modeLabel: getAutomationModeLabel(script.type),
    codeDisplay: buildAutomationCodeDisplay(
      script,
      options.profiles,
      options.dualLaunchCodes,
    ),
    primaryActionLabel: cardMode === "skill" ? "Skill" : "cURL",
    primaryActionText:
      cardMode === "skill"
        ? buildAutomationSkillPrompt(script, requestPayload)
        : requestCurlDemo,
    primaryActionSuccessMessage:
      cardMode === "skill" ? "Skill 提示词已复制" : "模拟 cURL 已复制",
    secondaryActionLabel: "JSON",
    secondaryActionText: requestPayloadText,
    secondaryActionSuccessMessage: "请求 JSON 已复制",
    modeToneClass: getAutomationModeToneClass(script.type),
    publicAPIEnabled: resolvedPublicAPI.enabled,
    railClassName: getAutomationCardRailClass(script.id),
  };
}

export function buildDualInstanceFallbackPresentation(options: {
  dualLaunchCodes: DualLaunchCodes;
  dualInstanceRunPayloadText: string;
  dualInstanceRunCurlDemo: string;
}): AutomationCardPresentation {
  return {
    key: `${DUAL_INSTANCE_SCRIPT_ID}-fallback`,
    title: "双实例启动与 Runtime 切换",
    versionLabel: "V -",
    scriptType: "launch-api",
    modeLabel: "接口模式",
    codeDisplay: `${options.dualLaunchCodes.primaryCode} / ${options.dualLaunchCodes.secondaryCode}`,
    primaryActionLabel: "cURL",
    primaryActionText: options.dualInstanceRunCurlDemo,
    primaryActionSuccessMessage: "模拟 cURL 已复制",
    secondaryActionLabel: "JSON",
    secondaryActionText: options.dualInstanceRunPayloadText,
    secondaryActionSuccessMessage: "请求 JSON 已复制",
    modeToneClass: getAutomationModeToneClass("launch-api"),
    publicAPIEnabled: false,
    railClassName: getAutomationCardRailClass(DUAL_INSTANCE_SCRIPT_ID),
  };
}

function collectAvailableLaunchCodes(profiles: BrowserProfile[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];

  for (const profile of profiles) {
    const code = normalizeCode(profile.launchCode);
    if (!code || seen.has(code)) {
      continue;
    }
    seen.add(code);
    result.push(code);
  }

  return result;
}

export function resolveDualLaunchCodes(profiles: BrowserProfile[]): DualLaunchCodes {
  const availableCodes = collectAvailableLaunchCodes(profiles);
  if (availableCodes.length >= 2) {
    return {
      primaryCode: availableCodes[0],
      secondaryCode: availableCodes[1],
    };
  }
  if (availableCodes.length === 1) {
    return {
      primaryCode: availableCodes[0],
      secondaryCode: "BUYER_002",
    };
  }

  return {
    primaryCode: "BUYER_001",
    secondaryCode: "BUYER_002",
  };
}

function buildCurlAuthHeaderLine(
  apiAuthEnabled: boolean,
  apiAuthHeader: string,
): string {
  if (!apiAuthEnabled) {
    return "";
  }
  return `  -H "${apiAuthHeader}: <YOUR_API_KEY>" \\\n`;
}

export function buildPersistablePublicAPIConfig(
  script: AutomationScriptRecord,
): AutomationScriptPublicAPIConfig {
  return prepareAutomationScriptPublicAPIConfigForSave({
    ...script,
    publicAPI: resolveAutomationScriptPublicAPIConfig(script),
  });
}

export function mergeImportedScripts(
  current: AutomationScriptRecord[],
  imported: AutomationScriptRecord[],
): AutomationScriptRecord[] {
  const deduped = new Map(imported.map((item) => [item.id, item]));
  return [...imported, ...current.filter((item) => !deduped.has(item.id))];
}
