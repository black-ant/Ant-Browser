import { Navigate, Route, Routes } from "react-router-dom";
import { lazyNamed } from "./lazyNamed";

const SettingsLegacyPage = lazyNamed(
  () => import("../modules/settings/SettingsPage"),
  "SettingsPage",
);
const FishSettingsPage = lazyNamed(
  () => import("../ui-v2/fish/FishSettingsPage"),
  "FishSettingsPage",
);
const BackupPage = lazyNamed(
  () => import("../modules/backup/BackupPage"),
  "BackupPage",
);
const S3ConfigPage = lazyNamed(
  () => import("../modules/backup/channels/s3/S3ConfigPage"),
  "S3ConfigPage",
);
const ProfileLegacyPage = lazyNamed(
  () => import("../modules/profile/ProfilePage"),
  "ProfilePage",
);
const FishProfilePage = lazyNamed(
  () => import("../ui-v2/fish/FishProfilePage"),
  "FishProfilePage",
);
const NotificationsPage = lazyNamed(
  () => import("../modules/notifications/NotificationsPage"),
  "NotificationsPage",
);
const ChartsLegacyPage = lazyNamed(
  () => import("../modules/charts/ChartsPage"),
  "ChartsPage",
);
const FishChartsPage = lazyNamed(
  () => import("../ui-v2/fish/FishChartsPage"),
  "FishChartsPage",
);
const BrowserListLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/BrowserListPage"),
  "BrowserListPage",
);
const FishBrowserListPage = lazyNamed(
  () => import("../ui-v2/fish/FishBrowserListPage"),
  "FishBrowserListPage",
);
const BrowserDetailLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/BrowserDetailPage"),
  "BrowserDetailPage",
);
const FishBrowserDetailPage = lazyNamed(
  () => import("../ui-v2/fish/FishBrowserDetailPage"),
  "FishBrowserDetailPage",
);
const BrowserEditLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/BrowserEditPage"),
  "BrowserEditPage",
);
const FishBrowserEditPage = lazyNamed(
  () => import("../ui-v2/fish/FishBrowserEditPage"),
  "FishBrowserEditPage",
);
const BrowserCopyLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/BrowserCopyPage"),
  "BrowserCopyPage",
);
const FishBrowserCopyPage = lazyNamed(
  () => import("../ui-v2/fish/FishBrowserCopyPage"),
  "FishBrowserCopyPage",
);
const BrowserLogsLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/BrowserLogsPage"),
  "BrowserLogsPage",
);
const FishLogsPage = lazyNamed(
  () => import("../ui-v2/fish/FishLogsPage"),
  "FishLogsPage",
);
const ProxyPoolLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/ProxyPoolPage"),
  "ProxyPoolPage",
);
const FishProxyPoolPage = lazyNamed(
  () => import("../ui-v2/fish/FishProxyPoolPage"),
  "FishProxyPoolPage",
);
const CoreManagementLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/CoreManagementPage"),
  "CoreManagementPage",
);
const FishCoreManagementPage = lazyNamed(
  () => import("../ui-v2/fish/FishCoreManagementPage"),
  "FishCoreManagementPage",
);
const BookmarkLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/BookmarkSettingsPage"),
  "BookmarkSettingsPage",
);
const FishBookmarksPage = lazyNamed(
  () => import("../ui-v2/fish/FishBookmarksPage"),
  "FishBookmarksPage",
);
const ExtensionLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/ExtensionManagementPage"),
  "ExtensionManagementPage",
);
const FishExtensionsPage = lazyNamed(
  () => import("../ui-v2/fish/FishExtensionsPage"),
  "FishExtensionsPage",
);
const DocsLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/LaunchApiDocsPage"),
  "LaunchApiDocsPage",
);
const FishDocsPage = lazyNamed(
  () => import("../ui-v2/fish/FishDocsPage"),
  "FishDocsPage",
);
const TagsLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/TagManagementPage"),
  "TagManagementPage",
);
const FishTagsPage = lazyNamed(
  () => import("../ui-v2/fish/FishTagsPage"),
  "FishTagsPage",
);
const AutomationLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/AutomationPage"),
  "AutomationPage",
);
const FishAutomationPage = lazyNamed(
  () => import("../ui-v2/fish/FishAutomationPage"),
  "FishAutomationPage",
);
const AutomationDetailLegacyPage = lazyNamed(
  () => import("../modules/browser/pages/AutomationScriptDetailPage"),
  "AutomationScriptDetailPage",
);
const FishAutomationDetailPage = lazyNamed(
  () => import("../ui-v2/fish/FishAutomationDetailPage"),
  "FishAutomationDetailPage",
);

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/browser/list" replace />} />
      <Route path="/charts" element={<FishChartsPage />} />
      <Route path="/charts-legacy" element={<ChartsLegacyPage />} />
      <Route path="/settings" element={<FishSettingsPage />} />
      <Route path="/settings-legacy" element={<SettingsLegacyPage />} />
      <Route path="/system/backup" element={<BackupPage />} />
      <Route path="/system/backup/s3" element={<S3ConfigPage />} />
      <Route path="/profile" element={<FishProfilePage />} />
      <Route path="/profile-legacy" element={<ProfileLegacyPage />} />
      <Route path="/notifications" element={<NotificationsPage />} />
      <Route path="/browser/list" element={<FishBrowserListPage />} />
      <Route path="/browser/list-legacy" element={<BrowserListLegacyPage />} />
      <Route path="/browser/list-v2" element={<Navigate to="/browser/list" replace />} />
      <Route path="/browser/detail/:id" element={<FishBrowserDetailPage />} />
      <Route path="/browser/detail-legacy/:id" element={<BrowserDetailLegacyPage />} />
      <Route path="/browser/edit/:id" element={<FishBrowserEditPage />} />
      <Route path="/browser/edit-fish/:id" element={<FishBrowserEditPage />} />
      <Route path="/browser/edit-legacy/:id" element={<BrowserEditLegacyPage />} />
      <Route path="/browser/copy/:id" element={<FishBrowserCopyPage />} />
      <Route path="/browser/copy-legacy/:id" element={<BrowserCopyLegacyPage />} />
      <Route path="/browser/monitor" element={<Navigate to="/browser/list" replace />} />
      <Route path="/browser/logs" element={<FishLogsPage />} />
      <Route path="/browser/logs-legacy" element={<BrowserLogsLegacyPage />} />
      <Route path="/browser/proxy-pool" element={<FishProxyPoolPage />} />
      <Route path="/browser/proxy-pool-legacy" element={<ProxyPoolLegacyPage />} />
      <Route path="/browser/cores" element={<FishCoreManagementPage />} />
      <Route path="/browser/cores-legacy" element={<CoreManagementLegacyPage />} />
      <Route path="/browser/extensions" element={<FishExtensionsPage />} />
      <Route path="/browser/extensions-legacy" element={<ExtensionLegacyPage />} />
      <Route path="/browser/bookmarks" element={<FishBookmarksPage />} />
      <Route path="/browser/bookmarks-legacy" element={<BookmarkLegacyPage />} />
      <Route path="/browser/automation" element={<FishAutomationPage />} />
      <Route path="/browser/automation-legacy" element={<AutomationLegacyPage />} />
      <Route path="/browser/automation/:scriptId" element={<FishAutomationDetailPage />} />
      <Route path="/browser/automation-legacy/:scriptId" element={<AutomationDetailLegacyPage />} />
      <Route path="/system/docs" element={<FishDocsPage />} />
      <Route path="/system/docs-legacy" element={<DocsLegacyPage />} />
      <Route path="/browser/launch-api" element={<Navigate to="/system/docs" replace />} />
      <Route path="/browser/tags" element={<FishTagsPage />} />
      <Route path="/browser/tags-legacy" element={<TagsLegacyPage />} />
      <Route path="/system/tutorial" element={<Navigate to="/system/docs" replace />} />
    </Routes>
  );
}
