import {
  ArrowLeft,
  Download,
  Link,
  Play,
  RefreshCw,
  Save,
  Trash2,
} from "lucide-react";
import {
  Button,
  FormItem,
  Input,
  Select,
  Textarea,
} from "../../../../shared/components";
import {
  AUTOMATION_SCRIPT_STATUS_OPTIONS,
  getAutomationScriptRefreshLabel,
  getAutomationScriptTypeLabel,
  type AutomationScriptTargetConfig,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptRecord,
  type AutomationScriptStatus,
} from "../../automationScripts";
import { AutomationInstanceSelector } from "../../components/AutomationInstanceSelector";
import type { ScriptParamsHelpContent } from "./paramsHelp";
import {
  formatDateTime,
  formatScriptSource,
  type DualRuntimePreviewResult,
} from "./helpers";
import { AutomationScriptDetailBodyPanels } from "./AutomationScriptDetailBodyPanels";
import {
  CompactMetaField,
  DetailPanel,
  StructuredInfoCell,
} from "./shared";
import type { AutomationScriptDetailBusyAction } from "./useAutomationScriptDetailState";

interface AutomationScriptDetailPanelsProps {
  draft: AutomationScriptRecord;
  dirty: boolean;
  busy: boolean;
  busyAction: AutomationScriptDetailBusyAction;
  canRefresh: boolean;
  isDualInstanceRuntimeScript: boolean;
  isLaunchApiScript: boolean;
  usesManualSelector: boolean;
  resolvedPublicAPI: AutomationScriptPublicAPIConfig;
  publicAPIURL: string;
  paramsHelp: ScriptParamsHelpContent | null;
  launchBaseUrl: string;
  apiAuthHeader: string;
  dualRuntimePreview: DualRuntimePreviewResult;
  showDualRuntimeRequests: boolean;
  openClawDualSiteCommand: string;
  onLeavePage: () => void;
  onUpdateDraft: (patch: Partial<AutomationScriptRecord>) => void;
  onUpdateTargetConfig: (patch: Partial<AutomationScriptTargetConfig>) => void;
  onOpenRunModal: () => void;
  onOpenPublicApiManager: () => void;
  onOpenPublicApiTester: () => void;
  onOpenExportModal: () => void;
  onSave: () => void;
  onDelete: () => void;
  onRefresh: () => void;
  onOpenExistingTargetConfig: () => void;
  onToggleDualRuntimeRequests: () => void;
  onCopyOpenClawCommand: () => void;
  onOpenParamsHelp: () => void;
}

