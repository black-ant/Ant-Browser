import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "../../../shared/components";
import { AutomationScriptHistoryModal } from "../components/AutomationScriptHistoryModal";
import { AutomationScriptPublicApiModal } from "../components/AutomationScriptPublicApiModal";
import { AutomationScriptRunModal } from "../components/AutomationScriptRunModal";

import {
  exportAutomationScriptsBatchZip,
  importAutomationScriptFromGit,
  importAutomationScriptFromLocalDirectory,
  importAutomationScriptFromLocalFile,
  saveAutomationScript,
} from "../automationScriptApi";
import {
  createAutomationScriptDraft,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptRecord,
  type AutomationScriptType,
} from "../automationScripts";
import { useLaunchContext } from "../hooks/useLaunchContext";
import { AutomationCardsSection } from "./AutomationCardsSection";
import { AutomationPageHeader } from "./AutomationPageHeader";
import { useAutomationPageData } from "./useAutomationPageData";
import { CreateAutomationScriptModal, ImportAutomationScriptModal } from "./AutomationPageModals";
import {
  DUAL_INSTANCE_SCRIPT_ID,
  buildAutomationCardPresentation,
  buildAutomationRequestPayloadText,
  buildAutomationRunCurlDemo,
  buildDualInstanceFallbackPresentation,
  buildPersistablePublicAPIConfig,
  mergeImportedScripts,
  resolveDualLaunchCodes,
  type AutomationCardPresentation,
  type ImportMode,
} from "./AutomationPage.helpers";
export function AutomationPage() {
  const navigate = useNavigate();
  const { launchBaseUrl, apiAuth } = useLaunchContext();
  const { scripts, setScripts, profiles, loading, refreshing, handleRefresh } = useAutomationPageData();
  const [historyOpen, setHistoryOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [runModalOpen, setRunModalOpen] = useState(false);
  const [activeRunScript, setActiveRunScript] =
    useState<AutomationScriptRecord | null>(null);
  const [publicApiModalOpen, setPublicApiModalOpen] = useState(false);
  const [publicApiTestFocusTrigger, setPublicApiTestFocusTrigger] = useState(0);
  const [activePublicApiScript, setActivePublicApiScript] =
    useState<AutomationScriptRecord | null>(null);
  const [publicApiSaving, setPublicApiSaving] = useState(false);
  const [createType, setCreateType] =
    useState<AutomationScriptType>("playwright-cdp");
  const [createName, setCreateName] = useState("");
  const [importMode, setImportMode] = useState<ImportMode>("local");
  const [gitURL, setGitURL] = useState("");
  const [gitRef, setGitRef] = useState("");
  const [gitScriptPath, setGitScriptPath] = useState("");
  const [busyAction, setBusyAction] = useState<"none" | "create" | "import" | "export">(
    "none",
  );
  const [selectedScriptIds, setSelectedScriptIds] = useState<string[]>([]);


  const openScript = (scriptId: string) => {
    navigate(`/browser/automation/${scriptId}`);
  };

  const handleToggleScriptSelection = (scriptId: string, selected: boolean) => {
    setSelectedScriptIds((current) => {
      if (selected) {
        return current.includes(scriptId) ? current : [...current, scriptId];
      }
      return current.filter((item) => item !== scriptId);
    });
  };

  const handleExportSelectedScripts = async () => {
    const exportScriptIds = selectedScriptIds.filter((scriptId) =>
      scripts.some((script) => script.id === scriptId),
    );
    if (exportScriptIds.length === 0) {
      toast.warning("请先勾选要导出的脚本");
      return;
    }

    setBusyAction("export");
    try {
      const result = await exportAutomationScriptsBatchZip(exportScriptIds);
      if (!result.cancelled) {
        toast.success(`已导出 ${exportScriptIds.length} 个脚本`);
        setSelectedScriptIds([]);
      } else {
        toast.warning("已取消导出");
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本导出失败";
      toast.error(message);
    } finally {
      setBusyAction("none");
    }
  };

  const handleOpenRunModal = (script: AutomationScriptRecord) => {
    setActiveRunScript(script);
    setRunModalOpen(true);
  };

  const handleOpenPublicApiModal = (
    script: AutomationScriptRecord,
    options?: { focusTest?: boolean },
  ) => {
    setActivePublicApiScript({ ...script });
    if (options?.focusTest) {
      setPublicApiTestFocusTrigger((current) => current + 1);
    } else {
      setPublicApiTestFocusTrigger(0);
    }
    setPublicApiModalOpen(true);
  };

  const updateActivePublicApiConfig = (
    publicAPI: AutomationScriptPublicAPIConfig,
  ) => {
    setActivePublicApiScript((current) =>
      current
        ? {
            ...current,
            publicAPI,
          }
        : current,
    );
  };

  const persistPublicApiScript = async (
    script: AutomationScriptRecord,
    options?: { silentSuccess?: boolean },
  ): Promise<AutomationScriptRecord | null> => {
    setPublicApiSaving(true);
    try {
      const saved = await saveAutomationScript({
        ...script,
        name: script.name.trim(),
        description: script.description.trim(),
        publicAPI: buildPersistablePublicAPIConfig(script),
        updatedAt: new Date().toISOString(),
      });
      setScripts((current) =>
        current.map((item) => (item.id === saved.id ? saved : item)),
      );
      setActivePublicApiScript(saved);
      if (!options?.silentSuccess) {
        toast.success("接口配置已保存");
      }
      return saved;
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : "接口配置保存失败";
      toast.error(message);
      return null;
    } finally {
      setPublicApiSaving(false);
    }
  };

  const handlePreparePublicApiInvoke = async (
    publicAPI: AutomationScriptPublicAPIConfig,
  ): Promise<boolean> => {
    if (!activePublicApiScript) {
      return false;
    }

    const nextScript = {
      ...activePublicApiScript,
      publicAPI,
    };
    setActivePublicApiScript(nextScript);
    const saved = await persistPublicApiScript(nextScript, {
      silentSuccess: true,
    });
    return Boolean(saved);
  };


  const resetCreateModal = () => {
    setCreateType("playwright-cdp");
    setCreateName("");
  };

  const closeCreateModal = () => {
    if (busyAction !== "none") {
      return;
    }
    setCreateOpen(false);
    resetCreateModal();
  };

  const resetImportModal = () => {
    setImportMode("local");
    setGitURL("");
    setGitRef("");
    setGitScriptPath("");
  };

  const closeImportModal = () => {
    if (busyAction !== "none") {
      return;
    }
    setImportOpen(false);
    resetImportModal();
  };

  const handleCreate = async () => {
    setBusyAction("create");
    try {
      const draft = createAutomationScriptDraft(createType);
      if (createName.trim()) {
        draft.name = createName.trim();
      }

      const saved = await saveAutomationScript(draft);
      setScripts((current) => [
        saved,
        ...current.filter((item) => item.id !== saved.id),
      ]);
      setCreateOpen(false);
      resetCreateModal();
      toast.success("脚本已创建");
      openScript(saved.id);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本创建失败";
      toast.error(message);
    } finally {
      setBusyAction("none");
    }
  };

  const finishImport = (imported: AutomationScriptRecord[], failedCount = 0) => {
    if (imported.length === 0) {
      throw new Error("未导入任何脚本");
    }

    const importTimestamp = new Date().toISOString();
    const nextImported = imported.map((script) => ({
      ...script,
      source: {
        ...script.source,
        importedAt: importTimestamp,
      },
      updatedAt: importTimestamp,
    }));

    setScripts((current) => mergeImportedScripts(current, nextImported));
    setImportOpen(false);
    resetImportModal();
    if (nextImported.length === 1 && failedCount === 0) {
      toast.success("脚本已导入");
      openScript(nextImported[0].id);
      return;
    }

    toast.success(`已导入 ${nextImported.length} 个脚本`);
    if (failedCount > 0) {
      toast.warning(`${failedCount} 个脚本包导入失败`);
    }
  };

  const handleLocalImport = async (kind: "file" | "directory") => {
    setBusyAction("import");
    try {
      const imported = kind === "directory"
        ? await importAutomationScriptFromLocalDirectory()
        : await importAutomationScriptFromLocalFile();
      finishImport([imported]);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本导入失败";
      toast.error(message);
    } finally {
      setBusyAction("none");
    }
  };

  const handleImport = async () => {
    setBusyAction("import");
    try {
      switch (importMode) {
        case "git":
          finishImport([
            await importAutomationScriptFromGit(gitURL, gitRef, gitScriptPath),
          ]);
          break;
        default:
          throw new Error("请选择要导入的文件或文件夹");
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本导入失败";
      toast.error(message);
    } finally {
      setBusyAction("none");
    }
  };

  const dualLaunchCodes = resolveDualLaunchCodes(profiles);
  const dualInstanceRunPayload = {
    scriptId: DUAL_INSTANCE_SCRIPT_ID,
    params: {
      browsers: [
        {
          code: dualLaunchCodes.primaryCode,
          skipDefaultStartUrls: true,
        },
        {
          code: dualLaunchCodes.secondaryCode,
          skipDefaultStartUrls: true,
        },
      ],
      timeoutMs: 45000,
    },
  };
  const dualInstanceRunPayloadText = buildAutomationRequestPayloadText(
    dualInstanceRunPayload,
  );
  const dualInstanceRunCurlDemo = buildAutomationRunCurlDemo({
    launchBaseUrl,
    apiAuthEnabled: apiAuth.enabled,
    apiAuthHeader: apiAuth.header,
    payload: dualInstanceRunPayload,
  });
  const hasDualInstanceBaseline = scripts.some(
    (item) => item.id === DUAL_INSTANCE_SCRIPT_ID,
  );
  const orderedScripts = [...scripts].sort((left, right) => {
    if (left.id === DUAL_INSTANCE_SCRIPT_ID) {
      return -1;
    }
    if (right.id === DUAL_INSTANCE_SCRIPT_ID) {
      return 1;
    }
    return 0;
  });
  const scriptCards = orderedScripts.map((script) =>
    buildAutomationCardPresentation({
      script,
      profiles,
      launchBaseUrl,
      apiAuthEnabled: apiAuth.enabled,
      apiAuthHeader: apiAuth.header,
      dualLaunchCodes,
      dualInstanceRunPayload,
      dualInstanceRunPayloadText,
      dualInstanceRunCurlDemo,
    }),
  );
  const cards: AutomationCardPresentation[] = hasDualInstanceBaseline
    ? scriptCards
    : [
        buildDualInstanceFallbackPresentation({
          dualLaunchCodes,
          dualInstanceRunPayloadText,
          dualInstanceRunCurlDemo,
        }),
        ...scriptCards,
      ];
  const modalBusyAction =
    busyAction === "export" ? "none" : busyAction;

  return (
    <div className="space-y-4 animate-fade-in">
      <AutomationPageHeader
        refreshing={refreshing}
        exporting={busyAction === "export"}
        selectedCount={selectedScriptIds.length}
        onRefresh={() => void handleRefresh()}
        onCreate={() => setCreateOpen(true)}
        onExportSelected={() => void handleExportSelectedScripts()}
        onImport={() => setImportOpen(true)}
        onOpenHistory={() => setHistoryOpen(true)}
      />

      <AutomationCardsSection
        loading={loading}
        cards={cards}
        scripts={scripts}
        selectedScriptIds={selectedScriptIds}
        onCreate={() => setCreateOpen(true)}
        onImport={() => setImportOpen(true)}
        onToggleScriptSelection={handleToggleScriptSelection}
        onOpenScript={openScript}
        onRunAutomationScript={handleOpenRunModal}
        onOpenPublicApi={handleOpenPublicApiModal}
      />

      <CreateAutomationScriptModal
        open={createOpen}
        busyAction={modalBusyAction}
        createName={createName}
        createType={createType}
        onClose={closeCreateModal}
        onCreate={handleCreate}
        onCreateNameChange={setCreateName}
        onCreateTypeChange={setCreateType}
      />

      <ImportAutomationScriptModal
        open={importOpen}
        busyAction={modalBusyAction}
        importMode={importMode}
        gitURL={gitURL}
        gitRef={gitRef}
        gitScriptPath={gitScriptPath}
        onClose={closeImportModal}
        onImport={handleImport}
        onImportLocalFile={() => handleLocalImport("file")}
        onImportLocalDirectory={() => handleLocalImport("directory")}
        onImportModeChange={setImportMode}
        onGitURLChange={setGitURL}
        onGitRefChange={setGitRef}
        onGitScriptPathChange={setGitScriptPath}
      />

      <AutomationScriptRunModal
        open={runModalOpen}
        script={activeRunScript}
        dirty={false}
        onClose={() => {
          setRunModalOpen(false);
          setActiveRunScript(null);
        }}
      />
      {activePublicApiScript ? (
        <AutomationScriptPublicApiModal
          open={publicApiModalOpen}
          script={activePublicApiScript}
          busy={publicApiSaving}
          launchBaseUrl={launchBaseUrl}
          apiAuthEnabled={apiAuth.enabled}
          apiAuthHeader={apiAuth.header}
          profiles={profiles}
          focusTestTrigger={publicApiTestFocusTrigger}
          onClose={() => {
            if (publicApiSaving) {
              return;
            }
            setPublicApiModalOpen(false);
            setActivePublicApiScript(null);
          }}
          onChange={updateActivePublicApiConfig}
          onBeforeInvoke={handlePreparePublicApiInvoke}
        />
      ) : null}
      <AutomationScriptHistoryModal
        open={historyOpen}
        onClose={() => setHistoryOpen(false)}
      />
    </div>
  );
}
