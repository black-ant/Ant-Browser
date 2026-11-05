import type { Dispatch, SetStateAction } from "react";
import { FileText, Play } from "lucide-react";
import {
  Badge,
  Button,
  FormItem,
  Input,
  Modal,
  Textarea,
} from "../../../shared/components";
import {
  describeAutomationScriptTargetConfig,
  getAutomationScriptTypeLabel,
  type AutomationScriptRecord,
  type AutomationScriptRunRecord,
  type AutomationScriptTargetSelector,
} from "../automationScripts";
import { TargetSelectorEditor } from "../pages/automationScriptDetail/shared";
import type { SelectorSuggestion } from "../pages/automationScriptDetail/helpers";
import { AutomationInstanceSelector } from "./AutomationInstanceSelector";
import { AutomationScriptRunResultPanel } from "./AutomationScriptRunResultPanel";
import type { DemoCreateDraft, DemoPreparationMode, RunVariableInputs, SelectableProfile } from "./AutomationScriptRunModal.types";
import { formatDateTime } from "./AutomationScriptRunModal.helpers";

type Option = { value: string; label: string };

interface AutomationScriptRunModalViewProps {
  open: boolean;
  dirty: boolean;
  script: AutomationScriptRecord;
  running: boolean;
  demoBusy: boolean;
  launchApiExecutable: boolean;
  showDemoProfilePicker: boolean;
  isManualTargetMode: boolean;
  usesStoredTargetConfig: boolean;
  isDualInstanceRuntimeScript: boolean;
  selectorDetachedFromSelectedProfile: boolean;
  showsSelectorInput: boolean;
  hasPublicAPIVariables: boolean;
  hasUnusedPublicAPIVariables: boolean;
  profilesLoading: boolean;
  selectedProfileId: string;
  selectedProfile: SelectableProfile | null;
  selectedLaunchCode: string;
  selectorText: string;
  paramsText: string;
  paramsFieldLabel: string;
  paramsPlaceholder: string;
  demoMode: DemoPreparationMode;
  createDraft: DemoCreateDraft;
  rotateSelector: AutomationScriptTargetSelector;
  variableInputs: RunVariableInputs;
  publicAPIVariables: Array<{ name: string; description?: string; defaultValue?: string }>;
  selectableProfileOptions: Option[];
  templateProfileOptions: Option[];
  codeSuggestions: SelectorSuggestion[];
  profileIdSuggestions: SelectorSuggestion[];
  profileNameSuggestions: SelectorSuggestion[];
  groupOptions: Option[];
  lastRun: AutomationScriptRunRecord | null;
  handleClose: () => void;
  handleOpenScriptDetail: () => void;
  handlePrimaryAction: () => Promise<void>;
  handleSelectedProfileChange: (profileId: string) => void;
  handleLaunchCodeChange: (code: string) => void;
  handleRestoreSelectedProfileSelector: () => void;
  handleSelectorTextChange: (value: string) => void;
  handleOpenOutputPath: (path: string) => Promise<void>;
  setCreateDraft: Dispatch<SetStateAction<DemoCreateDraft>>;
  updateVariableInput: (name: string, value: string) => void;
  updateParamsText: (value: string) => void;
  updateRotateSelector: (patch: Partial<AutomationScriptTargetSelector>) => void;
}

