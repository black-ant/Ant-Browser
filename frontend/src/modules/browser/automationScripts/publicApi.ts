import {
  AUTOMATION_SCRIPT_PUBLIC_API_BASE_PATH,
  AUTOMATION_SCRIPT_PUBLIC_API_DEFAULT_TIMEOUT_MS,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptPublicAPIRequestMode,
  type AutomationScriptPublicAPIResponseMode,
  type AutomationScriptRecord,
} from "./definitions";
import {
  normalizeAutomationScriptPublicAPIVariableList,
  safeParseAutomationScriptPublicAPIJSONObject,
} from "./publicApiUtils";

export {
  applyAutomationScriptPublicAPIVariables,
  collectAutomationScriptPublicAPIVariableValues,
  isAutomationScriptPublicAPIVariableName,
} from "./publicApiUtils";

export const AUTOMATION_SCRIPT_PUBLIC_API_METHOD_OPTIONS = [
  { value: "POST", label: "POST" },
] as const;

export const AUTOMATION_SCRIPT_PUBLIC_API_REQUEST_MODE_OPTIONS: Array<{
  value: AutomationScriptPublicAPIRequestMode;
  label: string;
}> = [
  { value: "standard", label: "标准请求" },
  { value: "params-only", label: "仅透传 params" },
];

export const AUTOMATION_SCRIPT_PUBLIC_API_RESPONSE_MODE_OPTIONS: Array<{
  value: AutomationScriptPublicAPIResponseMode;
  label: string;
}> = [
  { value: "envelope", label: "标准信封" },
  { value: "result-only", label: "仅返回 result" },
];

function normalizeAutomationScriptPublicAPIPathSegment(value: string): string {
  let result = "";
  let lastDash = false;

  for (const char of value.trim()) {
    const lower = char.toLowerCase();
    if (
      (lower >= "a" && lower <= "z") ||
      (lower >= "0" && lower <= "9")
    ) {
      result += lower;
      lastDash = false;
      continue;
    }
    if (lower === "-" || lower === "_" || lower === ".") {
      result += lower;
      lastDash = false;
      continue;
    }
    if (!lastDash) {
      result += "-";
      lastDash = true;
    }
  }

  return result.replace(/^-+|-+$/g, "");
}

function normalizeAutomationScriptPublicAPIPath(value: unknown): string {
  const source = String(value || "")
    .trim()
    .replace(/\\/g, "/");
  if (!source) {
    return "";
  }

  const lower = source.toLowerCase();
  const lowerBase = AUTOMATION_SCRIPT_PUBLIC_API_BASE_PATH.toLowerCase();
  const trimmed =
    lower.startsWith(`${lowerBase}/`)
      ? source.slice(AUTOMATION_SCRIPT_PUBLIC_API_BASE_PATH.length + 1)
      : lower.startsWith(`${lowerBase.slice(1)}/`)
        ? source.slice(AUTOMATION_SCRIPT_PUBLIC_API_BASE_PATH.length)
        : source;

  const segments = trimmed
    .split("/")
    .map((item) => normalizeAutomationScriptPublicAPIPathSegment(item))
    .filter(Boolean);

  return segments.join("/");
}

export function buildAutomationScriptPublicAPIPath(path: string): string {
  const normalizedPath = normalizeAutomationScriptPublicAPIPath(path);
  return normalizedPath
    ? `${AUTOMATION_SCRIPT_PUBLIC_API_BASE_PATH}/${normalizedPath}`
    : AUTOMATION_SCRIPT_PUBLIC_API_BASE_PATH;
}

export function suggestAutomationScriptPublicAPIPath(
  script: Pick<AutomationScriptRecord, "id" | "name">,
): string {
  const namePath = normalizeAutomationScriptPublicAPIPath(script.name);
  if (namePath) {
    return namePath;
  }
  return normalizeAutomationScriptPublicAPIPath(script.id);
}

export function createAutomationScriptPublicAPIConfig(): AutomationScriptPublicAPIConfig {
  return {
    enabled: false,
    method: "POST",
    path: "",
    requestMode: "standard",
    responseMode: "envelope",
    timeoutMs: AUTOMATION_SCRIPT_PUBLIC_API_DEFAULT_TIMEOUT_MS,
    requestBodyText: "",
    responseBodyText: "",
    variables: [],
  };
}

