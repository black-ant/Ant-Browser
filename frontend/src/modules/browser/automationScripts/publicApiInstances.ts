import { safeParseAutomationScriptPublicAPIJSONObject } from "./publicApiUtils";

function normalizeLaunchCode(value: unknown): string {
  return String(value || "").trim().toUpperCase();
}

function isPlainJSONObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}

export function readAutomationScriptPublicAPIParamObject(
  body: Record<string, unknown>,
): Record<string, unknown> {
  if (isPlainJSONObject(body.param)) {
    return body.param;
  }
  if (isPlainJSONObject(body.params)) {
    return body.params;
  }
  return {};
}

export function readAutomationScriptPublicAPITargetCode(bodyText: string): string {
  const body = safeParseAutomationScriptPublicAPIJSONObject(bodyText);
  if (!body) return "";
  const instance = isPlainJSONObject(body.instance) ? body.instance : null;
  const selector = isPlainJSONObject(instance?.selector) ? instance.selector : null;
  if (selector?.code) {
    return normalizeLaunchCode(selector.code);
  }
  return normalizeLaunchCode(body.code || body.launchCode);
}

export function readAutomationScriptPublicAPIInstanceType(bodyText: string): string {
  const body = safeParseAutomationScriptPublicAPIJSONObject(bodyText);
  if (!body) return "";
  const instance = isPlainJSONObject(body.instance) ? body.instance : null;
  return String(instance?.type || "").trim();
}

export function normalizeAutomationScriptPublicAPIRequestBodyForInvoke(
  bodyText: string,
): string {
  const body = safeParseAutomationScriptPublicAPIJSONObject(bodyText);
  if (!body) {
    return bodyText;
  }

  const params = readAutomationScriptPublicAPIParamObject(body);
  const topLevelParams: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(body)) {
    if (
      [
        "code",
        "launchCode",
        "instance",
        "params",
        "param",
        "timeoutMs",
        "selector",
      ].includes(key)
    ) {
      continue;
    }
    topLevelParams[key] = value;
  }

  const nextBody: Record<string, unknown> = {
    params: {
      ...params,
      ...topLevelParams,
    },
  };

  if (isPlainJSONObject(body.instance)) {
    nextBody.instance = body.instance;
  } else {
    const code = normalizeLaunchCode(body.code || body.launchCode);
    if (code) {
      nextBody.instance = {
        type: "existing",
        selector: { code },
      };
    }
  }

  if (Number.isFinite(Number(body.timeoutMs))) {
    nextBody.timeoutMs = Math.round(Number(body.timeoutMs));
  }

  return JSON.stringify(nextBody, null, 2);
}

export function buildAutomationScriptPublicAPIRequestBodyWithTargetCode(
  currentBodyText: string,
  fallbackBodyText: string,
  code: string,
): string {
  const sourceBody =
    safeParseAutomationScriptPublicAPIJSONObject(currentBodyText) ||
    safeParseAutomationScriptPublicAPIJSONObject(fallbackBodyText) ||
    {};
  const sourceParam = readAutomationScriptPublicAPIParamObject(sourceBody);
  const nextBody: Record<string, unknown> = {
    ...sourceBody,
    instance: {
      type: "existing",
      selector: {
        code: normalizeLaunchCode(code),
      },
    },
    params: sourceParam,
  };
  delete nextBody.code;
  delete nextBody.launchCode;
  delete nextBody.selector;
  delete nextBody.param;

  return JSON.stringify(nextBody, null, 2);
}
