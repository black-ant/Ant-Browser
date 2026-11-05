import {
  AUTOMATION_SCRIPT_MANIFEST_VERSION,
  AUTOMATION_SCRIPT_PACKAGE_FORMAT,
  DUAL_INSTANCE_RUNTIME_SCRIPT_ID,
  type AutomationScriptRecord,
  type AutomationScriptType,
} from "./automationScripts/definitions";
import {
  createAutomationScriptPublicAPIConfig,
  prepareAutomationScriptPublicAPIConfigForSave,
} from "./automationScripts/publicApi";
import {
  normalizeAutomationScriptTargetConfig,
} from "./automationScripts/targets";
import {
  buildDefaultAutomationScripts,
  buildNotesTemplate,
  buildParamsTemplate,
  buildScriptTemplate,
  buildSelectorTemplate,
  createDualInstanceRuntimeScriptDraft,
  normalizeDualInstanceRuntimeParamsText,
} from "./automationScripts/builtins";
import {
  normalizeAutomationScriptSource,
} from "./automationScripts/metadata";

export * from "./automationScripts/definitions";
export {
  AUTOMATION_SCRIPT_PUBLIC_API_METHOD_OPTIONS,
  AUTOMATION_SCRIPT_PUBLIC_API_REQUEST_MODE_OPTIONS,
  AUTOMATION_SCRIPT_PUBLIC_API_RESPONSE_MODE_OPTIONS,
  applyAutomationScriptPublicAPIVariables,
  buildAutomationScriptPublicAPIPath,
  buildAutomationScriptPublicAPIRequestExample,
  buildAutomationScriptPublicAPIResponseExample,
  collectAutomationScriptPublicAPIVariableValues,
  createAutomationScriptPublicAPIConfig,
  getAutomationScriptPublicAPIRequestModeLabel,
  getAutomationScriptPublicAPIResponseModeLabel,
  isAutomationScriptPublicAPIVariableName,
  normalizeAutomationScriptPublicAPIConfig,
  prepareAutomationScriptPublicAPIConfigForSave,
  resolveAutomationScriptPublicAPIConfig,
  suggestAutomationScriptPublicAPIPath,
} from "./automationScripts/publicApi";
export {
  buildAutomationScriptPublicAPIRequestBodyWithTargetCode,
  normalizeAutomationScriptPublicAPIRequestBodyForInvoke,
  readAutomationScriptPublicAPIInstanceType,
  readAutomationScriptPublicAPIParamObject,
  readAutomationScriptPublicAPITargetCode,
} from "./automationScripts/publicApiInstances";
export {
  canRefreshAutomationScriptSource,
  getAutomationScriptRefreshLabel,
  getAutomationScriptSourceLabel,
  getAutomationScriptStatusLabel,
  getAutomationScriptTypeLabel,
  normalizeAutomationScriptSource,
} from "./automationScripts/metadata";
export {
  createAutomationScriptTargetSelector,
  describeAutomationScriptTargetConfig,
  findAutomationTargetProfile,
  formatAutomationTargetIdentity,
  getAutomationScriptTargetModeLabel,
  normalizeAutomationScriptTargetConfig,
  normalizeAutomationScriptTargetSelector,
} from "./automationScripts/targets";

const AUTOMATION_SCRIPTS_STORAGE_KEY = "automation_scripts_v1";
function nowIso(): string {
  return new Date().toISOString();
}

