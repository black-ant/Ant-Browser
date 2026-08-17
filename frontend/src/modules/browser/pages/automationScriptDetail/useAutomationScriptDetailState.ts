import { useEffect, useState } from "react";
import { toast } from "../../../../shared/components";
import { fetchBrowserProfiles, fetchGroups } from "../../api";
import { fetchAutomationScripts } from "../../automationScriptApi";
import type {
  AutomationScriptPublicAPIConfig,
  AutomationScriptRecord,
  AutomationScriptTargetConfig,
  AutomationScriptTargetSelector,
} from "../../automationScripts";
import type { BrowserGroupWithCount, BrowserProfile } from "../../types";

export type AutomationScriptDetailBusyAction =
  | "none"
  | "save"
  | "delete"
  | "refresh"
  | "export";

export function useAutomationScriptDetailState(scriptId: string) {
  const [draft, setDraft] = useState<AutomationScriptRecord | null>(null);
  const [profiles, setProfiles] = useState<BrowserProfile[]>([]);
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [runModalOpen, setRunModalOpen] = useState(false);
  const [exportModalOpen, setExportModalOpen] = useState(false);
  const [publicApiModalOpen, setPublicApiModalOpen] = useState(false);
  const [publicApiTestFocusTrigger, setPublicApiTestFocusTrigger] = useState(0);
  const [paramsHelpOpen, setParamsHelpOpen] = useState(false);
  const [showDualRuntimeRequests, setShowDualRuntimeRequests] = useState(false);
  const [busyAction, setBusyAction] =
    useState<AutomationScriptDetailBusyAction>("none");

  useEffect(() => {
    let disposed = false;

    setLoading(true);
    setNotFound(false);

    void fetchAutomationScripts()
      .then((items) => {
        if (disposed) {
          return;
        }

        const current = items.find((item) => item.id === scriptId) || null;
        setDraft(current);
        setDirty(false);
        setNotFound(!current);
      })
      .catch(() => {
        if (!disposed) {
          toast.error("脚本加载失败");
        }
      })
      .finally(() => {
        if (!disposed) {
          setLoading(false);
        }
      });

    return () => {
      disposed = true;
    };
  }, [scriptId]);

  useEffect(() => {
    setShowDualRuntimeRequests(false);
    setPublicApiModalOpen(false);
    setPublicApiTestFocusTrigger(0);
    setParamsHelpOpen(false);
  }, [scriptId]);

  useEffect(() => {
    let disposed = false;

    void Promise.allSettled([fetchBrowserProfiles(), fetchGroups()]).then(
      ([profilesResult, groupsResult]) => {
        if (disposed) {
          return;
        }
        if (profilesResult.status === "fulfilled") {
          setProfiles(profilesResult.value || []);
        }
        if (groupsResult.status === "fulfilled") {
          setGroups(groupsResult.value || []);
        }
      },
    );

    return () => {
      disposed = true;
    };
  }, []);

  useEffect(() => {
    if (!dirty) {
      return undefined;
    }

    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };

    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => {
      window.removeEventListener("beforeunload", handleBeforeUnload);
    };
  }, [dirty]);

  const updateDraft = (patch: Partial<AutomationScriptRecord>) => {
    setDraft((current) => {
      if (!current) {
        return current;
      }
      return {
        ...current,
        ...patch,
      };
    });
    setDirty(true);
  };

  const updateTargetConfig = (patch: Partial<AutomationScriptTargetConfig>) => {
    setDraft((current) => {
      if (!current) {
        return current;
      }
      return {
        ...current,
        targetConfig: {
          ...current.targetConfig,
          ...patch,
        },
      };
    });
    setDirty(true);
  };

  const updateTargetSelector = (
    key: "selector" | "templateSelector",
    patch: Partial<AutomationScriptTargetSelector>,
  ) => {
    setDraft((current) => {
      if (!current) {
        return current;
      }
      return {
        ...current,
        targetConfig: {
          ...current.targetConfig,
          [key]: {
            ...current.targetConfig[key],
            ...patch,
          },
        },
      };
    });
    setDirty(true);
  };

  const updatePublicAPI = (publicAPI: AutomationScriptPublicAPIConfig) => {
    setDraft((current) => {
      if (!current) {
        return current;
      }
      return {
        ...current,
        publicAPI,
      };
    });
    setDirty(true);
  };

  return {
    draft,
    setDraft,
    profiles,
    groups,
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
    updateTargetSelector,
    updatePublicAPI,
  };
}
