import { Copy, Settings } from "lucide-react";
import {
  Badge,
  Button,
  FormItem,
  Textarea,
} from "../../../../shared/components";
import type {
  AutomationScriptPublicAPIConfig,
  AutomationScriptRecord,
} from "../../automationScripts";
import type { ScriptParamsHelpContent } from "./paramsHelp";
import {
  buildRuntimeSessionHttpPreview,
  type DualRuntimePreviewResult,
} from "./helpers";
import {
  CompactMetaField,
  DetailPanel,
  FieldLabelWithHelp,
} from "./shared";

interface AutomationScriptDetailBodyPanelsProps {
  draft: AutomationScriptRecord;
  busy: boolean;
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
  onUpdateDraft: (patch: Partial<AutomationScriptRecord>) => void;
  onOpenTargetConfig: () => void;
  onToggleDualRuntimeRequests: () => void;
  onCopyOpenClawCommand: () => void;
  onOpenParamsHelp: () => void;
}

function formatManualSelectorSummary(selectorText: string): string {
  const normalized = selectorText.trim();
  if (!normalized) {
    return "未选择实例";
  }

  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return "已设置自定义目标";
    }
    const selector = parsed as Record<string, unknown>;
    const code = String(selector.code || "").trim();
    if (code) {
      return `Code：${code}`;
    }
    const profileName = String(selector.profileName || "").trim();
    if (profileName) {
      return `实例：${profileName}`;
    }
    const profileId = String(selector.profileId || "").trim();
    if (profileId) {
      return `实例 ID：${profileId}`;
    }
    return "已设置自定义目标";
  } catch {
    return "已设置自定义目标";
  }
}

