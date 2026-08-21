import { Suspense, useEffect, useState } from "react";
import { BrowserRouter as Router } from "react-router-dom";
import { ThemeProvider } from "./shared/theme";
import { Layout } from "./shared/layout";
import { ToastContainer, Modal, Button, Loading, toast } from "./shared/components";
import { AlertCircle } from "lucide-react";
import { AppRoutes } from "./routes/AppRoutes";
import { lazyNamed } from "./routes/lazyNamed";
import { useNotificationStore } from "./store/notificationStore";
import { useBackupStore } from "./store/backupStore";
import { installWailsOperationLogger } from "./utils/wailsOperationLogger";
import {
  ForceQuit as ForceQuitApp,
  QuitAppOnly as QuitAppOnlyApp,
} from "./wailsjs/go/main/App";
import {
  Environment,
  Quit,
  WindowHide,
  WindowMinimise,
} from "./wailsjs/runtime/runtime";

const QuickLaunchModal = lazyNamed(
  () => import("./modules/browser/components/QuickLaunchModal"),
  "QuickLaunchModal",
);

function useWailsNotifications() {
  const addNotification = useNotificationStore((s) => s.addNotification);

  useEffect(() => {
    const runtime = (window as any).runtime;
    if (!runtime?.EventsOn) return;

    const offCrashed = runtime.EventsOn(
      "browser:instance:crashed",
      (data: { profileId: string; profileName: string; error: string }) => {
        const action = data.profileId
          ? {
              type: "navigate" as const,
              label: "查看实例",
              path: `/browser/detail/${encodeURIComponent(data.profileId)}`,
            }
          : undefined;
        addNotification({
          type: "error",
          title: "实例异常退出",
          message: `「${data.profileName || data.profileId}」意外崩溃：${data.error}`,
          source: "runtime",
          dedupeKey: `browser:instance:crashed:${data.profileId}`,
          persistent: true,
          action,
        });
      },
    );

    const offBridgeFailed = runtime.EventsOn(
      "proxy:bridge:failed",
      (data: { profileId: string; profileName: string; error: string }) => {
        const action = data.profileId
          ? {
              type: "navigate" as const,
              label: "查看实例",
              path: `/browser/detail/${encodeURIComponent(data.profileId)}`,
            }
          : undefined;
        toast.warning(`「${data.profileName || data.profileId}」代理桥接失败，已直连启动`, 8_000, {
          title: "代理已降级直连",
          source: "runtime",
          dedupeKey: `proxy:bridge:failed:${data.profileId}`,
          persistent: true,
          action,
        });
      },
    );

    const offBridgeDied = runtime.EventsOn(
      "proxy:bridge:died",
      (data: { key: string; error: string }) => {
        addNotification({
          type: "warning",
          title: "连接池节点失效",
          message: `代理节点 ${data.key} 连接中断，相关实例可能无法访问网络`,
          source: "runtime",
          dedupeKey: `proxy:bridge:died:${data.key}`,
          persistent: true,
          action: {
            type: "navigate",
            label: "查看代理池",
            path: "/browser/proxy-pool",
          },
        });
      },
    );

    return () => {
      offCrashed?.();
      offBridgeFailed?.();
      offBridgeDied?.();
    };
  }, [addNotification]);
}

function useGlobalErrorNotifications() {
  const addNotification = useNotificationStore((s) => s.addNotification);

  useEffect(() => {
    const toMessage = (value: unknown) => {
      if (value instanceof Error) return value.message || String(value);
      if (typeof value === "string") return value;
      try {
        return JSON.stringify(value);
      } catch {
        return String(value);
      }
    };

    const handleError = (event: ErrorEvent) => {
      const message = event.message || toMessage(event.error) || "未知脚本错误";
      addNotification({
        type: "error",
        title: "前端异常",
        message,
        source: "frontend",
        dedupeKey: `frontend:error:${message.slice(0, 160)}`,
        persistent: true,
      });
    };

    const handleUnhandledRejection = (event: PromiseRejectionEvent) => {
      const message = toMessage(event.reason) || "未知 Promise 异常";
      addNotification({
        type: "error",
        title: "未处理异步异常",
        message,
        source: "frontend",
        dedupeKey: `frontend:rejection:${message.slice(0, 160)}`,
        persistent: true,
      });
    };

    window.addEventListener("error", handleError);
    window.addEventListener("unhandledrejection", handleUnhandledRejection);
    return () => {
      window.removeEventListener("error", handleError);
      window.removeEventListener("unhandledrejection", handleUnhandledRejection);
    };
  }, [addNotification]);
}

