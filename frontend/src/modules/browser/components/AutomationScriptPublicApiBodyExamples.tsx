import { Copy } from "lucide-react";
import { Button, Textarea } from "../../../shared/components";
import { copyText } from "./AutomationScriptPublicApiModal.helpers";
import type { AutomationScriptPublicAPIConfig } from "../automationScripts";

interface AutomationScriptPublicApiBodyExamplesProps {
  busy: boolean;
  resolvedConfig: AutomationScriptPublicAPIConfig;
  requestExampleFallback: string;
  responseExampleFallback: string;
  requestBodyError: string;
  responseBodyError: string;
  updateConfig: (patch: Partial<AutomationScriptPublicAPIConfig>) => void;
}

export function AutomationScriptPublicApiBodyExamples({
  busy,
  resolvedConfig,
  requestExampleFallback,
  responseExampleFallback,
  requestBodyError,
  responseBodyError,
  updateConfig,
}: AutomationScriptPublicApiBodyExamplesProps) {
  return (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="text-sm font-medium text-[var(--color-text-primary)]">
                请求 Body
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() =>
                    void copyText(
                      resolvedConfig.requestBodyText || requestExampleFallback,
                      "Body 已复制",
                    )
                  }
                >
                  <Copy className="h-4 w-4" />
                  复制
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => updateConfig({ requestBodyText: "" })}
                  disabled={busy}
                >
                  默认
                </Button>
              </div>
            </div>
            <Textarea
              rows={10}
              value={resolvedConfig.requestBodyText}
              onChange={(event) =>
                updateConfig({ requestBodyText: event.target.value })
              }
              className="mt-3 font-mono"
              placeholder={requestExampleFallback}
              disabled={busy}
            />
            {requestBodyError ? (
              <p className="mt-2 text-xs text-[var(--color-error)]">
                {requestBodyError}
              </p>
            ) : (
              <p className="mt-2 text-xs text-[var(--color-text-muted)]">
                只放外部调用要传的字段；页面按钮、输入框、图片定位由脚本内部处理。
              </p>
            )}
          </div>

          <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="text-sm font-medium text-[var(--color-text-primary)]">
                响应示例
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() =>
                    void copyText(
                      resolvedConfig.responseBodyText || responseExampleFallback,
                      "Response 已复制",
                    )
                  }
                >
                  <Copy className="h-4 w-4" />
                  复制
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => updateConfig({ responseBodyText: "" })}
                  disabled={busy}
                >
                  默认
                </Button>
              </div>
            </div>
            <Textarea
              rows={13}
              value={resolvedConfig.responseBodyText}
              onChange={(event) =>
                updateConfig({ responseBodyText: event.target.value })
              }
              className="mt-3 font-mono"
              placeholder={responseExampleFallback}
              disabled={busy}
            />
            {responseBodyError ? (
              <p className="mt-2 text-xs text-[var(--color-error)]">
                {responseBodyError}
              </p>
            ) : null}
          </div>
        </div>
  );
}
