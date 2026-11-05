import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from "react";
import { toast } from "../../../shared/components";
import { fetchBrowserProfiles, fetchGroups } from "../api";
import type { AutomationScriptRecord } from "../automationScripts";
import type { AutomationDemoSession } from "../demoSession";
import type { BrowserGroupWithCount, BrowserProfile } from "../types";
import {
  buildGroupOptions,
  buildProfileSuggestions,
} from "../pages/automationScriptDetail/helpers";
import type { DemoCreateDraft, DemoPreparationMode, SelectableProfile } from "./AutomationScriptRunModal.types";
import { DEFAULT_DEMO_CREATE_DRAFT } from "./AutomationScriptRunModal.types";
import {
  buildDemoSelectorText,
  filterSelectableProfiles,
  isCodeOnlySelectorForLaunchCode,
  isPlaceholderSelectorText,
  normalizeLaunchCode,
  resolveInitialSelectorText,
  resolvePreferredProfileId,
  resolveSelectorLaunchCode,
  sortTemplateProfiles,
} from "./AutomationScriptRunModal.helpers";

interface UseAutomationScriptRunProfilesOptions {
  open: boolean;
  script: AutomationScriptRecord | null;
  isManualTargetMode: boolean;
  usesStoredTargetConfig: boolean;
  selectorText: string;
  setSelectorText: (value: string) => void;
  setDemoSession: Dispatch<SetStateAction<AutomationDemoSession>>;
  reloadDemoSession: () => AutomationDemoSession;
}