export function AutomationScriptDetailBodyPanels({
  draft,
  busy,
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
  onUpdateDraft,
  onOpenTargetConfig,
  onToggleDualRuntimeRequests,
  onCopyOpenClawCommand,
  onOpenParamsHelp,
}: AutomationScriptDetailBodyPanelsProps) {
  return (
    <>
      <DetailPanel
        title="对外接口"
        actions={
          <Badge
            variant={resolvedPublicAPI.enabled ? "success" : "default"}
            size="sm"
          >
            {resolvedPublicAPI.enabled ? "已启用" : "未启用"}
          </Badge>
        }
      >
        <div className="flex min-w-0 items-start gap-3 rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
          <span className="shrink-0 pt-0.5 text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-text-muted)]">
            URL
          </span>
          <code className="min-w-0 break-all text-sm text-[var(--color-text-primary)]">
            {publicAPIURL}
          </code>
        </div>
      </DetailPanel>

      {isDualInstanceRuntimeScript ? (
        <DetailPanel title="启动配置">
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
              <CompactMetaField label="Method" value="POST" />
              <CompactMetaField label="Path" value={<code>/api/runtime/session</code>} />
              <CompactMetaField
                label="Base URL"
                value={<span className="break-all">{launchBaseUrl}</span>}
              />
              <CompactMetaField
                label="调用次数"
                value={`${dualRuntimePreview.requests.length || 0} 次`}
              />
            </div>

            {dualRuntimePreview.error ? (
              <div className="rounded-xl border border-[var(--color-warning)] bg-[var(--color-bg-surface)] px-3 py-3 text-sm text-[var(--color-text-secondary)]">
                当前启动配置 JSON 无法解析，暂时不能展开接口示例：{" "}
                <code>{dualRuntimePreview.error}</code>
              </div>
            ) : null}

            {dualRuntimePreview.requests.length > 0 ? (
              <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="text-sm font-medium text-[var(--color-text-primary)]">
                    已生成 {dualRuntimePreview.requests.length} 次接口调用示例
                  </div>
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    onClick={onToggleDualRuntimeRequests}
                  >
                    {showDualRuntimeRequests ? "收起请求示例" : "展开请求示例"}
                  </Button>
                </div>
                {showDualRuntimeRequests ? (
                  <div className="mt-3 grid grid-cols-1 gap-4 xl:grid-cols-2">
                    {dualRuntimePreview.requests.map((request, index) => (
                      <div
                        key={`${request.code}-${index}`}
                        className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-3 py-3"
                      >
                        <div className="mb-2 text-sm font-medium text-[var(--color-text-primary)]">
                          第 {index + 1} 次接口调用 · <code>{request.code}</code>
                        </div>
                        <pre className="overflow-x-auto rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] p-3 text-xs leading-6 text-[var(--color-text-secondary)]">
                          <code>
                            {buildRuntimeSessionHttpPreview(
                              launchBaseUrl,
                              apiAuthHeader,
                              request.payload,
                            )}
                          </code>
                        </pre>
                      </div>
                    ))}
                  </div>
                ) : null}
              </div>
            ) : null}

            <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="text-sm font-medium text-[var(--color-text-primary)]">
                  OpenClaw 指令模板
                </div>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={onCopyOpenClawCommand}
                >
                  <Copy className="h-4 w-4" />
                  复制指令
                </Button>
              </div>
              <pre className="mt-3 overflow-x-auto rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] p-3 text-xs leading-6 text-[var(--color-text-secondary)]">
                <code>{openClawDualSiteCommand}</code>
              </pre>
            </div>

            <FormItem label="接口请求源配置 JSON">
              <Textarea
                rows={12}
                value={draft.paramsText}
                onChange={(event) =>
                  onUpdateDraft({ paramsText: event.target.value })
                }
                className="font-mono"
                placeholder={`{
  "browsers": [
    { "code": "BUYER_001", "skipDefaultStartUrls": true },
    { "code": "BUYER_002", "skipDefaultStartUrls": true }
  ],
  "timeoutMs": 45000
}`}
                disabled={busy}
              />
            </FormItem>
          </div>
        </DetailPanel>
      ) : (
        <>
          {usesManualSelector ? (
            <DetailPanel
              title="目标选择器"
              actions={
                <Button
                  type="button"
                  size="sm"
                  variant="secondary"
                  onClick={onOpenTargetConfig}
                  disabled={busy}
                >
                  <Settings className="h-4 w-4" />
                  选择实例
                </Button>
              }
            >
              <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-text-muted)]">
                  当前目标
                </div>
                <div className="mt-2 text-sm font-medium text-[var(--color-text-primary)]">
                  {formatManualSelectorSummary(draft.selectorText)}
                </div>
              </div>
            </DetailPanel>
          ) : null}

          <DetailPanel title={resolvedPublicAPI.enabled ? "接口参数" : "运行参数"}>
            <FormItem
              label={
                paramsHelp ? (
                  <FieldLabelWithHelp label="JSON" onOpen={onOpenParamsHelp} />
                ) : (
                  "JSON"
                )
              }
            >
              <Textarea
                rows={12}
                value={draft.paramsText}
                onChange={(event) =>
                  onUpdateDraft({ paramsText: event.target.value })
                }
                className="font-mono"
                placeholder='{"startUrls":["https://example.com"]}'
                disabled={busy}
              />
            </FormItem>
          </DetailPanel>
        </>
      )}

      {isLaunchApiScript ? (
        <DetailPanel title="固定模板">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            <CompactMetaField label="类型" value="接口模式" />
            <CompactMetaField label="执行方式" value="系统固定" />
            <CompactMetaField
              label="可编辑项"
              value={
                isDualInstanceRuntimeScript ? "启动配置" : "目标策略 / 运行参数"
              }
            />
          </div>
          <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3 text-sm text-[var(--color-text-secondary)]">
            固定接口模板由系统维护。
          </div>
        </DetailPanel>
      ) : (
        <DetailPanel title="脚本">
          <FormItem label="脚本内容">
            <Textarea
              rows={24}
              value={draft.scriptText}
              onChange={(event) =>
                onUpdateDraft({ scriptText: event.target.value })
              }
              className="min-h-[520px] font-mono leading-6"
              placeholder="module.exports.run = async () => {}"
            />
          </FormItem>
        </DetailPanel>
      )}
    </>
  );
}