export function AutomationScriptDetailPanels({
  draft,
  dirty,
  busy,
  busyAction,
  canRefresh,
  isDualInstanceRuntimeScript,
  isLaunchApiScript,
  usesManualSelector,
  resolvedPublicAPI,
  publicAPIURL,
  paramsHelp,
  launchBaseUrl,
  apiAuthHeader,
  dualRuntimePreview,
  showDualRuntimeRequests,
  openClawDualSiteCommand,
  onLeavePage,
  onUpdateDraft,
  onUpdateTargetConfig,
  onOpenRunModal,
  onOpenPublicApiManager,
  onOpenPublicApiTester,
  onOpenExportModal,
  onSave,
  onDelete,
  onRefresh,
  onOpenExistingTargetConfig,
  onToggleDualRuntimeRequests,
  onCopyOpenClawCommand,
  onOpenParamsHelp,
}: AutomationScriptDetailPanelsProps) {
  const headerRunButtonClassName =
    "!h-8 !shrink-0 !whitespace-nowrap !border !px-3 hover:!opacity-90";

  return (
    <div className="space-y-5 animate-fade-in">
      <section className="rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-3 shadow-[var(--shadow-sm)]">
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="secondary" size="sm" onClick={onLeavePage}>
            <ArrowLeft className="h-4 w-4" />
            返回目录
          </Button>
          <div className="min-w-[220px] flex-1">
            <Input
              value={draft.name}
              onChange={(event) => onUpdateDraft({ name: event.target.value })}
              placeholder="脚本名称"
              className="h-9 text-sm font-semibold"
            />
          </div>
          {canRefresh ? (
            <Button
              size="sm"
              variant="secondary"
              onClick={onRefresh}
              loading={busyAction === "refresh"}
              disabled={busyAction !== "none" && busyAction !== "refresh"}
            >
              <RefreshCw className="h-4 w-4" />
              {getAutomationScriptRefreshLabel(draft.source)}
            </Button>
          ) : null}
          <Button
            size="sm"
            onClick={onOpenRunModal}
            disabled={busyAction !== "none"}
            className={`${headerRunButtonClassName} hover:!border-[var(--color-accent-hover)] hover:!bg-[var(--color-accent-hover)] focus-visible:!ring-[var(--color-accent)]`}
            style={{
              backgroundColor: "var(--color-accent)",
              borderColor: "var(--color-accent)",
              color: "var(--color-text-inverse)",
            }}
          >
            <Play className="h-4 w-4" />
            执行脚本
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={
              resolvedPublicAPI.enabled
                ? onOpenPublicApiTester
                : onOpenPublicApiManager
            }
            disabled={busyAction !== "none"}
            className={`${headerRunButtonClassName} hover:!border-[var(--color-accent-hover)] hover:!bg-[var(--color-accent-hover)] focus-visible:!ring-[var(--color-accent)]`}
            style={{
              backgroundColor: "var(--color-accent)",
              borderColor: "var(--color-accent)",
              color: "var(--color-text-inverse)",
            }}
          >
            <Link className="h-4 w-4" />
            {resolvedPublicAPI.enabled ? "执行接口" : "配置接口"}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={onOpenExportModal}
            loading={busyAction === "export"}
            disabled={busyAction !== "none" && busyAction !== "export"}
            className={`${headerRunButtonClassName} hover:!border-black hover:!bg-black focus-visible:!ring-black`}
            style={{
              backgroundColor: "#000000",
              borderColor: "#000000",
              color: "#ffffff",
            }}
          >
            <Download className="h-4 w-4" />
            导出
          </Button>
          <Button
            size="sm"
            onClick={onSave}
            loading={busyAction === "save"}
            disabled={busyAction !== "none" && busyAction !== "save"}
          >
            <Save className="h-4 w-4" />
            保存
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={onDelete}
            loading={busyAction === "delete"}
            disabled={busyAction !== "none" && busyAction !== "delete"}
          >
            <Trash2 className="h-4 w-4" />
            删除
          </Button>
        </div>
      </section>

      <div className="grid grid-cols-1 items-stretch gap-4 2xl:grid-cols-[minmax(0,1.1fr)_minmax(320px,0.9fr)]">
        <DetailPanel title="基础信息" className="h-full">
          <FormItem label="描述">
            <Textarea
              rows={2}
              value={draft.description}
              onChange={(event) =>
                onUpdateDraft({ description: event.target.value })
              }
              placeholder="说明这套脚本要做什么"
            />
          </FormItem>

          <div className="overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-[var(--shadow-sm)]">
            <div className="grid grid-cols-1 divide-y divide-[var(--color-border-muted)] md:grid-cols-2 md:divide-x md:divide-y xl:grid-cols-4 xl:divide-y-0">
              <StructuredInfoCell label="类型">
                {getAutomationScriptTypeLabel(draft.type)}
              </StructuredInfoCell>
              <StructuredInfoCell label="状态">
                <Select
                  value={draft.status}
                  options={AUTOMATION_SCRIPT_STATUS_OPTIONS}
                  onChange={(event) =>
                    onUpdateDraft({
                      status: event.target.value as AutomationScriptStatus,
                    })
                  }
                  className="h-9"
                  disabled={busy}
                />
              </StructuredInfoCell>
              <StructuredInfoCell label="最近更新">
                {formatDateTime(draft.updatedAt)}
              </StructuredInfoCell>
              <StructuredInfoCell label="编辑状态">
                <span
                  className={dirty ? "text-[var(--color-warning)]" : undefined}
                >
                  {dirty ? "未保存" : "已保存"}
                </span>
              </StructuredInfoCell>
            </div>
          </div>

          <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-2.5">
            <div className="text-sm text-[var(--color-text-secondary)]">
              {formatScriptSource(draft)}
            </div>
            {draft.source.importedAt ? (
              <div className="mt-1 text-xs text-[var(--color-text-muted)]">
                最近导入 {formatDateTime(draft.source.importedAt)}
              </div>
            ) : null}
          </div>
        </DetailPanel>

        {isDualInstanceRuntimeScript ? (
          <DetailPanel title="执行模型" className="h-full">
            <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
              <CompactMetaField label="类型" value="接口模拟" />
              <CompactMetaField label="执行方式" value="逐个启动" />
              <CompactMetaField label="Selector" value="不使用" />
            </div>
          </DetailPanel>
        ) : (
          <DetailPanel
            title="实例策略"
            className="h-full"
          >
            <AutomationInstanceSelector
              title="实例来源"
              mode={
                draft.targetConfig.mode === "existing"
                  ? "manual"
                  : draft.targetConfig.mode
              }
              modes={["manual", "create", "rotate"]}
              showFields={false}
              disabled={busy}
              onModeChange={(mode) =>
                onUpdateTargetConfig({
                  mode: mode as AutomationScriptTargetConfig["mode"],
                })
              }
            />
          </DetailPanel>
        )}
      </div>

      <AutomationScriptDetailBodyPanels
        draft={draft}
        busy={busy}
        isDualInstanceRuntimeScript={isDualInstanceRuntimeScript}
        isLaunchApiScript={isLaunchApiScript}
        usesManualSelector={usesManualSelector}
        resolvedPublicAPI={resolvedPublicAPI}
        publicAPIURL={publicAPIURL}
        paramsHelp={paramsHelp}
        launchBaseUrl={launchBaseUrl}
        apiAuthHeader={apiAuthHeader}
        dualRuntimePreview={dualRuntimePreview}
        showDualRuntimeRequests={showDualRuntimeRequests}
        openClawDualSiteCommand={openClawDualSiteCommand}
        onUpdateDraft={onUpdateDraft}
        onOpenTargetConfig={onOpenExistingTargetConfig}
        onToggleDualRuntimeRequests={onToggleDualRuntimeRequests}
        onCopyOpenClawCommand={onCopyOpenClawCommand}
        onOpenParamsHelp={onOpenParamsHelp}
      />
    </div>
  );
}
