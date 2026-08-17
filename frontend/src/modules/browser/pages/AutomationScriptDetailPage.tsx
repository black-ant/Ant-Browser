import { ArrowLeft } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";
import { Button, toast } from "../../../shared/components";
import { deleteAutomationScript, exportAutomationScriptDirectory, exportAutomationScriptTemplate, exportAutomationScriptZip, refreshAutomationScript, saveAutomationScript } from "../automationScriptApi";
import {
  AutomationScriptExportModal,
  type AutomationScriptExportFormat,
} from "../components/AutomationScriptExportModal";
import { AutomationScriptPublicApiModal } from "../components/AutomationScriptPublicApiModal";
import { AutomationScriptRunModal } from "../components/AutomationScriptRunModal";
import {
  DUAL_INSTANCE_RUNTIME_SCRIPT_ID, buildAutomationScriptPublicAPIPath, canRefreshAutomationScriptSource,
  resolveAutomationScriptPublicAPIConfig,
  type AutomationScriptPublicAPIConfig, type AutomationScriptRecord,
} from "../automationScripts";
import { useLaunchContext } from "../hooks/useLaunchContext";
import { AutomationScriptDetailModals } from "./automationScriptDetail/AutomationScriptDetailModals";
import { AutomationScriptDetailPanels } from "./automationScriptDetail/AutomationScriptDetailPanels";
import {
  buildDualRuntimeRequestPreviews, buildOpenClawDualSiteCommand, buildPersistablePublicAPIConfig,
  hasSamePublicAPIConfig, preparePublicAPIConfigForCompare,
  validatePublicAPIConfig, validateTargetConfig,
} from "./automationScriptDetail/helpers";
import { getScriptParamsHelp } from "./automationScriptDetail/paramsHelp";
import { useAutomationScriptDetailState } from "./automationScriptDetail/useAutomationScriptDetailState";

