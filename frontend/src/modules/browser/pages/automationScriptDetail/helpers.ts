import {
  AUTOMATION_SCRIPT_TARGET_MODE_OPTIONS,
  DUAL_INSTANCE_RUNTIME_SCRIPT_ID,
  findAutomationTargetProfile,
  formatAutomationTargetIdentity,
  getAutomationScriptSourceLabel,
  type AutomationScriptRecord,
  type AutomationScriptTargetConfig,
  type AutomationScriptTargetSelector,
} from "../../automationScripts";
import type { BrowserGroupWithCount, BrowserProfile } from "../../types";
export {
  buildPersistablePublicAPIConfig,
  hasSamePublicAPIConfig,
  preparePublicAPIConfigForCompare,
  validatePublicAPIConfig,
} from "./publicApiConfigHelpers";

export interface DualRuntimeRequestPreview {
  code: string;
  payload: Record<string, unknown>;
}

export interface DualRuntimePreviewResult {
  requests: DualRuntimeRequestPreview[];
  error: string;
}

export interface SelectorSuggestion {
  key: string;
  value: string;
  label?: string;
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

export function targetModeBadgeVariant(
  mode: AutomationScriptTargetConfig["mode"],
): "default" | "info" | "warning" | "success" {
  switch (mode) {
    case "existing":
      return "info";
    case "create":
      return "success";
    case "rotate":
      return "warning";
    default:
      return "default";
  }
}

export function formatTargetModeLabel(
  mode: AutomationScriptTargetConfig["mode"],
): string {
  return (
    AUTOMATION_SCRIPT_TARGET_MODE_OPTIONS.find((item) => item.value === mode)
      ?.label || mode
  );
}

export function formatScriptSource(script: AutomationScriptRecord): string {
  const { source } = script;
  if (!source.type && !source.uri && !source.path) {
    return "手动维护";
  }

  const mainValue = source.uri || source.path || "已导入";
  const extras = [source.ref, source.path && source.uri ? source.path : ""]
    .filter(Boolean)
    .join(" · ");
  const sourceLabel = getAutomationScriptSourceLabel(source);

  return extras
    ? `${sourceLabel} · ${mainValue} · ${extras}`
    : `${sourceLabel} · ${mainValue}`;
}

export function parseSelectorTerms(text: string): string[] {
  const deduped = new Set<string>();
  for (const item of text.split(/[\n,]/g)) {
    const normalized = item.trim();
    if (normalized) {
      deduped.add(normalized);
    }
  }
  return Array.from(deduped);
}

export function formatSelectorTerms(items: string[]): string {
  return items.join("\n");
}

function normalizeStringList(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value
    .map((item) => String(item || "").trim())
    .filter(Boolean);
}

function normalizeDualRuntimeCode(value: unknown, fallback = ""): string {
  return String(value || fallback || "").trim().toUpperCase();
}

export function buildDualRuntimeRequestPreviews(
  paramsText: string,
): DualRuntimePreviewResult {
  const sourceText = paramsText.trim();
  if (!sourceText) {
    return { requests: [], error: "" };
  }

  try {
    const parsed = JSON.parse(sourceText) as Record<string, unknown>;
    const timeoutMs = Number.isFinite(Number(parsed.timeoutMs))
      ? Math.max(1000, Math.round(Number(parsed.timeoutMs)))
      : 45000;
    const defaultSkipDefaultStartUrls = parsed.skipDefaultStartUrls !== false;

    const normalizeBrowser = (
      value: unknown,
      fallbackCode: string,
    ): DualRuntimeRequestPreview | null => {
      const raw =
        value && typeof value === "object"
          ? (value as Record<string, unknown>)
          : {};
      const directCode =
        typeof value === "string" || typeof value === "number" ? value : "";
      const code = normalizeDualRuntimeCode(
        raw.code ?? raw.launchCode ?? directCode,
        fallbackCode,
      );
      if (!code) {
        return null;
      }

      const skipDefaultStartUrls =
        raw.skipDefaultStartUrls !== undefined
          ? raw.skipDefaultStartUrls !== false
          : defaultSkipDefaultStartUrls;
      const startUrls = normalizeStringList(raw.startUrls);
      const launchArgs = normalizeStringList(raw.launchArgs);

      return {
        code,
        payload: {
          selector: { code, matchMode: "unique" },
          skipDefaultStartUrls,
          ...(startUrls.length > 0 ? { startUrls } : {}),
          ...(launchArgs.length > 0 ? { launchArgs } : {}),
          timeoutMs,
        },
      };
    };

    let requests = Array.isArray(parsed.browsers)
      ? parsed.browsers
          .map((item, index) =>
            normalizeBrowser(item, index === 0 ? "BUYER_001" : "BUYER_002"),
          )
          .filter((item): item is DualRuntimeRequestPreview => Boolean(item))
      : [];

    if (requests.length === 0) {
      requests = [
        normalizeBrowser(parsed.primaryCode, "BUYER_001"),
        normalizeBrowser(parsed.secondaryCode, "BUYER_002"),
      ].filter((item): item is DualRuntimeRequestPreview => Boolean(item));
    }

    return { requests, error: "" };
  } catch (error: unknown) {
    return {
      requests: [],
      error: error instanceof Error ? error.message : "JSON 解析失败",
    };
  }
}

export function buildRuntimeSessionHttpPreview(
  baseUrl: string,
  authHeader: string,
  payload: Record<string, unknown>,
): string {
  const lines = [
    `POST ${baseUrl}/api/runtime/session`,
    "Content-Type: application/json",
  ];

  if (authHeader) {
    lines.push(`${authHeader}: <your-api-key>`);
  }

  lines.push("", JSON.stringify(payload, null, 2));
  return lines.join("\n");
}

export function buildOpenClawDualSiteCommand(
  scriptID: string,
  codes: string[],
): string {
  const dedupedCodes = Array.from(
    new Set(
      codes
        .map((item) => item.trim().toUpperCase())
        .filter(Boolean),
    ),
  );
  const primaryCode = dedupedCodes[0] || "BUYER_001";
  const secondaryCode = dedupedCodes[1] || "BUYER_002";
  const targetScriptID = scriptID.trim() || DUAL_INSTANCE_RUNTIME_SCRIPT_ID;

  return [
    "使用 ant-chrome-openclaw skill。",
    `请由 OpenClaw 触发执行预置脚本 ${targetScriptID}。`,
    `参数里 browsers 使用 ${primaryCode} 和 ${secondaryCode}，并分别设置 startUrls。`,
    `${primaryCode} 的 startUrls 固定为 ["https://finance.sina.com.cn/"]。`,
    `${secondaryCode} 的 startUrls 固定为 ["https://map.baidu.com/"]。`,
    "必须直接在 runtime/session 请求中带 startUrls，一次启动到目标站点。",
    "不要先空启动 runtime/session 再调用 launch（否则会先出现 about:blank）。",
    "两个站点必须分开实例执行，不要混用会话，不要停止实例。",
    "返回两个实例各自的页面标题、当前 URL 和执行结果。",
  ].join("\n");
}

export function buildProfileSuggestions(
  profiles: BrowserProfile[],
  resolveValue: (profile: BrowserProfile) => string | undefined,
  resolveLabel: (profile: BrowserProfile) => string,
): SelectorSuggestion[] {
  const seen = new Set<string>();
  const suggestions: SelectorSuggestion[] = [];

  profiles.forEach((profile) => {
    const value = resolveValue(profile)?.trim();
    if (!value || seen.has(value)) {
      return;
    }
    seen.add(value);
    suggestions.push({
      key: profile.profileId,
      value,
      label: resolveLabel(profile),
    });
  });

  return suggestions.sort((left, right) =>
    left.value.localeCompare(right.value, "zh-CN"),
  );
}

export function buildGroupOptions(
  groups: BrowserGroupWithCount[],
): Array<{ value: string; label: string }> {
  const result: Array<{ value: string; label: string }> = [];

  const appendChildren = (parentId: string, level: number) => {
    groups
      .filter((group) => group.parentId === parentId)
      .sort((left, right) => left.sortOrder - right.sortOrder)
      .forEach((group) => {
        result.push({
          value: group.groupId,
          label: `${"\u3000".repeat(level)}${group.groupName}`,
        });
        appendChildren(group.groupId, level + 1);
      });
  };

  appendChildren("", 0);
  return result;
}

export function buildExactProfileOptions(
  profiles: BrowserProfile[],
  selector: AutomationScriptTargetSelector,
  placeholder: string,
): Array<{ value: string; label: string }> {
  const options = [
    { value: "", label: placeholder },
    ...profiles
      .slice()
      .sort((left, right) =>
        (left.profileName || left.profileId).localeCompare(
          right.profileName || right.profileId,
          "zh-CN",
        ),
      )
      .map((profile) => ({
        value: profile.profileId,
        label: [
          profile.launchCode || "",
          profile.profileName || profile.profileId,
        ]
          .filter(Boolean)
          .join(" · "),
      })),
  ];

  const currentProfileId = findMatchedProfileId(selector, profiles);
  const currentCode = selector.code.trim().toUpperCase();
  if (
    (currentProfileId || currentCode) &&
    !options.some((item) => item.value === currentProfileId)
  ) {
    options.push({
      value: currentProfileId,
      label: [currentCode, currentProfileId || "未绑定实例", "已不存在"]
        .filter(Boolean)
        .join(" · "),
    });
  }

  return options;
}

function hasTargetSelectorCondition(
  selector: AutomationScriptTargetSelector,
): boolean {
  return Boolean(
    selector.code ||
      selector.profileId ||
      selector.profileName ||
      selector.groupId ||
      selector.keywords.length > 0 ||
      selector.tags.length > 0,
  );
}

export function validateTargetConfig(
  config: AutomationScriptTargetConfig,
): string {
  switch (config.mode) {
    case "existing":
      return "";
    case "create":
      return hasTargetSelectorCondition(config.templateSelector)
        ? ""
        : "“按模板新建实例”至少要填一个模板条件";
    case "rotate":
      return hasTargetSelectorCondition(config.selector)
        ? ""
        : "“按条件轮询实例”至少要填一个轮询条件";
    default:
      return "";
  }
}

export function formatTargetSelectorSummary(
  selector: AutomationScriptTargetSelector,
  profiles: BrowserProfile[],
  fallback: string,
): string {
  const parts: string[] = [];
  const identity = formatAutomationTargetIdentity(selector, profiles, {
    fallback: "",
  }).trim();

  if (identity) {
    parts.push(identity);
  }
  if (selector.groupId.trim()) {
    parts.push(`分组 ${selector.groupId.trim()}`);
  }
  if (selector.tags.length > 0) {
    parts.push(`标签 ${selector.tags.join(" / ")}`);
  }
  if (selector.keywords.length > 0) {
    parts.push(`关键字 ${selector.keywords.join(" / ")}`);
  }

  return parts.length > 0 ? parts.join(" · ") : fallback;
}

export function findMatchedProfileId(
  selector: AutomationScriptTargetSelector,
  profiles: BrowserProfile[],
): string {
  const profile = findAutomationTargetProfile(selector, profiles);
  return String(profile?.profileId || selector.profileId || "").trim();
}