function createScriptId(): string {
  if (
    typeof crypto !== "undefined" &&
    typeof crypto.randomUUID === "function"
  ) {
    return crypto.randomUUID();
  }
  return `script-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function normalizeTags(tags: unknown): string[] {
  if (!Array.isArray(tags)) {
    return [];
  }

  const deduped = new Set<string>();
  for (const item of tags) {
    const normalized = String(item || "").trim();
    if (normalized) {
      deduped.add(normalized);
    }
  }
  return Array.from(deduped);
}

function normalizeScriptRecord(raw: unknown): AutomationScriptRecord | null {
  if (!raw || typeof raw !== "object") {
    return null;
  }

  const source = raw as Partial<AutomationScriptRecord>;
  const type = source.type === "launch-api" ? "launch-api" : "playwright-cdp";
  const status =
    source.status === "ready" || source.status === "disabled"
      ? source.status
      : "draft";
  const createdAt =
    typeof source.createdAt === "string" && source.createdAt.trim()
      ? source.createdAt
      : nowIso();
  const updatedAt =
    typeof source.updatedAt === "string" && source.updatedAt.trim()
      ? source.updatedAt
      : createdAt;
  const normalizedSource = normalizeAutomationScriptSource(source.source);
  const normalizedTargetConfig = normalizeAutomationScriptTargetConfig(
    source.targetConfig,
  );
  const record: AutomationScriptRecord = {
    packageFormat:
      typeof source.packageFormat === "string" && source.packageFormat.trim()
        ? source.packageFormat.trim()
        : AUTOMATION_SCRIPT_PACKAGE_FORMAT,
    manifestVersion:
      typeof source.manifestVersion === "number" && source.manifestVersion > 0
        ? source.manifestVersion
        : AUTOMATION_SCRIPT_MANIFEST_VERSION,
    id:
      typeof source.id === "string" && source.id.trim()
        ? source.id
        : createScriptId(),
    name:
      typeof source.name === "string" && source.name.trim()
        ? source.name.trim()
        : "未命名脚本",
    description:
      typeof source.description === "string" ? source.description.trim() : "",
    type,
    status,
    entryFile:
      typeof source.entryFile === "string" && source.entryFile.trim()
        ? source.entryFile.trim()
        : "index.cjs",
    tags: normalizeTags(source.tags),
    selectorText:
      typeof source.selectorText === "string" && source.selectorText.trim()
        ? source.selectorText
        : buildSelectorTemplate(type),
    paramsText:
      typeof source.paramsText === "string" && source.paramsText.trim()
        ? source.paramsText
        : buildParamsTemplate(type),
    scriptText:
      typeof source.scriptText === "string" && source.scriptText.trim()
        ? source.scriptText
        : buildScriptTemplate(type),
    notes:
      typeof source.notes === "string" && source.notes.trim()
        ? source.notes
        : buildNotesTemplate(type),
    targetConfig: normalizedTargetConfig,
    publicAPI: createAutomationScriptPublicAPIConfig(),
    source: normalizedSource,
    createdAt,
    updatedAt,
  };

  record.publicAPI = prepareAutomationScriptPublicAPIConfigForSave({
    id: record.id,
    name: record.name,
    selectorText: record.selectorText,
    paramsText: record.paramsText,
    publicAPI: source.publicAPI ?? null,
  });

  if (record.id === DUAL_INSTANCE_RUNTIME_SCRIPT_ID) {
    const dualInstanceDraft = createDualInstanceRuntimeScriptDraft();
    const usesLegacyDualInstanceScript =
      record.scriptText.includes("params.primaryCode") ||
      record.scriptText.includes("params.secondaryCode");

    return {
      ...record,
      selectorText: "",
      paramsText: normalizeDualInstanceRuntimeParamsText(record.paramsText),
      targetConfig: normalizeAutomationScriptTargetConfig(null),
      scriptText: usesLegacyDualInstanceScript
        ? dualInstanceDraft.scriptText
        : record.scriptText,
    };
  }

  return record;
}

export function normalizeAutomationScriptRecordPayload(
  raw: unknown,
): AutomationScriptRecord | null {
  return normalizeScriptRecord(raw);
}

export function createAutomationScriptDraft(
  type: AutomationScriptType = "playwright-cdp",
): AutomationScriptRecord {
  const createdAt = nowIso();
  const name =
    type === "launch-api" ? "新建 Launch API 脚本" : "百度搜索示例";
  const description =
    type === "launch-api"
      ? ""
      : "启动示例实例，打开百度并搜索关键词，用来验证 Launch API + Playwright CDP 链路。";

  return {
    packageFormat: AUTOMATION_SCRIPT_PACKAGE_FORMAT,
    manifestVersion: AUTOMATION_SCRIPT_MANIFEST_VERSION,
    id: createScriptId(),
    name,
    description,
    type,
    status: type === "playwright-cdp" ? "ready" : "draft",
    entryFile: "index.cjs",
    tags: type === "launch-api" ? ["HTTP"] : ["Playwright", "示例"],
    selectorText: buildSelectorTemplate(type),
    paramsText: buildParamsTemplate(type),
    scriptText: buildScriptTemplate(type),
    notes: buildNotesTemplate(type),
    targetConfig: normalizeAutomationScriptTargetConfig(null),
    publicAPI: createAutomationScriptPublicAPIConfig(),
    source: {
      type: "manual",
      uri: "",
      ref: "",
      path: "",
      importedAt: "",
    },
    createdAt,
    updatedAt: createdAt,
  };
}

export function duplicateAutomationScript(
  script: AutomationScriptRecord,
): AutomationScriptRecord {
  const createdAt = nowIso();
  return {
    ...script,
    id: createScriptId(),
    name: `${script.name} - 副本`,
    status: "draft",
    publicAPI: {
      ...createAutomationScriptPublicAPIConfig(),
      requestBodyText: script.publicAPI.requestBodyText,
      responseBodyText: script.publicAPI.responseBodyText,
      variables: script.publicAPI.variables.map((variable) => ({ ...variable })),
    },
    createdAt,
    updatedAt: createdAt,
  };
}

function stringifyJsonBlock(value: unknown, fallback: string): string {
  if (typeof value === "string" && value.trim()) {
    return value;
  }

  if (value && typeof value === "object") {
    try {
      return JSON.stringify(value, null, 2);
    } catch {
      return fallback;
    }
  }

  return fallback;
}

export function importAutomationScript(text: string): AutomationScriptRecord {
  const normalized = text.trim();
  if (!normalized) {
    throw new Error("导入内容不能为空");
  }

  let parsed: any;
  try {
    parsed = JSON.parse(normalized);
  } catch {
    throw new Error("导入内容不是合法 JSON");
  }

  const manifest =
    parsed?.manifest && typeof parsed.manifest === "object"
      ? parsed.manifest
      : parsed;
  const type: AutomationScriptType =
    manifest?.type === "launch-api" ? "launch-api" : "playwright-cdp";
  const timestamp = nowIso();
  const imported = normalizeScriptRecord({
    packageFormat:
      typeof parsed?.packageFormat === "string"
        ? parsed.packageFormat
        : typeof parsed?.format === "string"
          ? parsed.format
          : AUTOMATION_SCRIPT_PACKAGE_FORMAT,
    manifestVersion:
      typeof parsed?.manifestVersion === "number"
        ? parsed.manifestVersion
        : AUTOMATION_SCRIPT_MANIFEST_VERSION,
    id: createScriptId(),
    name: typeof manifest?.name === "string" ? manifest.name : undefined,
    description:
      typeof manifest?.description === "string" ? manifest.description : "",
    type,
    status: "draft",
    entryFile:
      typeof manifest?.entryFile === "string"
        ? manifest.entryFile
        : "index.cjs",
    tags: Array.isArray(manifest?.tags) ? manifest.tags : [],
    selectorText: stringifyJsonBlock(
      parsed?.selector ?? parsed?.selectorText,
      buildSelectorTemplate(type),
    ),
    paramsText: stringifyJsonBlock(
      parsed?.params ?? parsed?.paramsText,
      buildParamsTemplate(type),
    ),
    scriptText:
      typeof parsed?.script === "string"
        ? parsed.script
        : typeof parsed?.scriptText === "string"
          ? parsed.scriptText
          : buildScriptTemplate(type),
    notes:
      typeof parsed?.notes === "string"
        ? parsed.notes
        : typeof parsed?.manifest?.notes === "string"
          ? parsed.manifest.notes
          : buildNotesTemplate(type),
    targetConfig:
      parsed?.targetConfig && typeof parsed.targetConfig === "object"
        ? parsed.targetConfig
        : parsed?.manifest?.targetConfig &&
            typeof parsed.manifest.targetConfig === "object"
          ? parsed.manifest.targetConfig
          : null,
    publicAPI:
      parsed?.publicAPI && typeof parsed.publicAPI === "object"
        ? parsed.publicAPI
        : parsed?.manifest?.publicAPI &&
            typeof parsed.manifest.publicAPI === "object"
          ? parsed.manifest.publicAPI
          : null,
    source:
      parsed?.source && typeof parsed.source === "object"
        ? parsed.source
        : {
            type: "text",
            uri: "",
            ref: "",
            path: "",
            importedAt: timestamp,
          },
    createdAt: timestamp,
    updatedAt: timestamp,
  });

  if (!imported) {
    throw new Error("导入内容无法识别为脚本");
  }

  return imported;
}

function buildDefaultScripts(): AutomationScriptRecord[] {
  return buildDefaultAutomationScripts();
}

function sortScripts(
  items: AutomationScriptRecord[],
): AutomationScriptRecord[] {
  return [...items].sort((left, right) => {
    return (
      new Date(right.updatedAt).getTime() - new Date(left.updatedAt).getTime()
    );
  });
}

export function loadAutomationScripts(): AutomationScriptRecord[] {
  if (typeof window === "undefined" || !window.localStorage) {
    return buildDefaultScripts();
  }

  try {
    const raw = window.localStorage.getItem(AUTOMATION_SCRIPTS_STORAGE_KEY);
    if (!raw) {
      return buildDefaultScripts();
    }

    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return buildDefaultScripts();
    }

    const scripts = parsed
      .map((item) => normalizeScriptRecord(item))
      .filter((item): item is AutomationScriptRecord => item !== null);

    if (scripts.length === 0) {
      return buildDefaultScripts();
    }

    return sortScripts(scripts);
  } catch {
    return buildDefaultScripts();
  }
}

export function saveAutomationScripts(scripts: AutomationScriptRecord[]) {
  if (typeof window === "undefined" || !window.localStorage) {
    return;
  }

  try {
    window.localStorage.setItem(
      AUTOMATION_SCRIPTS_STORAGE_KEY,
      JSON.stringify(sortScripts(scripts)),
    );
  } catch {
    // Ignore storage failures and keep the editor usable.
  }
}

export function exportAutomationScript(script: AutomationScriptRecord): string {
  return JSON.stringify(
    {
      manifest: {
        packageFormat: script.packageFormat,
        manifestVersion: script.manifestVersion,
        id: script.id,
        name: script.name,
        description: script.description,
        type: script.type,
        status: script.status,
        entryFile: script.entryFile,
        tags: script.tags,
        notes: script.notes,
        targetConfig: script.targetConfig,
        publicAPI: script.publicAPI,
        source: script.source,
        createdAt: script.createdAt,
        updatedAt: script.updatedAt,
      },
      format: script.packageFormat,
      manifestVersion: script.manifestVersion,
      selector: safeParseJson(script.selectorText),
      params: safeParseJson(script.paramsText),
      script: script.scriptText,
      notes: script.notes,
      targetConfig: script.targetConfig,
      publicAPI: script.publicAPI,
      source: script.source,
    },
    null,
    2,
  );
}

function safeParseJson(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}