function CloseConfirmModal() {
  const [open, setOpen] = useState(false);
  const [platform, setPlatform] = useState("windows");
  const [quittingAction, setQuittingAction] = useState<
    "app-only" | "app-and-browser" | null
  >(null);
  const importInProgress = useBackupStore((s) => s.importInProgress);
  const importProgress = useBackupStore((s) => s.importProgress);
  const importMessage = useBackupStore((s) => s.importMessage);
  const supportsTray = platform === "windows";
  const quitting = quittingAction !== null;

  useEffect(() => {
    const runtime = (window as any).runtime;
    if (!runtime?.EventsOn) return;

    const off = runtime.EventsOn("app:request-close", () => {
      setQuittingAction(null);
      setOpen(true);
    });
    return () => {
      if (typeof off === "function") off();
    };
  }, []);

  useEffect(() => {
    let cancelled = false;

    Environment()
      .then((info) => {
        if (!cancelled && info?.platform) {
          setPlatform(info.platform);
        }
      })
      .catch(() => {});

    return () => {
      cancelled = true;
    };
  }, []);

  const closeModal = () => {
    if (quitting) return;
    setOpen(false);
  };

  const handleMinimize = () => {
    if (quitting) return;
    setOpen(false);
    if (supportsTray) {
      WindowHide();
      return;
    }
    WindowMinimise();
  };

  const handleQuitAppOnly = async () => {
    setQuittingAction("app-only");
    try {
      await QuitAppOnlyApp();
    } catch (error) {
      console.error("QuitAppOnly failed", error);
      setQuittingAction(null);
    }
  };

  const handleQuitAppAndBrowsers = async () => {
    setQuittingAction("app-and-browser");
    try {
      await Promise.race([
        ForceQuitApp(),
        new Promise((resolve) => setTimeout(resolve, 1200)),
      ]);
    } catch (error) {
      console.error("ForceQuit failed, falling back to runtime.Quit()", error);
    }
    Quit();
  };

  return (
    <Modal
      open={open}
      onClose={closeModal}
      title={importInProgress ? "关闭应用确认" : undefined}
      width={importInProgress ? "360px" : "420px"}
      closable={!quitting}
    >
      <div className="flex flex-col items-center pt-2 pb-6 px-4">
        <div
          className={`w-12 h-12 rounded-full flex items-center justify-center mb-4 ${
            importInProgress
              ? "bg-amber-50 text-amber-500"
              : "bg-red-50 text-red-500"
          }`}
        >
          <AlertCircle className="w-6 h-6" />
        </div>
        {importInProgress && (
          <h3 className="text-lg font-medium text-[var(--color-text-primary)] mb-2">
            正在导入备份，是否关闭？
          </h3>
        )}
        {importInProgress ? (
          <p className="text-sm text-[var(--color-text-secondary)] text-center mb-6">
            当前正在导入备份
            {importProgress > 0 ? `（${importProgress}%）` : ""}。
            <br />
            {importMessage || "强制关闭会中断本次导入，是否仍要关闭应用？"}
          </p>
        ) : (
          <p className="mb-6 text-sm text-center text-[var(--color-text-secondary)]">
            可仅退出应用，或连同浏览器一起关闭。
          </p>
        )}

        <div
          className={`w-full ${importInProgress ? "flex gap-3" : "flex flex-col gap-2"}`}
        >
          {importInProgress ? (
            <>
              <Button
                variant="secondary"
                className="flex-1"
                onClick={closeModal}
                disabled={quitting}
              >
                继续加载
              </Button>
              <Button
                variant="danger"
                className="flex-1"
                onClick={handleQuitAppAndBrowsers}
                loading={quittingAction === "app-and-browser"}
              >
                仍要关闭
              </Button>
            </>
          ) : (
            <>
              <Button
                variant="secondary"
                className="w-full !bg-[#f3f4f6] !border-[#e5e7eb] !text-[var(--color-text-primary)] hover:!bg-[#e5e7eb]"
                onClick={supportsTray ? handleMinimize : closeModal}
                disabled={quitting}
              >
                {supportsTray ? "最小化到托盘" : "取消"}
              </Button>
              <Button
                className="w-full"
                onClick={handleQuitAppOnly}
                loading={quittingAction === "app-only"}
                disabled={quitting}
              >
                仅退出应用
              </Button>
              <Button
                variant="danger"
                className="w-full"
                onClick={handleQuitAppAndBrowsers}
                loading={quittingAction === "app-and-browser"}
                disabled={quitting}
              >
                退出应用与浏览器
              </Button>
            </>
          )}
        </div>
      </div>
    </Modal>
  );
}

function App() {
  useEffect(() => {
    installWailsOperationLogger();
  }, []);
  useWailsNotifications();
  useGlobalErrorNotifications();
  const [quickLaunchOpen, setQuickLaunchOpen] = useState(false);
  const routeFallback = (
    <div className="flex min-h-[240px] items-center justify-center py-10">
      <Loading text="页面加载中..." />
    </div>
  );

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.isComposing) return;
      if (!(event.ctrlKey || event.metaKey)) return;
      if (event.key.toLowerCase() !== "k") return;
      event.preventDefault();
      setQuickLaunchOpen((prev) => !prev);
    };

    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
    };
  }, []);

  return (
    <ThemeProvider>
      <Router>
        <Layout>
          <Suspense fallback={routeFallback}>
            <AppRoutes />
          </Suspense>
        </Layout>
        <ToastContainer />
        <CloseConfirmModal />
        <Suspense fallback={null}>
          {quickLaunchOpen ? (
            <QuickLaunchModal
              open={quickLaunchOpen}
              onClose={() => setQuickLaunchOpen(false)}
            />
          ) : null}
        </Suspense>
      </Router>
    </ThemeProvider>
  );
}

export default App;
