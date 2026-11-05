import type { RefObject } from "react";
import { Copy, FolderOpen, Play, Plus, Sparkles, Trash2 } from "lucide-react";
import { Button, FormItem, Input, Modal, Switch } from "../../../shared/components";
import { AutomationInstanceSelector } from "./AutomationInstanceSelector";
import {
  buildCurlPreview,
  copyText,
  formatInvokeResult,
  formatPublicApiOutputName,
  type PublicApiOutputEntry,
} from "./AutomationScriptPublicApiModal.helpers";
import {
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptPublicAPIVariable,
  type AutomationScriptRecord,
} from "../automationScripts";
import type { BrowserProfile } from "../types";
import type { AutomationScriptPublicApiInvokeResult } from "../automationScriptApi";
import { AutomationScriptPublicApiBodyExamples } from "./AutomationScriptPublicApiBodyExamples";

interface VisiblePublicApiVariable {
  variable: AutomationScriptPublicAPIVariable;
  index: number;
}

interface AutomationScriptPublicApiModalViewProps {
  open: boolean;
  onClose: () => void;
  busy: boolean;
  script: AutomationScriptRecord;
  launchBaseUrl: string;
  apiAuthEnabled: boolean;
  apiAuthHeader: string;
  profiles: BrowserProfile[];
  fullURL: string;
  resolvedConfig: AutomationScriptPublicAPIConfig;
  requestExampleFallback: string;
  responseExampleFallback: string;
  visibleVariables: VisiblePublicApiVariable[];
  variableError: string;
  requestBodyError: string;
  responseBodyError: string;
  isDualInstanceRuntimeScript: boolean;
  selectedTargetCode: string;
  selectedPrimaryTargetCode: string;
  selectedSecondaryTargetCode: string;
  invokeDisabled: boolean;
  apiKey: string;
  setApiKey: (value: string) => void;
  invoking: boolean;
  invokeResult: AutomationScriptPublicApiInvokeResult | null;
  invokeError: string;
  outputEntries: PublicApiOutputEntry[];
  testSectionRef: RefObject<HTMLDivElement>;
  updateConfig: (patch: Partial<AutomationScriptPublicAPIConfig>) => void;
  updateVariable: (index: number, patch: Partial<AutomationScriptPublicAPIVariable>) => void;
  handleApplySuggestedPath: () => void;
  handleAddVariable: () => void;
  handleRemoveVariable: (index: number) => void;
  handleTargetCodeChange: (code: string) => void;
  handleDualTargetCodeChange: (index: number, code: string) => void;
  handleInvoke: () => Promise<void>;
  handleOpenOutputPath: (path: string) => Promise<void>;
}

