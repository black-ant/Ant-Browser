import type { BrowserProfile } from "../types";
import {
  AUTOMATION_SCRIPT_TARGET_MODE_OPTIONS,
  type AutomationScriptTargetConfig,
  type AutomationScriptTargetMode,
  type AutomationScriptTargetSelector,
} from "./definitions";

function normalizeTargetTerms(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const deduped = new Set<string>();
  for (const item of value) {
    const normalized = String(item || "").trim();
    if (normalized) {
      deduped.add(normalized);
    }
  }
  return Array.from(deduped);
}

export function normalizeAutomationScriptTargetSelector(
  selector: unknown,
): AutomationScriptTargetSelector {
  if (!selector || typeof selector !== "object") {
    return createAutomationScriptTargetSelector();
  }

  const raw = selector as Partial<AutomationScriptTargetSelector>;
  return {
    code:
      typeof raw.code === "string"
        ? raw.code.trim().toUpperCase()
        : typeof (selector as { launchCode?: unknown }).launchCode === "string"
          ? String((selector as { launchCode?: unknown }).launchCode)
              .trim()
              .toUpperCase()
          : "",
    profileId:
      typeof raw.profileId === "string" ? raw.profileId.trim() : "",
    profileName:
      typeof raw.profileName === "string" ? raw.profileName.trim() : "",
    groupId: typeof raw.groupId === "string" ? raw.groupId.trim() : "",
    keywords: normalizeTargetTerms(raw.keywords),
    tags: normalizeTargetTerms(raw.tags),
  };
}

export function createAutomationScriptTargetSelector(): AutomationScriptTargetSelector {
  return {
    code: "",
    profileId: "",
    profileName: "",
    groupId: "",
    keywords: [],
    tags: [],
  };
}

export function normalizeAutomationScriptTargetConfig(
  config: unknown,
): AutomationScriptTargetConfig {
  if (!config || typeof config !== "object") {
    return {
      mode: "manual",
      selector: createAutomationScriptTargetSelector(),
      templateSelector: createAutomationScriptTargetSelector(),
      createNameTemplate: "",
    };
  }

  const raw = config as Partial<AutomationScriptTargetConfig>;
  const mode: AutomationScriptTargetMode =
    raw.mode === "existing" ||
    raw.mode === "create" ||
    raw.mode === "rotate"
      ? raw.mode
      : "manual";

  return {
    mode,
    selector: normalizeAutomationScriptTargetSelector(raw.selector),
    templateSelector: normalizeAutomationScriptTargetSelector(
      raw.templateSelector,
    ),
    createNameTemplate:
      typeof raw.createNameTemplate === "string"
        ? raw.createNameTemplate.trim()
        : "",
  };
}

function selectorSummaryParts(selector: AutomationScriptTargetSelector): string[] {
  const parts: string[] = [];
  if (selector.code) {
    parts.push(`Code=${selector.code}`);
  }
  if (selector.profileName) {
    parts.push(`实例=${selector.profileName}`);
  }
  if (selector.profileId && !selector.code) {
    parts.push(`实例ID=${selector.profileId}`);
  }
  if (selector.groupId) {
    parts.push(`分组=${selector.groupId}`);
  }
  if (selector.tags.length > 0) {
    parts.push(`标签=${selector.tags.join(" / ")}`);
  }
  if (selector.keywords.length > 0) {
    parts.push(`关键字=${selector.keywords.join(" / ")}`);
  }
  return parts;
}

export function getAutomationScriptTargetModeLabel(
  mode: AutomationScriptTargetMode,
): string {
  return (
    AUTOMATION_SCRIPT_TARGET_MODE_OPTIONS.find((item) => item.value === mode)
      ?.label || mode
  );
}

function normalizeSelectorCode(value?: string): string {
  return String(value || "")
    .trim()
    .toUpperCase();
}

function normalizeSelectorText(value?: string): string {
  return String(value || "").trim();
}

export function findAutomationTargetProfile(
  selector: AutomationScriptTargetSelector,
  profiles: BrowserProfile[],
): BrowserProfile | null {
  const normalizedProfileId = normalizeSelectorText(selector.profileId);
  if (normalizedProfileId) {
    const matchedById = profiles.find(
      (profile) => normalizeSelectorText(profile.profileId) === normalizedProfileId,
    );
    if (matchedById) {
      return matchedById;
    }
  }

  const normalizedCode = normalizeSelectorCode(selector.code);
  if (normalizedCode) {
    const matchedByCode = profiles.find(
      (profile) => normalizeSelectorCode(profile.launchCode) === normalizedCode,
    );
    if (matchedByCode) {
      return matchedByCode;
    }
  }

  const normalizedProfileName = normalizeSelectorText(selector.profileName);
  if (normalizedProfileName) {
    const matchedByName = profiles.find(
      (profile) =>
        normalizeSelectorText(profile.profileName).toLowerCase() ===
        normalizedProfileName.toLowerCase(),
    );
    if (matchedByName) {
      return matchedByName;
    }
  }

  return null;
}

export function formatAutomationTargetIdentity(
  selector: AutomationScriptTargetSelector,
  profiles: BrowserProfile[],
  options?: {
    includeProfileId?: boolean;
    fallback?: string;
  },
): string {
  const profile = findAutomationTargetProfile(selector, profiles);
  const code = normalizeSelectorCode(profile?.launchCode || selector.code);
  const profileName = normalizeSelectorText(
    profile?.profileName || selector.profileName,
  );
  const profileId = normalizeSelectorText(profile?.profileId || selector.profileId);

  const parts = [code, profileName].filter(Boolean);
  if (options?.includeProfileId && profileId) {
    parts.push(profileId);
  }

  if (parts.length > 0) {
    return parts.join(" · ");
  }
  if (profileId) {
    return options?.includeProfileId ? profileId : `实例 ID ${profileId}`;
  }

  return options?.fallback || "-";
}

export function describeAutomationScriptTargetConfig(
  config: AutomationScriptTargetConfig,
): string {
  switch (config.mode) {
    case "existing": {
      const parts = selectorSummaryParts(config.selector);
      return parts.length > 0
        ? `传入实例：${parts.join(" · ")}`
        : "传入实例";
    }
    case "create": {
      const parts = selectorSummaryParts(config.templateSelector);
      const namePart = config.createNameTemplate
        ? `命名=${config.createNameTemplate}`
        : "";
      return ["按模板新建实例", ...parts, namePart]
        .filter(Boolean)
        .join(" · ");
    }
    case "rotate": {
      const parts = selectorSummaryParts(config.selector);
      return parts.length > 0
        ? `按条件轮询实例：${parts.join(" · ")}`
        : "按条件轮询实例";
    }
    default:
      return "传入实例";
  }
}
