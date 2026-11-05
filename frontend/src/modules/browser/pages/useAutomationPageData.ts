import { useEffect, useState } from "react";
import { toast } from "../../../shared/components";
import { fetchBrowserProfiles } from "../api";
import { fetchAutomationScripts } from "../automationScriptApi";
import type { AutomationScriptRecord } from "../automationScripts";
import type { BrowserProfile } from "../types";

export function useAutomationPageData() {
  const [scripts, setScripts] = useState<AutomationScriptRecord[]>([]);
  const [profiles, setProfiles] = useState<BrowserProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  useEffect(() => {
    let disposed = false;

    void fetchAutomationScripts()
      .then((items) => {
        if (!disposed) {
          setScripts(items);
        }
      })
      .catch(() => {
        toast.error("脚本列表加载失败");
      })
      .finally(() => {
        if (!disposed) {
          setLoading(false);
        }
      });

    return () => {
      disposed = true;
    };
  }, []);

  useEffect(() => {
    let disposed = false;

    void fetchBrowserProfiles()
      .then((items) => {
        if (!disposed) {
          setProfiles(items || []);
        }
      })
      .catch(() => {
        if (!disposed) {
          setProfiles([]);
        }
      });

    return () => {
      disposed = true;
    };
  }, []);

  const handleRefresh = async () => {
    if (refreshing) {
      return;
    }

    setRefreshing(true);
    try {
      const [scriptsResult, profilesResult] = await Promise.allSettled([
        fetchAutomationScripts(),
        fetchBrowserProfiles(),
      ]);
      if (scriptsResult.status === "fulfilled") {
        setScripts(scriptsResult.value);
      } else {
        toast.error("脚本列表刷新失败");
      }
      if (profilesResult.status === "fulfilled") {
        setProfiles(profilesResult.value || []);
      }
    } finally {
      setRefreshing(false);
    }
  };

  return {
    scripts,
    setScripts,
    profiles,
    loading,
    refreshing,
    handleRefresh,
  };
}