export function AutomationScriptRunModalView({
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
}: AutomationScriptRunModalViewProps) {
  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="执行脚本"
      width="880px"
      footer={
        <>
          <Button
            variant="secondary"
            onClick={handleClose}
            disabled={running || demoBusy}
          >
            关闭
          </Button>
          <Button
            onClick={() => void handlePrimaryAction()}
            loading={running}
            disabled={!launchApiExecutable || demoBusy}
          >
            <Play className="h-4 w-4" />
            立即执行
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-border-muted)] pb-3">
          <div className="min-w-0">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <div className="max-w-[26rem] truncate text-sm font-semibold text-[var(--color-text-primary)]">
                {script.name}
              </div>
              <span className="text-xs text-[var(--color-text-muted)]">
                {formatDateTime(script.updatedAt)}
              </span>
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-2">
                <Badge
                  variant={script.type === "launch-api" ? "info" : "default"}
                  size="sm"
                >
                  {getAutomationScriptTypeLabel(script.type)}
                </Badge>
                <Badge
                  variant={
                    script.status === "ready"
                      ? "success"
                      : script.status === "disabled"
                        ? "default"
                        : "warning"
                  }
                  size="sm"
                  dot
                >
                  {script.status === "ready"
                    ? "可用"
                    : script.status === "disabled"
                      ? "停用"
                      : "草稿"}
                </Badge>
            </div>
          </div>
          <Button
            variant="secondary"
            size="sm"
            onClick={handleOpenScriptDetail}
            disabled={running || demoBusy}
          >
            <FileText className="h-4 w-4" />
            脚本详情
          </Button>
        </div>

        {dirty && (
          <div className="rounded-xl border border-[var(--color-warning)]/30 bg-[var(--color-warning)]/10 px-4 py-3 text-sm text-[var(--color-text-secondary)]">
            {isDualInstanceRuntimeScript
              ? "当前详情页还有未保存修改。本次执行只使用弹窗里的启动配置，不会自动保存页面内容。"
              : "当前详情页还有未保存修改。本次执行只使用弹窗里的 selector / params，不会自动保存页面内容。"}
          </div>
        )}

        {usesStoredTargetConfig && (
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-3 text-sm text-[var(--color-text-secondary)]">
            <div>
              {describeAutomationScriptTargetConfig(script.targetConfig)}
            </div>
            <div className="mt-2 text-xs text-[var(--color-text-muted)]">
              本次执行沿用脚本配置的实例策略，只填写本策略需要的执行配置。
            </div>
          </div>
        )}

        {showDemoProfilePicker && isManualTargetMode ? (
          <AutomationInstanceSelector
            title="传入实例"
            mode="manual"
            modes={["manual"]}
            loading={profilesLoading}
            disabled={running || demoBusy}
            selectedCode={selectedLaunchCode}
            selectedProfileId={selectedProfileId}
            profileOptions={selectableProfileOptions}
            selectPlaceholder="暂无可选实例"
            codePlaceholder="例如 BUYER_001"
            onCodeChange={handleLaunchCodeChange}
            onSelectProfile={handleSelectedProfileChange}
            extra={
              selectorDetachedFromSelectedProfile ? (
                <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
                  <span>当前 selector 已手动修改，执行以下方 JSON 为准。</span>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={handleRestoreSelectedProfileSelector}
                    disabled={running || demoBusy || !selectedProfile}
                  >
                    恢复实例联动
                  </Button>
                </div>
              ) : null
            }
          />
        ) : null}

        {script.targetConfig.mode === "create" ? (
          <AutomationInstanceSelector
            title="模板创建"
            mode="create"
            modes={["create"]}
            loading={profilesLoading}
            disabled={running || demoBusy}
            createName={createDraft.profileName}
            templateProfileId={createDraft.templateProfileId}
            templateOptions={templateProfileOptions}
            templatePlaceholder="暂无模板"
            onCreateNameChange={(profileName) =>
              setCreateDraft((current) => ({
                ...current,
                profileName,
              }))
            }
            onTemplateChange={(templateProfileId) =>
              setCreateDraft((current) => ({
                ...current,
                templateProfileId,
              }))
            }
          />
        ) : null}

        {script.targetConfig.mode === "rotate" ? (
          <AutomationInstanceSelector
            title="条件轮询"
            mode="rotate"
            modes={["rotate"]}
            disabled={running || demoBusy}
            extra={
              <TargetSelectorEditor
                selector={rotateSelector}
                onChange={updateRotateSelector}
                codeSuggestions={codeSuggestions}
                profileIdSuggestions={profileIdSuggestions}
                profileNameSuggestions={profileNameSuggestions}
                groupOptions={groupOptions}
                disabled={running || demoBusy}
              />
            }
          />
        ) : null}

        {showDemoProfilePicker && !isManualTargetMode && demoMode === "select" ? (
          <AutomationInstanceSelector
            title="实例选择"
            mode="select"
            modes={["select"]}
            loading={profilesLoading}
            disabled={running || demoBusy}
            selectedProfileId={selectedProfileId}
            profileOptions={selectableProfileOptions}
            selectPlaceholder="暂无可选实例"
            hint="也可在下方手动填 selector。"
            onSelectProfile={handleSelectedProfileChange}
            extra={
              selectorDetachedFromSelectedProfile ? (
                <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
                  <span>当前 selector 已手动修改，执行以下方 JSON 为准。</span>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={handleRestoreSelectedProfileSelector}
                    disabled={running || demoBusy}
                  >
                    恢复实例联动
                  </Button>
                </div>
              ) : null
            }
          />
        ) : null}

        {script.status === "disabled" ? (
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-4 text-sm text-[var(--color-text-secondary)]">
            该脚本当前处于停用状态，先把状态切回可用再执行。
          </div>
        ) : (
          <div className="space-y-3">
              {hasPublicAPIVariables ? (
                <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-3">
                  <div className="mb-3 flex items-center justify-between gap-3">
                    <div className="text-sm font-semibold text-[var(--color-text-primary)]">
                      接口变量
                    </div>
                    {hasUnusedPublicAPIVariables ? (
                      <div className="text-xs text-[var(--color-text-muted)]">
                        未引用变量不生效
                      </div>
                    ) : null}
                  </div>
                  <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                    {publicAPIVariables.map((variable) => (
                      <FormItem key={variable.name} label={variable.name}>
                        <Input
                          value={variableInputs[variable.name] || ""}
                          onChange={(event) =>
                            updateVariableInput(variable.name, event.target.value)
                          }
                          placeholder={variable.description || variable.defaultValue}
                          className="h-10 rounded-lg"
                          disabled={running || demoBusy}
                        />
                      </FormItem>
                    ))}
                  </div>
                </div>
              ) : null}

            <div
              className={
                showsSelectorInput
                  ? "grid grid-cols-1 gap-3 xl:grid-cols-2"
                  : "grid grid-cols-1 gap-3"
              }
            >
              {showsSelectorInput && (
                <FormItem label="目标选择器 JSON">
                  <Textarea
                    rows={hasPublicAPIVariables ? 6 : 9}
                    value={selectorText}
                    onChange={(event) => handleSelectorTextChange(event.target.value)}
                    className="font-mono text-xs"
                    placeholder='{"code":"DEMO_ABC123"}'
                    disabled={running || demoBusy}
                  />
                </FormItem>
              )}

              <FormItem label={paramsFieldLabel}>
                <Textarea
                  rows={hasPublicAPIVariables ? 6 : 9}
                  value={paramsText}
                  onChange={(event) => updateParamsText(event.target.value)}
                  className="font-mono text-xs"
                  placeholder={paramsPlaceholder}
                  disabled={running || demoBusy}
                />
              </FormItem>
            </div>
          </div>
        )}

        {lastRun && (
          <AutomationScriptRunResultPanel
            lastRun={lastRun}
            handleOpenOutputPath={handleOpenOutputPath}
          />
        )}
      </div>
    </Modal>
  );
}
