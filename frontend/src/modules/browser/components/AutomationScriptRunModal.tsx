import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "../../../shared/components";
import { openCorePath } from "../api";
import { runAutomationScript } from "../automationScriptApi";
import {
  DUAL_INSTANCE_RUNTIME_SCRIPT_ID,
  applyAutomationScriptPublicAPIVariables,
  createAutomationScriptTargetSelector,
  normalizeAutomationScriptTargetSelector,
  resolveAutomationScriptPublicAPIConfig,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptRunRecord,
  type AutomationScriptTargetSelector,
} from "../automationScripts";
import { useAutomationDemoSession } from "../hooks/useAutomationDemoSession";
import { useAutomationScriptRunProfiles } from "./useAutomationScriptRunProfiles";
import type {
  AutomationScriptRunModalProps,
  RunVariableInputs,
} from "./AutomationScriptRunModal.types";
import {
  buildDemoSelectorText,
  buildParamsTextFromPublicAPIRequest,
  buildPublicAPIVariableInputs,
  buildSelectableProfileOptions,
  buildTemplateProfileOptions,
  isPlaceholderSelectorText,
  resolveInitialSelectorText,
  resolveRunnableSelectorText,
  resolveSelectorLaunchCode,
  validateJsonObjectText,
} from "./AutomationScriptRunModal.helpers";
import { AutomationScriptRunModalView } from "./AutomationScriptRunModalView";