export function normalizeAutomationScriptPublicAPIConfig(
  config: unknown,
): AutomationScriptPublicAPIConfig {
  if (!config || typeof config !== "object") {
    return createAutomationScriptPublicAPIConfig();
  }

  const raw = config as Partial<AutomationScriptPublicAPIConfig>;
  const timeoutMs = Number.isFinite(Number(raw.timeoutMs))
    ? Math.round(Number(raw.timeoutMs))
    : AUTOMATION_SCRIPT_PUBLIC_API_DEFAULT_TIMEOUT_MS;

  return {
    enabled: raw.enabled === true,
    method: "POST",
    path: normalizeAutomationScriptPublicAPIPath(raw.path),
    requestMode: raw.requestMode === "params-only" ? "params-only" : "standard",
    responseMode:
      raw.responseMode === "result-only" ? "result-only" : "envelope",
    timeoutMs:
      timeoutMs < 1000
        ? 1000
        : timeoutMs > 30 * 60 * 1000
          ? 30 * 60 * 1000
          : timeoutMs,
    requestBodyText:
      typeof raw.requestBodyText === "string" ? raw.requestBodyText.trim() : "",
    responseBodyText:
      typeof raw.responseBodyText === "string"
        ? raw.responseBodyText.trim()
        : "",
    variables: normalizeAutomationScriptPublicAPIVariableList(
      (raw as { variables?: unknown }).variables,
    ),
  };
}

function isPlainAutomationJSONObject(
  value: unknown,
): value is Record<string, unknown> {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}

const AUTOMATION_SCRIPT_PUBLIC_API_INTERNAL_PARAM_KEYS = new Set([
  "pageUrl",
  "url",
  "selectors",
  "outputFileName",
  "captureScreenshot",
]);

function buildAutomationScriptPublicAPIExampleParams(
  script: Pick<AutomationScriptRecord, "paramsText">,
): Record<string, unknown> {
  const params = safeParseAutomationScriptPublicAPIJSONObject(script.paramsText) || {};
  const result: Record<string, unknown> = {};
  Object.entries(params).forEach(([key, value]) => {
    if (!AUTOMATION_SCRIPT_PUBLIC_API_INTERNAL_PARAM_KEYS.has(key)) {
      result[key] = value;
    }
  });
  return result;
}

function hasAutomationScriptPublicAPIExampleParams(
  script: Pick<AutomationScriptRecord, "paramsText">,
  params: Record<string, unknown>,
): boolean {
  const expectedParams = buildAutomationScriptPublicAPIExampleParams(script);
  return Object.keys(expectedParams).every((key) => key in params);
}

function buildAutomationScriptPublicAPIDefaultRequestExample(
  script: Pick<AutomationScriptRecord, "id" | "name" | "paramsText" | "selectorText">,
  config: AutomationScriptPublicAPIConfig,
): string {
  const params = buildAutomationScriptPublicAPIExampleParams(script);

  return JSON.stringify(
    {
      instance: {
        type: "script-default",
      },
      params,
      timeoutMs: config.timeoutMs,
    },
    null,
    2,
  );
}

function buildAutomationScriptPublicAPIDefaultResponseExample(): string {
  return JSON.stringify(
    {
      ok: true,
      status: "success",
      message: "已返回脚本结果",
      data: {
        verificationCode: "429792",
      },
    },
    null,
    2,
  );
}


function isLegacyAutomationScriptPublicAPIRequestExample(
  script: Pick<AutomationScriptRecord, "id" | "name" | "paramsText" | "selectorText">,
  parsedBody: Record<string, unknown>,
): boolean {
  const allowedLegacyKeys = new Set(["code", "launchCode", "param", "params", "timeoutMs"]);
  if (Object.keys(parsedBody).some((key) => !allowedLegacyKeys.has(key))) {
    return false;
  }

  const targetCode = String(parsedBody.code || parsedBody.launchCode || "").trim();
  if (targetCode) {
    return false;
  }

  const paramsValue = "param" in parsedBody ? parsedBody.param : parsedBody.params;
  if (paramsValue !== undefined && !isPlainAutomationJSONObject(paramsValue)) {
    return false;
  }

  if (
    paramsValue !== undefined &&
    !hasAutomationScriptPublicAPIExampleParams(script, paramsValue)
  ) {
    return false;
  }

  return true;
}