export function AutomationScriptDetailPage() {
  const navigate = useNavigate();
  const { scriptId = "" } = useParams();
  const { launchBaseUrl, apiAuth } = useLaunchContext();
  const {
    draft,
    setDraft,
    profiles,
    loading,
    notFound,
    dirty,
    setDirty,
    runModalOpen,
    setRunModalOpen,
    exportModalOpen,
    setExportModalOpen,
    publicApiModalOpen,
    setPublicApiModalOpen,
    publicApiTestFocusTrigger,
    setPublicApiTestFocusTrigger,
    paramsHelpOpen,
    setParamsHelpOpen,
    showDualRuntimeRequests,
    setShowDualRuntimeRequests,
    busyAction,
    setBusyAction,
    updateDraft,
    updateTargetConfig,
    updatePublicAPI,
  } = useAutomationScriptDetailState(scriptId);

  const leavePage = () => {
    if (dirty && !window.confirm("当前脚本有未保存修改，确认离开吗？")) {
      return;
    }
    navigate("/browser/automation");
  };

  const validateDraftForSave = (script: AutomationScriptRecord): boolean => {
    if (!script.name.trim()) {
      toast.warning("脚本名称不能为空");
      return false;
    }
    if (!script.scriptText.trim()) {
      toast.warning(
        script.type === "launch-api"
          ? "固定接口模板不能为空"
          : "脚本内容不能为空",
      );
      return false;
    }

    const targetConfigError = validateTargetConfig(script.targetConfig);
    if (targetConfigError) {
      toast.warning(targetConfigError);
      return false;
    }

    const publicAPIError = validatePublicAPIConfig(script.publicAPI);
    if (publicAPIError) {
      toast.warning(publicAPIError);
      return false;
    }

    return true;
  };

  const persistDraft = async (
    script: AutomationScriptRecord,
    options?: { successMessage?: string; silentSuccess?: boolean },
  ): Promise<AutomationScriptRecord | null> => {
    const scriptToSave = {
      ...script,
      name: script.name.trim(),
      description: script.description.trim(),
      scriptText: script.scriptText,
      publicAPI: buildPersistablePublicAPIConfig(script),
      updatedAt: new Date().toISOString(),
    };
    if (!validateDraftForSave(scriptToSave)) {
      return null;
    }

    setBusyAction("save");
    try {
      const saved = await saveAutomationScript(scriptToSave);
      setDraft(saved);
      setDirty(false);
      if (!options?.silentSuccess) {
        toast.success(options?.successMessage || "脚本已保存");
      }
      return saved;
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本保存失败";
      toast.error(message);
      return null;
    } finally {
      setBusyAction("none");
    }
  };

  const handleSave = async () => {
    if (!draft) {
      return;
    }
    await persistDraft(draft, { successMessage: "脚本已保存" });
  };

  const handleOpenPublicApiManager = () => {
    setPublicApiTestFocusTrigger(0);
    setPublicApiModalOpen(true);
  };
  const handleOpenPublicApiTester = () => {
    setPublicApiTestFocusTrigger((current) => current + 1);
    setPublicApiModalOpen(true);
  };

  const handlePreparePublicApiInvoke = async (
    publicAPI: AutomationScriptPublicAPIConfig,
  ): Promise<boolean> => {
    if (!draft) {
      return false;
    }

    const currentPublicAPI = preparePublicAPIConfigForCompare(draft);
    const nextPublicAPI = preparePublicAPIConfigForCompare(draft, publicAPI);
    const nextDraft = {
      ...draft,
      publicAPI,
    };
    if (!dirty && hasSamePublicAPIConfig(currentPublicAPI, nextPublicAPI)) {
      return true;
    }

    const saved = await persistDraft(nextDraft, { silentSuccess: true });
    return Boolean(saved);
  };

  const handleOpenRunModal = () => {
    if (draft) setRunModalOpen(true);
  };

  const handleDelete = async () => {
    if (!draft) {
      return;
    }
    if (!window.confirm(`确认删除脚本「${draft.name || "未命名脚本"}」吗？`)) {
      return;
    }

    setBusyAction("delete");
    try {
      await deleteAutomationScript(draft.id);
      toast.success("脚本已删除");
      navigate("/browser/automation", { replace: true });
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本删除失败";
      toast.error(message);
    } finally {
      setBusyAction("none");
    }
  };

  const handleRefresh = async () => {
    if (!draft) {
      return;
    }
    if (!canRefreshAutomationScriptSource(draft.source)) {
      toast.warning("当前脚本来源不支持重新导入");
      return;
    }
    if (dirty) {
      toast.warning("请先保存当前修改，再重新导入");
      return;
    }
    if (!window.confirm("确认按来源重新导入当前脚本吗？这会覆盖当前脚本内容。")) {
      return;
    }

    setBusyAction("refresh");
    try {
      const refreshed = await refreshAutomationScript(draft.id);
      setDraft(refreshed);
      setDirty(false);
      toast.success(
        draft.source.type === "git" ? "脚本已重新拉取" : "脚本已重新导入",
      );
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : "脚本重新导入失败";
      toast.error(message);
    } finally {
      setBusyAction("none");
    }
  };

  const handleOpenExportModal = () => {
    if (draft) setExportModalOpen(true);
  };

  const handleExport = async (format: AutomationScriptExportFormat) => {
    if (!draft) {
      return;
    }
    if (dirty) {
      toast.warning("请先保存当前修改，再导出");
      return;
    }

    setBusyAction("export");
    try {
      const result =
        format === "json"
          ? await exportAutomationScriptTemplate(draft.id, draft)
          : format === "directory"
            ? await exportAutomationScriptDirectory(draft.id)
            : await exportAutomationScriptZip(draft.id);
      if (result.cancelled) {
        setExportModalOpen(false);
        return;
      }

      setExportModalOpen(false);
      switch (format) {
        case "directory":
          toast.success(
            result.fileCount > 1
              ? `目录已导出，包含 ${result.fileCount} 个文件`
              : result.message || "目录已导出",
          );
          break;
        case "zip":
          toast.success(
            result.fileCount > 1
              ? `ZIP 已导出，包含 ${result.fileCount} 个文件`
              : result.message || "ZIP 已导出",
          );
          break;
        default:
          toast.success(
            result.fileCount > 1
              ? `模板已导出，包含 ${result.fileCount} 个文件`
              : result.message || "模板已导出",
          );
          break;
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本导出失败";
      toast.error(message);
    } finally {
      setBusyAction("none");
    }
  };

  if (loading) {
    return <div className="animate-fade-in rounded-2xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-12 text-center text-sm text-[var(--color-text-muted)]">正在加载脚本...</div>;
  }
  if (notFound || !draft) {
    return (
      <div className="space-y-4 animate-fade-in">
        <Button variant="secondary" size="sm" onClick={() => navigate("/browser/automation")}>
          <ArrowLeft className="h-4 w-4" />
          返回列表
        </Button>
        <div className="rounded-2xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-12 text-center text-sm text-[var(--color-text-muted)]">
          脚本不存在或已被删除。
        </div>
      </div>
    );
  }

  const busy = busyAction !== "none";
  const canRefresh = canRefreshAutomationScriptSource(draft.source);
  const isDualInstanceRuntimeScript = draft.id === DUAL_INSTANCE_RUNTIME_SCRIPT_ID;
  const isLaunchApiScript = draft.type === "launch-api";
  const usesManualSelector =
    draft.targetConfig.mode === "manual" || draft.targetConfig.mode === "existing";
  const resolvedPublicAPI = resolveAutomationScriptPublicAPIConfig(draft);
  const publicAPIPath = buildAutomationScriptPublicAPIPath(resolvedPublicAPI.path);
  const publicAPIURL = `${launchBaseUrl}${publicAPIPath}`;
  const paramsHelp = getScriptParamsHelp(draft);
  const dualRuntimePreview = isDualInstanceRuntimeScript
    ? buildDualRuntimeRequestPreviews(draft.paramsText)
    : { requests: [], error: "" };
  const dualRuntimeCodes = dualRuntimePreview.requests.map((request) => request.code);
  const openClawDualSiteCommand = buildOpenClawDualSiteCommand(draft.id, dualRuntimeCodes);
  const handleCopyOpenClawCommand = async () => {
    try {
      await navigator.clipboard.writeText(openClawDualSiteCommand);
      toast.success("OpenClaw 指令已复制");
    } catch {
      toast.error("复制失败");
    }
  };

  return (
    <>
      <AutomationScriptDetailPanels
        draft={draft}
        dirty={dirty}
        busy={busy}
        busyAction={busyAction}
        canRefresh={canRefresh}
        isDualInstanceRuntimeScript={isDualInstanceRuntimeScript}
        isLaunchApiScript={isLaunchApiScript}
        usesManualSelector={usesManualSelector}
        resolvedPublicAPI={resolvedPublicAPI}
        publicAPIURL={publicAPIURL}
        paramsHelp={paramsHelp}
        launchBaseUrl={launchBaseUrl}
        apiAuthHeader={apiAuth.enabled ? apiAuth.header : ""}
        dualRuntimePreview={dualRuntimePreview}
        showDualRuntimeRequests={showDualRuntimeRequests}
        openClawDualSiteCommand={openClawDualSiteCommand}
        onLeavePage={leavePage}
        onUpdateDraft={updateDraft}
        onUpdateTargetConfig={updateTargetConfig}
        onOpenRunModal={handleOpenRunModal}
        onOpenPublicApiManager={handleOpenPublicApiManager}
        onOpenPublicApiTester={handleOpenPublicApiTester}
        onOpenExportModal={handleOpenExportModal}
        onSave={() => void handleSave()}
        onDelete={() => void handleDelete()}
        onRefresh={() => void handleRefresh()}
        onOpenExistingTargetConfig={() => {
          updateTargetConfig({ mode: "existing" });
        }}
        onToggleDualRuntimeRequests={() =>
          setShowDualRuntimeRequests((current) => !current)
        }
        onCopyOpenClawCommand={() => void handleCopyOpenClawCommand()}
        onOpenParamsHelp={() => setParamsHelpOpen(true)}
      />

      <AutomationScriptRunModal
        open={runModalOpen}
        script={draft}
        dirty={dirty}
        onClose={() => setRunModalOpen(false)}
      />
      <AutomationScriptExportModal
        open={exportModalOpen}
        busy={busyAction === "export"}
        onClose={() => setExportModalOpen(false)}
        onSubmit={(format) => void handleExport(format)}
      />
      <AutomationScriptPublicApiModal
        open={publicApiModalOpen}
        script={draft}
        busy={busy}
        launchBaseUrl={launchBaseUrl}
        apiAuthEnabled={apiAuth.enabled}
        apiAuthHeader={apiAuth.header}
        profiles={profiles}
        focusTestTrigger={publicApiTestFocusTrigger}
        onClose={() => setPublicApiModalOpen(false)}
        onChange={updatePublicAPI}
        onBeforeInvoke={handlePreparePublicApiInvoke}
      />
      <AutomationScriptDetailModals
        paramsHelp={paramsHelp}
        paramsHelpOpen={paramsHelpOpen}
        onCloseParamsHelp={() => setParamsHelpOpen(false)}
      />
    </>
  );
}