export function AutomationScriptPublicApiModalView({
  open,
  onClose,
  busy,
  script,
  launchBaseUrl,
  apiAuthEnabled,
  apiAuthHeader,
  profiles,
  fullURL,
  resolvedConfig,
  requestExampleFallback,
  responseExampleFallback,
  visibleVariables,
  variableError,
  requestBodyError,
  responseBodyError,
  isDualInstanceRuntimeScript,
  selectedTargetCode,
  selectedPrimaryTargetCode,
  selectedSecondaryTargetCode,
  invokeDisabled,
  apiKey,
  setApiKey,
  invoking,
  invokeResult,
  invokeError,
  outputEntries,
  testSectionRef,
  updateConfig,
  updateVariable,
  handleApplySuggestedPath,
  handleAddVariable,
  handleRemoveVariable,
  handleTargetCodeChange,
  handleDualTargetCodeChange,
  handleInvoke,
  handleOpenOutputPath,
}: AutomationScriptPublicApiModalViewProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="对外接口"
      width="1100px"
      footer={
        <div className="flex w-full items-center justify-end gap-2">
          <Button variant="secondary" onClick={onClose} disabled={invoking}>
            取消
          </Button>
          <Button
            type="button"
            onClick={() => void handleInvoke()}
            loading={invoking}
            disabled={invokeDisabled}
          >
            <Play className="h-4 w-4" />
            发送测试请求
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <section className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="flex h-9 items-center rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 font-mono text-xs font-semibold text-[var(--color-text-secondary)]">
              {resolvedConfig.method}
            </span>
            <div className="min-w-0 flex-1 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 py-2">
              <div className="break-all font-mono text-sm text-[var(--color-text-primary)]">
                {fullURL}
              </div>
            </div>
            <Button
              type="button"
              size="sm"
              variant="secondary"
              onClick={() => void copyText(fullURL, "接口地址已复制")}
              disabled={busy}
            >
              <Copy className="h-4 w-4" />
              URL
            </Button>
            <Button
              type="button"
              size="sm"
              variant="secondary"
              onClick={() =>
                void copyText(
                  buildCurlPreview(
                    script,
                    resolvedConfig,
                    launchBaseUrl,
                    apiAuthEnabled,
                    apiAuthHeader,
                  ),
                  "curl 已复制",
                )
              }
              disabled={busy}
            >
              <Copy className="h-4 w-4" />
              curl
            </Button>
            <label className="ml-auto flex h-9 items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 text-sm text-[var(--color-text-secondary)]">
              <span>{resolvedConfig.enabled ? "已启用" : "未启用"}</span>
              <Switch
                checked={resolvedConfig.enabled}
                onChange={(checked) => updateConfig({ enabled: checked })}
                disabled={busy}
              />
            </label>
          </div>
        </section>

        <section className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
          <div className="text-sm font-semibold text-[var(--color-text-primary)]">
            请求配置
          </div>
          <div className="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_180px]">
            <FormItem label="Path">
              <div className="flex gap-2">
                <Input
                  value={resolvedConfig.path}
                  onChange={(event) => updateConfig({ path: event.target.value })}
                  placeholder="mail/proton-first-message"
                  className="font-mono"
                  disabled={busy}
                />
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="!h-9 !min-w-[88px] shrink-0 whitespace-nowrap"
                  onClick={handleApplySuggestedPath}
                  disabled={busy}
                >
                  <Sparkles className="h-4 w-4" />
                  推荐
                </Button>
              </div>
            </FormItem>

            <FormItem label="Timeout">
              <Input
                type="number"
                min={1000}
                max={1800000}
                value={String(resolvedConfig.timeoutMs)}
                onChange={(event) =>
                  updateConfig({ timeoutMs: Number(event.target.value) || 0 })
                }
                disabled={busy}
              />
            </FormItem>
          </div>
        </section>

        <section className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="text-sm font-semibold text-[var(--color-text-primary)]">
              请求参数
            </div>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={handleAddVariable}
              disabled={busy}
            >
              <Plus className="h-4 w-4" />
              新增变量
            </Button>
          </div>

          {visibleVariables.length > 0 ? (
            <div className="mt-3 space-y-2">
              <div className="hidden grid-cols-[180px_minmax(0,1fr)_minmax(0,1fr)_86px_36px] gap-2 px-2 text-xs text-[var(--color-text-muted)] lg:grid">
                <span>变量名</span>
                <span>默认值</span>
                <span>说明</span>
                <span className="text-center">必填</span>
                <span />
              </div>
              {visibleVariables.map(({ variable, index }) => (
                <div
                  key={`${index}-${variable.name}`}
                  className="grid grid-cols-1 gap-2 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-2 py-2 lg:grid-cols-[180px_minmax(0,1fr)_minmax(0,1fr)_86px_36px]"
                >
                  <Input
                    value={variable.name}
                    onChange={(event) =>
                      updateVariable(index, { name: event.target.value })
                    }
                    placeholder="searchQuery"
                    className="font-mono"
                    disabled={busy}
                  />
                  <Input
                    value={variable.defaultValue}
                    onChange={(event) =>
                      updateVariable(index, { defaultValue: event.target.value })
                    }
                    placeholder="默认值"
                    disabled={busy}
                  />
                  <Input
                    value={variable.description}
                    onChange={(event) =>
                      updateVariable(index, { description: event.target.value })
                    }
                    placeholder="说明"
                    disabled={busy}
                  />
                  <label className="flex h-9 items-center justify-center gap-2 rounded-lg border border-[var(--color-border-muted)] text-sm text-[var(--color-text-secondary)]">
                    <input
                      type="checkbox"
                      checked={variable.required}
                      onChange={(event) =>
                        updateVariable(index, { required: event.target.checked })
                      }
                      disabled={busy}
                    />
                    必填
                  </label>
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    onClick={() => handleRemoveVariable(index)}
                    disabled={busy}
                    aria-label="删除变量"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          ) : null}

          {variableError ? (
            <p className="mt-2 text-xs text-[var(--color-error)]">
              {variableError}
            </p>
          ) : null}
        </section>

        <section className="space-y-3">
          {isDualInstanceRuntimeScript ? (
            <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
              <AutomationInstanceSelector
                title="实例 1"
                mode="manual"
                modes={["manual"]}
                profiles={profiles}
                selectedCode={selectedPrimaryTargetCode}
                disabled={busy}
                codePlaceholder="例如 BUYER_001"
                onCodeChange={(code) => handleDualTargetCodeChange(0, code)}
              />
              <AutomationInstanceSelector
                title="实例 2"
                mode="manual"
                modes={["manual"]}
                profiles={profiles}
                selectedCode={selectedSecondaryTargetCode}
                disabled={busy}
                codePlaceholder="例如 BUYER_002"
                onCodeChange={(code) => handleDualTargetCodeChange(1, code)}
              />
            </div>
          ) : (
            <AutomationInstanceSelector
              title="实例"
              mode="manual"
              modes={["manual"]}
              profiles={profiles}
              selectedCode={selectedTargetCode}
              disabled={busy}
              codePlaceholder="例如 BUYER_001"
              onCodeChange={handleTargetCodeChange}
            />
          )}
        </section>

        <AutomationScriptPublicApiBodyExamples
          busy={busy}
          resolvedConfig={resolvedConfig}
          requestExampleFallback={requestExampleFallback}
          responseExampleFallback={responseExampleFallback}
          requestBodyError={requestBodyError}
          responseBodyError={responseBodyError}
          updateConfig={updateConfig}
        />

        <section
          ref={testSectionRef}
          className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="text-sm font-semibold text-[var(--color-text-primary)]">
              测试结果
            </div>
            {invokeResult ? (
              <div className="text-xs text-[var(--color-text-muted)]">
                HTTP {invokeResult.status} {invokeResult.statusText}
              </div>
            ) : null}
          </div>

          {apiAuthEnabled ? (
            <div className="mt-3 max-w-xl">
              <FormItem label={`API Key · ${apiAuthHeader}`}>
                <Input
                  value={apiKey}
                  onChange={(event) => setApiKey(event.target.value)}
                  placeholder="留空使用应用默认 Key"
                />
              </FormItem>
            </div>
          ) : null}

          {invokeError ? (
            <div className="mt-3 rounded-lg border border-[var(--color-error)] bg-[var(--color-bg-surface)] px-3 py-3 text-sm text-[var(--color-text-secondary)]">
              {invokeError}
            </div>
          ) : null}

          {!invokeError && !invokeResult ? (
            <div className="mt-3 text-sm text-[var(--color-text-muted)]">
              暂无测试结果
            </div>
          ) : null}

          {invokeResult ? (
            <div className="mt-3 space-y-3">
              <pre className="overflow-x-auto rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] p-3 text-xs leading-6 text-[var(--color-text-secondary)]">
                <code>{formatInvokeResult(invokeResult)}</code>
              </pre>
              {outputEntries.length > 0 ? (
                <div className="space-y-2">
                  {outputEntries.map((output) => (
                    <div
                      key={`${output.key}-${output.path}`}
                      className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-3 py-2"
                    >
                      <div className="min-w-0 flex-1">
                        <div className="text-sm text-[var(--color-text-primary)]">
                          {output.label} · {formatPublicApiOutputName(output.path)}
                        </div>
                        <div className="mt-1 break-all text-xs text-[var(--color-text-muted)]">
                          {output.path}
                        </div>
                      </div>
                      <Button
                        type="button"
                        size="sm"
                        variant="secondary"
                        onClick={() => void handleOpenOutputPath(output.path)}
                      >
                        <FolderOpen className="h-3.5 w-3.5" />
                        打开文件夹
                      </Button>
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}
        </section>
      </div>
    </Modal>
  );
}
