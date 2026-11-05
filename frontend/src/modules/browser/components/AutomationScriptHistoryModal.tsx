import {
  Fragment,
  useEffect,
  useState,
  type KeyboardEvent,
} from "react";
import { ChevronDown, ChevronRight, FileText, RefreshCw } from "lucide-react";
import {
  Badge,
  Button,
  Modal,
  toast,
} from "../../../shared/components";
import { fetchAutomationScriptRuns } from "../automationScriptApi";
import type { AutomationScriptRunRecord } from "../automationScripts";

interface AutomationScriptHistoryModalProps {
  open: boolean;
  onClose: () => void;
}

function formatDateTime(value?: string): string {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString("zh-CN", { hour12: false });
}

function formatDuration(durationMs?: number): string {
  if (!durationMs || durationMs <= 0) {
    return "-";
  }
  if (durationMs < 1000) {
    return `${durationMs} ms`;
  }
  return `${(durationMs / 1000).toFixed(2)} s`;
}

function getRunStatusLabel(status: AutomationScriptRunRecord["status"]): string {
  switch (status) {
    case "success":
      return "成功";
    case "running":
      return "运行中";
    default:
      return "失败";
  }
}

function getRunStatusBadgeVariant(
  status: AutomationScriptRunRecord["status"],
): "success" | "info" | "error" {
  switch (status) {
    case "success":
      return "success";
    case "running":
      return "info";
    default:
      return "error";
  }
}

function normalizeRuns(items: AutomationScriptRunRecord[]): AutomationScriptRunRecord[] {
  return [...items].sort(
    (left, right) =>
      new Date(right.startedAt).getTime() - new Date(left.startedAt).getTime(),
  );
}

function HistoryStat({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] px-4 py-3">
      <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-text-muted)]">
        {label}
      </div>
      <div className="mt-2 text-sm font-semibold text-[var(--color-text-primary)]">
        {value}
      </div>
    </div>
  );
}

function HistoryDetailField({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] px-4 py-3">
      <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-text-muted)]">
        {label}
      </div>
      <div className="mt-2 break-all text-sm font-medium text-[var(--color-text-primary)]">
        {value}
      </div>
    </div>
  );
}

