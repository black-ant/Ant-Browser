import { Navigate, Route, Routes } from "react-router-dom";
import { lazyNamed } from "./lazyNamed";

const SettingsPage = lazyNamed(
  () => import("../modules/settings/SettingsPage"),
  "SettingsPage",
);
const AutomationSettingsPage = lazyNamed(
  () => import("../modules/settings/AutomationSettingsPage"),
  "AutomationSettingsPage",
);
const BackupPage = lazyNamed(
  () => import("../modules/backup/BackupPage"),
  "BackupPage",
);
const S3ConfigPage = lazyNamed(
  () => import("../modules/backup/channels/s3/S3ConfigPage"),
  "S3ConfigPage",
);
const ProfilePage = lazyNamed(
  () => import("../modules/profile/ProfilePage"),
  "ProfilePage",
);
const NotificationsPage = lazyNamed(
  () => import("../modules/notifications/NotificationsPage"),
  "NotificationsPage",
);
const ChartsPage = lazyNamed(
  () => import("../modules/charts/ChartsPage"),
  "ChartsPage",
);
const BrowserListPage = lazyNamed(
  () => import("../modules/browser/pages/BrowserListPage"),
  "BrowserListPage",
);
const BrowserDetailPage = lazyNamed(
  () => import("../modules/browser/pages/BrowserDetailPage"),
  "BrowserDetailPage",
);
const BrowserEditPage = lazyNamed(
  () => import("../modules/browser/pages/BrowserEditPage"),
  "BrowserEditPage",
);
const BrowserCopyPage = lazyNamed(
  () => import("../modules/browser/pages/BrowserCopyPage"),
  "BrowserCopyPage",
);
const BrowserLogsPage = lazyNamed(
  () => import("../modules/browser/pages/BrowserLogsPage"),
  "BrowserLogsPage",
);
const ProxyPoolPage = lazyNamed(
  () => import("../modules/browser/pages/ProxyPoolPage"),
  "ProxyPoolPage",
);
const CoreManagementPage = lazyNamed(
  () => import("../modules/browser/pages/CoreManagementPage"),
  "CoreManagementPage",
);
const BookmarkSettingsPage = lazyNamed(
  () => import("../modules/browser/pages/BookmarkSettingsPage"),
  "BookmarkSettingsPage",
);
const ExtensionManagementPage = lazyNamed(
  () => import("../modules/browser/pages/ExtensionManagementPage"),
  "ExtensionManagementPage",
);
const LaunchApiDocsPage = lazyNamed(
  () => import("../modules/browser/pages/LaunchApiDocsPage"),
  "LaunchApiDocsPage",
);
const TagManagementPage = lazyNamed(
  () => import("../modules/browser/pages/TagManagementPage"),
  "TagManagementPage",
);
const AutomationPage = lazyNamed(
  () => import("../modules/browser/pages/AutomationPage"),
  "AutomationPage",
);
const AutomationScriptDetailPage = lazyNamed(
  () => import("../modules/browser/pages/AutomationScriptDetailPage"),
  "AutomationScriptDetailPage",
);

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/browser/list" replace />} />
      <Route path="/charts" element={<ChartsPage />} />
      <Route path="/settings" element={<SettingsPage />} />
      <Route path="/settings/automation" element={<AutomationSettingsPage />} />
      <Route path="/system/backup" element={<BackupPage />} />
      <Route path="/system/backup/s3" element={<S3ConfigPage />} />
      <Route path="/profile" element={<ProfilePage />} />
      <Route path="/notifications" element={<NotificationsPage />} />
      <Route path="/browser/list" element={<BrowserListPage />} />
      <Route path="/browser/detail/:id" element={<BrowserDetailPage />} />
      <Route path="/browser/edit/:id" element={<BrowserEditPage />} />
      <Route path="/browser/copy/:id" element={<BrowserCopyPage />} />
      <Route
        path="/browser/monitor"
        element={<Navigate to="/browser/list" replace />}
      />
      <Route path="/browser/logs" element={<BrowserLogsPage />} />
      <Route path="/browser/proxy-pool" element={<ProxyPoolPage />} />
      <Route path="/browser/cores" element={<CoreManagementPage />} />
      <Route path="/browser/extensions" element={<ExtensionManagementPage />} />
      <Route path="/browser/bookmarks" element={<BookmarkSettingsPage />} />
      <Route path="/browser/automation" element={<AutomationPage />} />
      <Route
        path="/browser/automation/:scriptId"
        element={<AutomationScriptDetailPage />}
      />
      <Route path="/system/docs" element={<LaunchApiDocsPage />} />
      <Route
        path="/browser/launch-api"
        element={<Navigate to="/system/docs" replace />}
      />
      <Route path="/browser/tags" element={<TagManagementPage />} />
      <Route
        path="/system/tutorial"
        element={<Navigate to="/system/docs" replace />}
      />
    </Routes>
  );
}
