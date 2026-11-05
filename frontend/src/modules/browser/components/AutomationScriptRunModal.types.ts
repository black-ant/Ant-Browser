import type { AutomationScriptRecord } from "../automationScripts";
import type { BrowserProfile } from "../types";

export type DemoPreparationMode = "select" | "create";

export type SelectableProfile = BrowserProfile & {
  launchCode: string;
};

export interface DemoCreateDraft {
  profileName: string;
  templateProfileId: string;
}

export interface ResultOutputEntry {
  key: string;
  label: string;
  path: string;
}

export interface AutomationScriptRunModalProps {
  open: boolean;
  script: AutomationScriptRecord | null;
  dirty?: boolean;
  onClose: () => void;
}

export type RunVariableInputs = Record<string, string>;

export const DEFAULT_DEMO_CREATE_DRAFT: DemoCreateDraft = {
  profileName: "",
  templateProfileId: "",
};
