import type {
  AutomationScriptPublicAPIConfig,
  AutomationScriptPublicAPIVariable,
} from "./definitions";

export function parseAutomationScriptPublicAPIJSONText(
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

export function safeParseAutomationScriptPublicAPIJSONObject(
  text: string,
): Record<string, unknown> | null {
  const parsed = parseAutomationScriptPublicAPIJSONText(text);
  if (!parsed.ok || !parsed.value || typeof parsed.value !== "object") {
    return null;
  }
  if (Array.isArray(parsed.value)) {
    return null;
  }
  return parsed.value as Record<string, unknown>;
}

export function stringifyAutomationScriptPublicAPIJSONBlock(
  value: unknown,
  fallback: string,
): string {
  if (typeof value === "string" && value.trim()) {
    return value.trim();
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

function normalizeAutomationScriptPublicAPIVariables(
  value: unknown,
): AutomationScriptPublicAPIVariable[] {
  const rawItems: unknown[] = Array.isArray(value)
    ? value
    : value && typeof value === "object"
      ? Object.entries(value as Record<string, unknown>).map(
          ([name, rawValue]) => {
            if (
              rawValue &&
              typeof rawValue === "object" &&
              !Array.isArray(rawValue)
            ) {
              return { name, ...(rawValue as Record<string, unknown>) };
            }
            return { name, defaultValue: rawValue };
          },
        )
      : [];

  const seen = new Set<string>();
  const variables: AutomationScriptPublicAPIVariable[] = [];
  for (const item of rawItems) {
    if (!item || typeof item !== "object" || Array.isArray(item)) {
      continue;
    }
    const raw = item as Record<string, unknown>;
    const name = String(raw.name ?? raw.key ?? "").trim();
    if (!name || seen.has(name)) {
      continue;
    }
    seen.add(name);
    variables.push({
      name,
      defaultValue: String(
        raw.defaultValue ?? raw.default ?? raw.value ?? "",
      ).trim(),
      description: String(
        raw.description ?? raw.label ?? raw.note ?? "",
      ).trim(),
      required:
        raw.required === true ||
        String(raw.required ?? "").trim().toLowerCase() === "true",
    });
  }
  return variables;
}

export function normalizeAutomationScriptPublicAPIVariableList(
  value: unknown,
): AutomationScriptPublicAPIVariable[] {
  return normalizeAutomationScriptPublicAPIVariables(value);
}

export function isAutomationScriptPublicAPIVariableName(value: string): boolean {
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(value.trim());
}

function escapeAutomationScriptPublicAPIRegex(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function escapeAutomationScriptPublicAPIJSONStringValue(value: string): string {
  const encoded = JSON.stringify(String(value ?? ""));
  return encoded.slice(1, -1);
}

export function collectAutomationScriptPublicAPIVariableValues(
  config: AutomationScriptPublicAPIConfig,
): Record<string, string> {
  return config.variables.reduce<Record<string, string>>((values, variable) => {
    values[variable.name] = variable.defaultValue;
    return values;
  }, {});
}

export function applyAutomationScriptPublicAPIVariables(
  text: string,
  variables: AutomationScriptPublicAPIVariable[],
  values: Record<string, string> = {},
): { bodyText: string; missingRequired: string[]; usedVariables: string[] } {
  const sourceText = String(text || "");
  let bodyText = sourceText;
  const missingRequired: string[] = [];
  const usedVariables: string[] = [];

  for (const variable of variables) {
    if (!isAutomationScriptPublicAPIVariableName(variable.name)) {
      continue;
    }
    const rawValue = values[variable.name] ?? variable.defaultValue ?? "";
    const value = String(rawValue);
    const placeholders = [`\${${variable.name}}`, `{{${variable.name}}}`];
    const used = placeholders.some((placeholder) =>
      sourceText.includes(placeholder),
    );
    if (!used) {
      continue;
    }
    usedVariables.push(variable.name);
    if (variable.required && !value.trim()) {
      missingRequired.push(variable.name);
    }
    for (const placeholder of placeholders) {
      bodyText = bodyText.replace(
        new RegExp(escapeAutomationScriptPublicAPIRegex(placeholder), "g"),
        escapeAutomationScriptPublicAPIJSONStringValue(value),
      );
    }
  }

  return { bodyText, missingRequired, usedVariables };
}