function shouldUseDerivedAutomationScriptPublicAPIRequestBody(
  script: Pick<AutomationScriptRecord, "id" | "name" | "paramsText" | "selectorText">,
  config: AutomationScriptPublicAPIConfig,
): boolean {
  const sourceText = config.requestBodyText.trim();
  if (!sourceText) {
    return true;
  }

  if (
    sourceText ===
    buildAutomationScriptPublicAPIDefaultRequestExample(script, {
      ...config,
      requestBodyText: "",
    })
  ) {
    return true;
  }

  const parsedBody = safeParseAutomationScriptPublicAPIJSONObject(sourceText);
  if (!parsedBody) {
    return false;
  }

  if (isLegacyAutomationScriptPublicAPIRequestExample(script, parsedBody)) {
    return true;
  }

  const allowedKeys = new Set(["code", "instance", "params", "timeoutMs"]);
  if (Object.keys(parsedBody).some((key) => !allowedKeys.has(key))) {
    return false;
  }

  if (!("code" in parsedBody) && !("instance" in parsedBody)) {
    return false;
  }

  const targetCode = String(parsedBody.code || "").trim();
  if (targetCode) {
    return false;
  }

  const instanceValue = parsedBody.instance;
  if (instanceValue !== undefined) {
    if (!isPlainAutomationJSONObject(instanceValue)) {
      return false;
    }
    if (String(instanceValue.type || "").trim() !== "script-default") {
      return false;
    }
  }

  const paramsValue = parsedBody.params;
  if (paramsValue !== undefined && !isPlainAutomationJSONObject(paramsValue)) {
    return false;
  }

  if (
    paramsValue !== undefined &&
    !hasAutomationScriptPublicAPIExampleParams(script, paramsValue)
  ) {
    return false;
  }

  if (
    "timeoutMs" in parsedBody &&
    !Number.isFinite(Number(parsedBody.timeoutMs))
  ) {
    return false;
  }

  return true;
}

function shouldUseDerivedAutomationScriptPublicAPIResponseBody(
  config: AutomationScriptPublicAPIConfig,
): boolean {
  const sourceText = config.responseBodyText.trim();
  if (!sourceText) {
    return true;
  }

  return (
    sourceText ===
    buildAutomationScriptPublicAPIDefaultResponseExample()
  );
}

export function buildAutomationScriptPublicAPIRequestExample(
  script: Pick<AutomationScriptRecord, "id" | "name" | "paramsText" | "selectorText">,
  config: AutomationScriptPublicAPIConfig,
): string {
  if (!shouldUseDerivedAutomationScriptPublicAPIRequestBody(script, config)) {
    return config.requestBodyText.trim();
  }

  return buildAutomationScriptPublicAPIDefaultRequestExample(script, {
    ...config,
    requestBodyText: "",
  });
}

export function buildAutomationScriptPublicAPIResponseExample(
  _script: Pick<AutomationScriptRecord, "id" | "name">,
  config: AutomationScriptPublicAPIConfig,
): string {
  if (!shouldUseDerivedAutomationScriptPublicAPIResponseBody(config)) {
    return config.responseBodyText.trim();
  }

  return buildAutomationScriptPublicAPIDefaultResponseExample();
}

export function prepareAutomationScriptPublicAPIConfigForSave(
  script: Pick<
    AutomationScriptRecord,
    "id" | "name" | "paramsText" | "selectorText"
  > & {
    publicAPI: unknown;
  },
): AutomationScriptPublicAPIConfig {
  const config = normalizeAutomationScriptPublicAPIConfig(script.publicAPI);

  return {
    ...config,
    requestBodyText: shouldUseDerivedAutomationScriptPublicAPIRequestBody(
      script,
      config,
    )
      ? ""
      : config.requestBodyText.trim(),
    responseBodyText: shouldUseDerivedAutomationScriptPublicAPIResponseBody(config)
      ? ""
      : config.responseBodyText.trim(),
  };
}

export function resolveAutomationScriptPublicAPIConfig(
  script: Pick<
    AutomationScriptRecord,
    "id" | "name" | "paramsText" | "selectorText" | "publicAPI"
  >,
): AutomationScriptPublicAPIConfig {
  const config = prepareAutomationScriptPublicAPIConfigForSave(script);
  const path = config.path.trim()
    ? config.path
    : suggestAutomationScriptPublicAPIPath(script);
  const requestBodyText = buildAutomationScriptPublicAPIRequestExample(script, {
    ...config,
    path,
    requestBodyText: config.requestBodyText,
  });
  const responseBodyText = buildAutomationScriptPublicAPIResponseExample(
    script,
    {
      ...config,
      path,
      responseBodyText: config.responseBodyText,
    },
  );

  return {
    ...config,
    path,
    requestBodyText,
    responseBodyText,
  };
}

export function getAutomationScriptPublicAPIRequestModeLabel(
  mode: AutomationScriptPublicAPIRequestMode,
): string {
  return (
    AUTOMATION_SCRIPT_PUBLIC_API_REQUEST_MODE_OPTIONS.find(
      (item) => item.value === mode,
    )?.label || mode
  );
}

export function getAutomationScriptPublicAPIResponseModeLabel(
  mode: AutomationScriptPublicAPIResponseMode,
): string {
  return (
    AUTOMATION_SCRIPT_PUBLIC_API_RESPONSE_MODE_OPTIONS.find(
      (item) => item.value === mode,
    )?.label || mode
  );
}