export function AutomationScriptRunModal({
  open,
  script,
  dirty = false,
  onClose,
}: AutomationScriptRunModalProps) {
  const navigate = useNavigate();
  const [selectorText, setSelectorText] = useState("");
  const [paramsText, setParamsText] = useState("");
  const [variableInputs, setVariableInputs] = useState<RunVariableInputs>({});
  const [running, setRunning] = useState(false);
  const demoBusy = false;
  const [lastRun, setLastRun] = useState<AutomationScriptRunRecord | null>(
    null,
  );
  const [rotateSelector, setRotateSelector] =
    useState<AutomationScriptTargetSelector>(() =>
      createAutomationScriptTargetSelector(),
    );
  const {
    demoSession,
    setDemoSession,
    reloadDemoSession,
  } = useAutomationDemoSession({ enabled: open });
  const isDualInstanceRuntimeScript =
    script?.id === DUAL_INSTANCE_RUNTIME_SCRIPT_ID;
  const isManualTargetMode =
    !!script &&
    (script.targetConfig.mode === "manual" || script.targetConfig.mode === "existing");
  const usesStoredTargetConfig =
    !!script && !isManualTargetMode;
  const showsSelectorInput =
    !!script && !usesStoredTargetConfig && !isDualInstanceRuntimeScript;
  const publicAPIConfig = useMemo(
    () => (script ? resolveAutomationScriptPublicAPIConfig(script) : null),
    [script],
  );
  const publicAPIVariables = publicAPIConfig?.variables || [];
  const usedPublicAPIVariableNames = useMemo(() => {
    if (!publicAPIConfig) {
      return new Set<string>();
    }
    return new Set(
      applyAutomationScriptPublicAPIVariables(
        publicAPIConfig.requestBodyText,
        publicAPIConfig.variables,
        variableInputs,
      ).usedVariables,
    );
  }, [publicAPIConfig, variableInputs]);
  const hasPublicAPIVariables = publicAPIVariables.length > 0;
  const hasUnusedPublicAPIVariables =
    hasPublicAPIVariables &&
    publicAPIVariables.some(
      (variable) => !usedPublicAPIVariableNames.has(variable.name),
    );
  const {
    demoMode,
    setDemoMode,
    availableProfiles,
    templateProfiles,
    profilesLoading,
    selectedProfileId,
    setSelectedProfileId,
    createDraft,
    setCreateDraft,
    selectedProfile,
    selectorDetachedFromSelectedProfile,
    selectedLaunchCode,
    codeSuggestions,
    profileIdSuggestions,
    profileNameSuggestions,
    groupOptions,
    syncDemoSessionFromProfile,
    handleSelectedProfileChange,
    handleLaunchCodeChange,
    handleSelectorTextChange,
    handleRestoreSelectedProfileSelector,
  } = useAutomationScriptRunProfiles({
    open,
    script,
    isManualTargetMode,
    usesStoredTargetConfig,
    selectorText,
    setSelectorText,
    setDemoSession,
    reloadDemoSession,
  });

  const syncParamsFromPublicAPIVariables = (
    config: AutomationScriptPublicAPIConfig,
    inputs: RunVariableInputs,
    fallbackParamsText: string,
  ) => {
    const resolved = buildParamsTextFromPublicAPIRequest(
      config,
      inputs,
      fallbackParamsText,
    );
    setParamsText(resolved.paramsText);
    return resolved;
  };
  const updateVariableInput = (name: string, value: string) => {
    setVariableInputs((current) => {
      const nextInputs = {
        ...current,
        [name]: value,
      };
      if (publicAPIConfig) {
        syncParamsFromPublicAPIVariables(
          publicAPIConfig,
          nextInputs,
          script?.paramsText || paramsText,
        );
      }
      return nextInputs;
    });
  };
  const updateParamsText = (nextParamsText: string) => {
    setParamsText(nextParamsText);
  };
  const updateRotateSelector = (
    patch: Partial<AutomationScriptTargetSelector>,
  ) => {
    setRotateSelector((current) =>
      normalizeAutomationScriptTargetSelector({
        ...current,
        ...patch,
      }),
    );
  };
  const resolveParamsTextForRun = (): string => {
    if (!publicAPIConfig || !hasPublicAPIVariables) {
      return paramsText;
    }
    return syncParamsFromPublicAPIVariables(
      publicAPIConfig,
      variableInputs,
      script?.paramsText || paramsText,
    ).paramsText;
  };
  const paramsLabel = isDualInstanceRuntimeScript ? "启动配置" : "运行参数";
  const paramsFieldLabel = isDualInstanceRuntimeScript
    ? "浏览器列表 / 启动配置 JSON"
    : "运行参数 JSON";
  const paramsPlaceholder = isDualInstanceRuntimeScript
    ? `{
  "browsers": [
    { "code": "BUYER_001", "skipDefaultStartUrls": true },
    { "code": "BUYER_002", "skipDefaultStartUrls": true }
  ],
  "timeoutMs": 45000
}`
    : '{"startUrls":["https://example.com"]}';

  useEffect(() => {
    if (!open || !script) {
      return;
    }

    const nextDemoSession = reloadDemoSession();
    const nextSelectorText = resolveInitialSelectorText(script, nextDemoSession);
    setSelectorText(nextSelectorText);
    const nextInputs = publicAPIConfig
      ? buildPublicAPIVariableInputs(publicAPIConfig)
      : {};
    setVariableInputs(nextInputs);
    if (publicAPIConfig && publicAPIConfig.variables.length > 0) {
      syncParamsFromPublicAPIVariables(
        publicAPIConfig,
        nextInputs,
        script.paramsText || "",
      );
    } else {
      updateParamsText(script.paramsText || "");
    }
    setLastRun(null);
    setCreateDraft({
      profileName: script.targetConfig.createNameTemplate || "",
      templateProfileId: script.targetConfig.templateSelector.profileId || "",
    });
    setSelectedProfileId(script.targetConfig.selector.profileId || "");
    setRotateSelector(
      normalizeAutomationScriptTargetSelector(script.targetConfig.selector),
    );
    setDemoMode(
      nextDemoSession.launchCode ||
        resolveSelectorLaunchCode(nextSelectorText)
        ? "select"
        : "create",
    );
  }, [open, script, publicAPIConfig]);

  const handleClose = () => {
    if (running || demoBusy) {
      return;
    }
    onClose();
  };

  const buildRunTargetInput = (): Record<string, unknown> => {
    if (!script || isManualTargetMode) {
      return {};
    }
    if (script.targetConfig.mode === "create") {
      return {
        templateSelector: createDraft.templateProfileId
          ? { profileId: createDraft.templateProfileId }
          : {},
        createNameTemplate: createDraft.profileName.trim(),
      };
    }
    if (script.targetConfig.mode === "rotate") {
      return rotateSelector as unknown as Record<string, unknown>;
    }
    return {};
  };

  const validateRunTargetInput = (): string => {
    if (!script || isManualTargetMode) {
      return "";
    }
    if (script.targetConfig.mode === "create") {
      if (!createDraft.templateProfileId) {
        return "先选择一个模板实例";
      }
      if (!createDraft.profileName.trim()) {
        return "先输入新实例名称";
      }
    }
    if (script.targetConfig.mode === "rotate") {
      const selector = normalizeAutomationScriptTargetSelector(rotateSelector);
      if (
        !selector.code &&
        !selector.profileId &&
        !selector.profileName &&
        !selector.groupId &&
        selector.keywords.length === 0 &&
        selector.tags.length === 0
      ) {
        return "先填写至少一个轮询条件";
      }
    }
    return "";
  };

  const executeRun = async (nextSelectorText: string, nextParamsText: string) => {
    if (!script) {
      return;
    }

    const runnableSelectorText = usesStoredTargetConfig ? "" : nextSelectorText;
    const launchCode =
      script.type === "playwright-cdp" && !usesStoredTargetConfig
        ? resolveSelectorLaunchCode(runnableSelectorText)
        : "";

    setRunning(true);
    try {
      const run = await runAutomationScript({
        scriptId: script.id,
        selectorText: runnableSelectorText,
        targetInput: buildRunTargetInput(),
        paramsText: nextParamsText,
        useScriptSelector: usesStoredTargetConfig,
        useScriptParams: false,
        launchCode,
        startByCodeBeforeRun:
          script.type === "playwright-cdp" &&
          !usesStoredTargetConfig &&
          !!launchCode,
      });
      setLastRun(run);
      if (run.status === "success") {
        toast.success(run.summary || "脚本执行完成");
      } else {
        toast.error(run.error || run.summary || "脚本执行失败");
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本执行失败";
      toast.error(message);
    } finally {
      setRunning(false);
    }
  };

  const handleRun = async () => {
    if (!script) {
      return;
    }

    let nextSelectorText = usesStoredTargetConfig
      ? ""
      : resolveRunnableSelectorText(
          script,
          selectorText,
          demoSession,
        );
    const selectorError = usesStoredTargetConfig
      ? ""
      : validateJsonObjectText(
          nextSelectorText,
          "目标选择器",
          script.type === "launch-api" &&
            !usesStoredTargetConfig &&
            !isDualInstanceRuntimeScript,
        );
    if (selectorError) {
      toast.warning(selectorError);
      return;
    }

    const nextParamsText = resolveParamsTextForRun();
    const paramsError = validateJsonObjectText(nextParamsText, paramsLabel, false);
    if (paramsError) {
      toast.warning(paramsError);
      return;
    }

    const targetInputError = validateRunTargetInput();
    if (targetInputError) {
      toast.warning(targetInputError);
      return;
    }

    if (
      script.type === "playwright-cdp" &&
      !usesStoredTargetConfig &&
      isPlaceholderSelectorText(nextSelectorText)
    ) {
      if (demoMode === "select" && selectedProfile) {
        nextSelectorText = buildDemoSelectorText(selectedProfile.launchCode);
        setSelectorText(nextSelectorText);
        syncDemoSessionFromProfile(selectedProfile, "选择实例");
        toast.success("已自动回填所选实例 selector");
      } else {
        toast.warning(
          demoMode === "create"
            ? "先创建一个实例，或填入可用 code"
            : "先选择实例，或填入可用 Code",
        );
        return;
      }
    }

    if (nextSelectorText !== selectorText) {
      setSelectorText(nextSelectorText);
    }
    if (
      script.type === "playwright-cdp" &&
      !usesStoredTargetConfig &&
      demoMode === "select" &&
      selectedProfile &&
      !selectorDetachedFromSelectedProfile
    ) {
      syncDemoSessionFromProfile(selectedProfile, "选择实例");
    }

    await executeRun(nextSelectorText, nextParamsText);
  };

  const handlePrimaryAction = async () => {
    if (!script) {
      return;
    }
    await handleRun();
  };

  const handleOpenScriptDetail = () => {
    if (!script || running || demoBusy) {
      return;
    }
    onClose();
    navigate(`/browser/automation/${script.id}`);
  };

  const handleOpenOutputPath = async (path: string) => {
    try {
      await openCorePath(path);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "打开目录失败";
      toast.error(message);
    }
  };

  if (!script) {
    return null;
  }

  const launchApiExecutable = script.status !== "disabled";
  const showDemoProfilePicker =
    script.type === "playwright-cdp" && !usesStoredTargetConfig;
  const selectableProfileOptions = buildSelectableProfileOptions(availableProfiles);
  const templateProfileOptions = buildTemplateProfileOptions(templateProfiles);
  const viewProps = {
    open,
    dirty,
    script,
    running,
    demoBusy,
    launchApiExecutable,
    showDemoProfilePicker,
    isManualTargetMode,
    usesStoredTargetConfig,
    isDualInstanceRuntimeScript,
    selectorDetachedFromSelectedProfile,
    showsSelectorInput,
    hasPublicAPIVariables,
    hasUnusedPublicAPIVariables,
    profilesLoading,
    selectedProfileId,
    selectedProfile,
    selectedLaunchCode,
    selectorText,
    paramsText,
    paramsFieldLabel,
    paramsPlaceholder,
    demoMode,
    createDraft,
    rotateSelector,
    variableInputs,
    publicAPIVariables,
    selectableProfileOptions,
    templateProfileOptions,
    codeSuggestions,
    profileIdSuggestions,
    profileNameSuggestions,
    groupOptions,
    lastRun,
    handleClose,
    handleOpenScriptDetail,
    handlePrimaryAction,
    handleSelectedProfileChange,
    handleLaunchCodeChange,
    handleRestoreSelectedProfileSelector,
    handleSelectorTextChange,
    handleOpenOutputPath,
    setCreateDraft,
    updateVariableInput,
    updateParamsText,
    updateRotateSelector,
  };

  return <AutomationScriptRunModalView {...viewProps} />;
}