export function useAutomationScriptRunProfiles({
  open,
  script,
  isManualTargetMode,
  usesStoredTargetConfig,
  selectorText,
  setSelectorText,
  setDemoSession,
  reloadDemoSession,
}: UseAutomationScriptRunProfilesOptions) {
  const [demoMode, setDemoMode] = useState<DemoPreparationMode>("select");
  const [availableProfiles, setAvailableProfiles] = useState<SelectableProfile[]>(
    [],
  );
  const [templateProfiles, setTemplateProfiles] = useState<BrowserProfile[]>([]);
  const [allProfiles, setAllProfiles] = useState<BrowserProfile[]>([]);
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([]);
  const [profilesLoading, setProfilesLoading] = useState(false);
  const [selectedProfileId, setSelectedProfileId] = useState("");
  const [createDraft, setCreateDraft] = useState<DemoCreateDraft>(
    DEFAULT_DEMO_CREATE_DRAFT,
  );
  const lastRefreshKeyRef = useRef("");
  const activeRefreshKeyRef = useRef("");
  const selectedProfile =
    availableProfiles.find((profile) => profile.profileId === selectedProfileId) ||
    null;
  const selectorDetachedFromSelectedProfile =
    demoMode === "select" &&
    !!selectedProfile &&
    !!selectorText.trim() &&
    !isPlaceholderSelectorText(selectorText) &&
    !isCodeOnlySelectorForLaunchCode(selectorText, selectedProfile.launchCode);
  const selectedLaunchCode = resolveSelectorLaunchCode(selectorText);
  const codeSuggestions = buildProfileSuggestions(
    allProfiles,
    (profile) => profile.launchCode,
    (profile) =>
      profile.profileName
        ? `${profile.launchCode || "未设 Code"} · ${profile.profileName}`
        : profile.profileId,
  );
  const profileIdSuggestions = buildProfileSuggestions(
    allProfiles,
    (profile) => profile.profileId,
    (profile) =>
      profile.launchCode
        ? `${profile.launchCode} · ${profile.profileName || profile.profileId}`
        : profile.profileName || profile.profileId,
  );
  const profileNameSuggestions = buildProfileSuggestions(
    allProfiles,
    (profile) => profile.profileName,
    (profile) =>
      profile.launchCode
        ? `${profile.launchCode} · ${profile.profileId}`
        : profile.profileId,
  );
  const groupOptions = [{ value: "", label: "不限制" }, ...buildGroupOptions(groups)];
  const syncDemoSessionFromProfile = (
    profile: SelectableProfile,
    actionLabel: string,
  ) => {
    setDemoSession((current) => ({
      ...current,
      profileId: profile.profileId,
      profileName: profile.profileName,
      launchCode: profile.launchCode,
      cdpUrl:
        profile.running && profile.debugReady && profile.debugPort > 0
          ? `http://127.0.0.1:${profile.debugPort}`
          : "",
      debugPort:
        profile.running && profile.debugReady && profile.debugPort > 0
          ? profile.debugPort
          : 0,
      lastAction: actionLabel,
    }));
  };

  const refreshSelectableProfiles = async (
    preferredProfileId = "",
    preferredLaunchCode = "",
    showError = false,
  ) => {
    setProfilesLoading(true);
    try {
      const allProfiles = await fetchBrowserProfiles();
      setAllProfiles(allProfiles);
      const profiles = filterSelectableProfiles(allProfiles);
      const nextSelectedProfileId =
        resolvePreferredProfileId(
          profiles,
          preferredProfileId,
          preferredLaunchCode,
        ) ||
        (selectedProfileId &&
        profiles.some((profile) => profile.profileId === selectedProfileId)
          ? selectedProfileId
          : isManualTargetMode
            ? ""
            : profiles[0]?.profileId || "");
      const nextSelectedProfile =
        profiles.find((profile) => profile.profileId === nextSelectedProfileId) ||
        null;

      setAvailableProfiles(profiles);
      setTemplateProfiles(sortTemplateProfiles(allProfiles));
      setSelectedProfileId(nextSelectedProfileId);
      if (demoMode === "select" && nextSelectedProfile) {
        const keepManualSelector =
          !!selectorText.trim() &&
          !isPlaceholderSelectorText(selectorText) &&
          !isCodeOnlySelectorForLaunchCode(
            selectorText,
            nextSelectedProfile.launchCode,
          );
        const nextSelectorText = buildDemoSelectorText(
          nextSelectedProfile.launchCode,
        );
        if (
          !keepManualSelector &&
          resolveSelectorLaunchCode(selectorText) !==
          nextSelectedProfile.launchCode
        ) {
          setSelectorText(nextSelectorText);
        }
        if (!keepManualSelector) {
          syncDemoSessionFromProfile(nextSelectedProfile, "选择实例");
        }
      }
      setCreateDraft((current) => {
        if (
          current.templateProfileId &&
          allProfiles.some((profile) => profile.profileId === current.templateProfileId)
        ) {
          return current;
        }
        return {
          ...current,
          templateProfileId: allProfiles[0]?.profileId || "",
        };
      });
      if (!profiles.length && !isManualTargetMode) {
        setDemoMode("create");
      }
    } catch (error: unknown) {
      if (showError) {
        const message =
          error instanceof Error ? error.message : "实例列表刷新失败";
        toast.error(message);
      }
    } finally {
      setProfilesLoading(false);
    }
  };

  useEffect(() => {
    if (!open) {
      return;
    }
    let disposed = false;
    void fetchGroups().then(
      (items) => {
        if (!disposed) {
          setGroups(items || []);
        }
      },
      () => {
        if (!disposed) {
          setGroups([]);
        }
      },
    );
    return () => {
      disposed = true;
    };
  }, [open]);

  useEffect(() => {
    if (!open || !script) {
      setAvailableProfiles([]);
      setSelectedProfileId("");
      lastRefreshKeyRef.current = "";
      activeRefreshKeyRef.current = "";
      return;
    }

    const nextDemoSession = reloadDemoSession();
    const nextSelectorText = resolveInitialSelectorText(script, nextDemoSession);
    const preferredProfileId =
      script.targetConfig.selector.profileId || nextDemoSession.profileId;
    const preferredLaunchCode =
      resolveSelectorLaunchCode(nextSelectorText) || nextDemoSession.launchCode;
    const refreshKey = [
      script.id,
      usesStoredTargetConfig ? "stored" : "runtime",
      preferredProfileId,
      preferredLaunchCode,
    ].join("|");

    if (
      lastRefreshKeyRef.current === refreshKey ||
      activeRefreshKeyRef.current === refreshKey
    ) {
      return;
    }

    activeRefreshKeyRef.current = refreshKey;
    void refreshSelectableProfiles(preferredProfileId, preferredLaunchCode, false)
      .then(() => {
        lastRefreshKeyRef.current = refreshKey;
      })
      .finally(() => {
        if (activeRefreshKeyRef.current === refreshKey) {
          activeRefreshKeyRef.current = "";
        }
      });
  }, [open, script?.id, usesStoredTargetConfig]);

  const handleSelectedProfileChange = (profileId: string) => {
    setSelectedProfileId(profileId);
    const profile =
      availableProfiles.find((item) => item.profileId === profileId) || null;
    if (!profile) {
      return;
    }

    setSelectorText(buildDemoSelectorText(profile.launchCode));
    syncDemoSessionFromProfile(profile, "选择实例");
  };

  const handleLaunchCodeChange = (code: string) => {
    const launchCode = normalizeLaunchCode(code);
    setSelectorText(launchCode ? buildDemoSelectorText(launchCode) : "");
    const profile =
      availableProfiles.find((item) => item.launchCode === launchCode) || null;
    setSelectedProfileId(profile?.profileId || "");
    if (profile) {
      syncDemoSessionFromProfile(profile, "填写实例 Code");
    }
  };

  const handleSelectorTextChange = (value: string) => {
    setSelectorText(value);
    const launchCode = resolveSelectorLaunchCode(value);
    const profile =
      availableProfiles.find((item) => item.launchCode === launchCode) || null;
    setSelectedProfileId(profile?.profileId || "");
    if (profile) {
      syncDemoSessionFromProfile(profile, "填写 selector");
    }
  };

  const handleRestoreSelectedProfileSelector = () => {
    if (!selectedProfile) {
      return;
    }

    setSelectorText(buildDemoSelectorText(selectedProfile.launchCode));
    syncDemoSessionFromProfile(selectedProfile, "选择实例");
  };


  return {
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
  };
}
