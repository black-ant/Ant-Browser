import {
  applyAutomationScriptPublicAPIVariables,
  collectAutomationScriptPublicAPIVariableValues,
  isAutomationScriptPublicAPIVariableName,
  prepareAutomationScriptPublicAPIConfigForSave,
  resolveAutomationScriptPublicAPIConfig,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptRecord,
} from "../../automationScripts";

function isJSONObjectText(value: string): boolean {
  const normalized = value.trim();
  if (!normalized) {
    return true;
  }

  try {
    const parsed = JSON.parse(normalized);
    return Boolean(parsed && typeof parsed === "object" && !Array.isArray(parsed));
  } catch {
    return false;
  }
}

function isJSONText(value: string): boolean {
  const normalized = value.trim();
  if (!normalized) {
    return true;
  }

  try {
    JSON.parse(normalized);
    return true;
  } catch {
    return false;
  }
}

export function validatePublicAPIConfig(
  config: AutomationScriptPublicAPIConfig,
): string {
  if (!config.enabled) {
    return "";
  }
  if (!config.path.trim()) {
    return "已启用对外接口时，Path 不能为空";
  }

  const invalidVariableNames = config.variables
    .filter((variable) => !isAutomationScriptPublicAPIVariableName(variable.name))
    .map((variable) => variable.name);
  if (invalidVariableNames.length) {
    return `变量名不合法：${invalidVariableNames.join(", ")}`;
  }

  const resolvedBody = applyAutomationScriptPublicAPIVariables(
    config.requestBodyText,
    config.variables,
    collectAutomationScriptPublicAPIVariableValues(config),
  );
  if (resolvedBody.missingRequired.length) {
    return `必填变量缺少默认值：${resolvedBody.missingRequired.join(", ")}`;
  }
  if (!isJSONObjectText(resolvedBody.bodyText)) {
    return "替换变量后的请求 Body 必须是 JSON 对象";
  }
  if (!isJSONText(config.responseBodyText)) {
    return "响应示例必须是合法 JSON";
  }
  return "";
}

export function hasSamePublicAPIConfig(
  left: AutomationScriptPublicAPIConfig,
  right: AutomationScriptPublicAPIConfig,
): boolean {
  return (
    left.enabled === right.enabled &&
    left.method === right.method &&
    left.path === right.path &&
    left.requestMode === right.requestMode &&
    left.responseMode === right.responseMode &&
    left.timeoutMs === right.timeoutMs &&
    left.requestBodyText === right.requestBodyText &&
    left.responseBodyText === right.responseBodyText &&
    JSON.stringify(left.variables) === JSON.stringify(right.variables)
  );
}

export function preparePublicAPIConfigForCompare(
  script: AutomationScriptRecord,
  publicAPI: AutomationScriptPublicAPIConfig = script.publicAPI,
): AutomationScriptPublicAPIConfig {
  return prepareAutomationScriptPublicAPIConfigForSave({
    ...script,
    publicAPI,
  });
}

export function buildPersistablePublicAPIConfig(
  script: AutomationScriptRecord,
  publicAPI: AutomationScriptPublicAPIConfig = script.publicAPI,
): AutomationScriptPublicAPIConfig {
  return prepareAutomationScriptPublicAPIConfigForSave({
    ...script,
    publicAPI: resolveAutomationScriptPublicAPIConfig({
      ...script,
      publicAPI,
    }),
  });
}