export function AutomationScriptHistoryModal({
  open,
  onClose,
}: AutomationScriptHistoryModalProps) {
  const [runs, setRuns] = useState<AutomationScriptRunRecord[]>([]);
  const [expandedRunId, setExpandedRunId] = useState("");
  const [selectedLogRunId, setSelectedLogRunId] = useState("");
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  useEffect(() => {
    if (!open) {
      setRuns([]);
      setExpandedRunId("");
      setSelectedLogRunId("");
      setLoading(false);
      return;
    }

    let active = true;
    setLoading(true);

    void fetchAutomationScriptRuns(200)
      .then((items) => {
        if (!active) {
          return;
        }
        setRuns(normalizeRuns(items));
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }
        setRuns([]);
        const message =
          error instanceof Error ? error.message : "调用记录加载失败";
        toast.error(message);
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });

    return () => {
      active = false;
    };
  }, [open]);

  useEffect(() => {
    if (!expandedRunId) {
      return;
    }

    if (!runs.some((item) => item.id === expandedRunId)) {
      setExpandedRunId("");
    }
  }, [expandedRunId, runs]);

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      const items = await fetchAutomationScriptRuns(200);
      setRuns(normalizeRuns(items));
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : "调用记录刷新失败";
      toast.error(message);
    } finally {
      setRefreshing(false);
    }
  };

  const toggleExpandedRun = (runId: string) => {
    setExpandedRunId((current) => (current === runId ? "" : runId));
  };

  const handleRowKeyDown = (
    event: KeyboardEvent<HTMLTableRowElement>,
    runId: string,
  ) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      toggleExpandedRun(runId);
    }
  };

  const latestRun = runs[0] || null;
  const selectedLogRun =
    runs.find((item) => item.id === selectedLogRunId) || null;
  const successCount = runs.filter((item) => item.status === "success").length;
  const failedCount = runs.filter((item) => item.status === "failed").length;
  const scriptCount = new Set(
    runs.map((item) => String(item.scriptId || "").trim()).filter(Boolean),
  ).size;

  return (
    <>
    <Modal
      open={open}
      onClose={onClose}
      title="脚本调用记录"
      width="980px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            关闭
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="flex items-center justify-end">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => void handleRefresh()}
            loading={refreshing}
            disabled={loading}
          >
            <RefreshCw className="h-4 w-4" />
            刷新
          </Button>
        </div>

        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
          <HistoryStat label="最近调用" value={formatDateTime(latestRun?.startedAt)} />
          <HistoryStat
            label="成功 / 失败"
            value={`${successCount} / ${failedCount}`}
          />
          <HistoryStat label="记录 / 脚本" value={`${runs.length} / ${scriptCount}`} />
        </div>

        {loading ? (
          <div className="flex min-h-[280px] items-center justify-center rounded-2xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-surface)]">
            <div className="flex flex-col items-center gap-3">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-[var(--color-border-default)] border-t-[var(--color-accent)]" />
              <div className="text-sm text-[var(--color-text-muted)]">
                正在加载调用记录...
              </div>
            </div>
          </div>
        ) : runs.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-14 text-center">
            <div className="text-base font-medium text-[var(--color-text-primary)]">
              还没有调用记录
            </div>
            <div className="mt-2 text-sm text-[var(--color-text-muted)]">
              脚本执行过之后，这里会显示调用时间、脚本名称、状态和结果摘要。
            </div>
          </div>
        ) : (
          <div className="overflow-hidden rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-[var(--shadow-sm)]">
            <div className="max-h-[58vh] overflow-auto">
              <table className="w-full min-w-[1120px]">
                <thead className="sticky top-0 z-10 bg-[var(--color-bg-muted)]">
                  <tr>
                    <th className="w-14 whitespace-nowrap px-3 py-3 text-left text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-text-muted)]">
                      展开
                    </th>
                    <th className="w-[170px] whitespace-nowrap px-4 py-3 text-left text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-text-muted)]">
                      调用时间
                    </th>
                    <th className="w-[230px] whitespace-nowrap px-4 py-3 text-left text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-text-muted)]">
                      脚本
                    </th>
                    <th className="w-[90px] whitespace-nowrap px-4 py-3 text-left text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-text-muted)]">
                      状态
                    </th>
                    <th className="w-[150px] whitespace-nowrap px-4 py-3 text-left text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-text-muted)]">
                      类型
                    </th>
                    <th className="w-[110px] whitespace-nowrap px-4 py-3 text-left text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-text-muted)]">
                      耗时
                    </th>
                    <th className="whitespace-nowrap px-4 py-3 text-left text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-text-muted)]">
                      摘要
                    </th>
                    <th className="w-[120px] whitespace-nowrap px-4 py-3 text-left text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-text-muted)]">
                      操作
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-border-muted)]">
                  {runs.map((run) => {
                    const expanded = expandedRunId === run.id;
                    return (
                      <Fragment key={run.id}>
                        <tr
                          role="button"
                          tabIndex={0}
                          aria-expanded={expanded}
                          onClick={() => toggleExpandedRun(run.id)}
                          onKeyDown={(event) => handleRowKeyDown(event, run.id)}
                          className="cursor-pointer transition-colors duration-150 hover:bg-[var(--color-bg-muted)]/55 focus:outline-none focus-visible:bg-[var(--color-accent-muted)]/40"
                        >
                          <td className="whitespace-nowrap px-3 py-4 align-top text-[var(--color-text-muted)]">
                            <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)]">
                              {expanded ? (
                                <ChevronDown className="h-4 w-4" />
                              ) : (
                                <ChevronRight className="h-4 w-4" />
                              )}
                            </span>
                          </td>
                          <td className="whitespace-nowrap px-4 py-4 align-top text-sm text-[var(--color-text-primary)]">
                            <div className="whitespace-nowrap font-medium">
                              {formatDateTime(run.startedAt)}
                            </div>
                          </td>
                          <td className="whitespace-nowrap px-4 py-4 align-top text-sm text-[var(--color-text-primary)]">
                            <div
                              className="max-w-[180px] truncate font-medium"
                              title={run.scriptName || "未命名脚本"}
                            >
                              {run.scriptName || "未命名脚本"}
                            </div>
                          </td>
                          <td className="whitespace-nowrap px-4 py-4 align-top text-sm">
                            <span className="inline-flex whitespace-nowrap">
                              <Badge
                                variant={getRunStatusBadgeVariant(run.status)}
                                size="sm"
                                dot
                              >
                                {getRunStatusLabel(run.status)}
                              </Badge>
                            </span>
                          </td>
                          <td className="whitespace-nowrap px-4 py-4 align-top text-sm text-[var(--color-text-secondary)]">
                            <div
                              className="max-w-[140px] truncate"
                              title={run.scriptType || "-"}
                            >
                              {run.scriptType || "-"}
                            </div>
                          </td>
                          <td className="whitespace-nowrap px-4 py-4 align-top text-sm text-[var(--color-text-primary)]">
                            {formatDuration(run.durationMs)}
                          </td>
                          <td className="whitespace-nowrap px-4 py-4 align-top text-sm text-[var(--color-text-secondary)]">
                            <div
                              className="max-w-[320px] truncate"
                              title={run.summary || "未返回摘要"}
                            >
                              {run.summary || "未返回摘要"}
                            </div>
                          </td>
                          <td className="whitespace-nowrap px-4 py-4 align-top text-sm">
                            <Button
                              size="sm"
                              variant="secondary"
                              disabled={!run.logText}
                              onClick={(event) => {
                                event.stopPropagation();
                                setSelectedLogRunId(run.id);
                              }}
                            >
                              <FileText className="h-3.5 w-3.5" />
                              查看日志
                            </Button>
                          </td>
                        </tr>
                        {expanded ? (
                          <tr className="bg-[var(--color-bg-muted)]/35">
                            <td colSpan={8} className="px-4 pb-4 pt-1">
                              <div className="rounded-2xl border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] p-4">
                                <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
                                  <HistoryDetailField
                                    label="调用时间"
                                    value={formatDateTime(run.startedAt)}
                                  />
                                  <HistoryDetailField
                                    label="结束时间"
                                    value={formatDateTime(run.finishedAt)}
                                  />
                                  <HistoryDetailField
                                    label="执行耗时"
                                    value={formatDuration(run.durationMs)}
                                  />
                                  <HistoryDetailField
                                    label="记录 ID"
                                    value={run.id || "-"}
                                  />
                                </div>

                                <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2">
                                  <HistoryDetailField
                                    label="脚本名称"
                                    value={run.scriptName || "未命名脚本"}
                                  />
                                  <HistoryDetailField
                                    label="脚本类型"
                                    value={run.scriptType || "-"}
                                  />
                                </div>

                                <div className="mt-3 rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] px-4 py-3">
                                  <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-text-muted)]">
                                    详细摘要
                                  </div>
                                  <div className="mt-2 whitespace-pre-wrap break-all text-sm leading-6 text-[var(--color-text-secondary)]">
                                    {run.summary || "未返回摘要"}
                                  </div>
                                </div>

                                {run.error ? (
                                  <div className="mt-3 rounded-xl border border-[var(--color-error)]/20 bg-[var(--color-error)]/8 px-4 py-3">
                                    <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-error)]">
                                      错误信息
                                    </div>
                                    <pre className="mt-2 whitespace-pre-wrap break-all font-mono text-xs leading-6 text-[var(--color-error)]">
                                      {run.error}
                                    </pre>
                                  </div>
                                ) : null}

                                {run.resultText ? (
                                  <div className="mt-3 rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] px-4 py-3">
                                    <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-text-muted)]">
                                      返回内容
                                    </div>
                                    <pre className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap break-all font-mono text-xs leading-6 text-[var(--color-text-secondary)]">
                                      {run.resultText}
                                    </pre>
                                  </div>
                                ) : null}

                                {!run.error && !run.resultText && !run.logText ? (
                                  <div className="mt-3 rounded-xl border border-dashed border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] px-4 py-3 text-sm text-[var(--color-text-muted)]">
                                    这条记录没有更多详情。
                                  </div>
                                ) : null}
                              </div>
                            </td>
                          </tr>
                        ) : null}
                      </Fragment>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </Modal>
    <Modal
      open={Boolean(selectedLogRun)}
      onClose={() => setSelectedLogRunId("")}
      title="执行日志"
      width="760px"
      footer={
        <Button variant="secondary" onClick={() => setSelectedLogRunId("")}>
          关闭
        </Button>
      }
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] px-4 py-3 text-sm">
          <span className="font-medium text-[var(--color-text-primary)]">
            {selectedLogRun?.scriptName || "未命名脚本"}
          </span>
          <span className="text-xs text-[var(--color-text-muted)]">
            {formatDateTime(selectedLogRun?.startedAt)}
          </span>
        </div>
        <pre className="max-h-[56vh] overflow-auto rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] px-4 py-3 font-mono text-xs leading-6 text-[var(--color-text-secondary)] whitespace-pre-wrap break-all">
          {selectedLogRun?.logText || "暂无执行日志"}
        </pre>
      </div>
    </Modal>
    </>
  );
}
