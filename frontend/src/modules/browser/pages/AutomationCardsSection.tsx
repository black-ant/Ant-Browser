import { PlusSquare, Upload } from "lucide-react";
import { Button } from "../../../shared/components";
import { resolveAutomationScriptPublicAPIConfig, type AutomationScriptRecord } from "../automationScripts";
import { AutomationScriptSummaryCard } from "./AutomationScriptSummaryCard";
import type { AutomationCardPresentation } from "./AutomationPage.helpers";

interface AutomationCardsSectionProps {
  loading: boolean;
  cards: AutomationCardPresentation[];
  scripts: AutomationScriptRecord[];
  selectedScriptIds: string[];
  onCreate: () => void;
  onImport: () => void;
  onToggleScriptSelection: (scriptId: string, selected: boolean) => void;
  onOpenScript: (scriptId: string) => void;
  onRunAutomationScript: (script: AutomationScriptRecord) => void;
  onOpenPublicApi: (script: AutomationScriptRecord, options?: { focusTest?: boolean }) => void;
}

export function AutomationCardsSection({
  loading,
  cards,
  scripts,
  selectedScriptIds,
  onCreate,
  onImport,
  onToggleScriptSelection,
  onOpenScript,
  onRunAutomationScript,
  onOpenPublicApi,
}: AutomationCardsSectionProps) {
  const scriptMap = new Map(scripts.map((script) => [script.id, script]));
  const selectedScriptIdSet = new Set(selectedScriptIds);

  return (
      <section className="rounded-[28px] border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] p-3 shadow-[var(--shadow-sm)] md:p-4">
        {loading ? (
          <div className="rounded-2xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-12 text-center text-sm text-[var(--color-text-muted)]">
            正在加载脚本列表...
          </div>
        ) : cards.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-14 text-center">
            <div className="text-base font-medium text-[var(--color-text-primary)]">
              还没有脚本
            </div>
            <div className="mt-2 text-sm text-[var(--color-text-muted)]">
              先新建一套脚本，或者导入已有脚本。
            </div>
            <div className="mt-5 flex justify-center gap-2">
              <Button size="sm" onClick={() => onCreate()}>
                <PlusSquare className="h-4 w-4" />
                新建
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => onImport()}
              >
                <Upload className="h-4 w-4" />
                导入
              </Button>
            </div>
          </div>
        ) : (
          <div
            className="grid items-stretch gap-3"
            style={{
              gridTemplateColumns:
                "repeat(auto-fit, minmax(min(100%, max(458px, calc((100% - 24px) / 3))), 1fr))",
            }}
          >
            {cards.map((card) => {
              const scriptId = card.scriptId;
              const onOpen = scriptId ? () => onOpenScript(scriptId) : undefined;
              const script = scriptId ? scriptMap.get(scriptId) : undefined;
              const publicAPIEnabled = script
                ? resolveAutomationScriptPublicAPIConfig(script).enabled
                : false;
              const runScriptAction = script && script.type !== "launch-api"
                ? () => onRunAutomationScript(script)
                : undefined;
              const onRunAPI = script
                ? () =>
                    onOpenPublicApi(script, {
                      focusTest: publicAPIEnabled,
                    })
                : undefined;

              return (
                <div key={card.key} className="min-w-0">
                  <AutomationScriptSummaryCard
                    card={card}
                    onOpen={onOpen}
                    onRunScript={runScriptAction}
                    onRunAPI={onRunAPI}
                    selected={scriptId ? selectedScriptIdSet.has(scriptId) : false}
                    onSelectedChange={
                      scriptId
                        ? (selected) => onToggleScriptSelection(scriptId, selected)
                        : undefined
                    }
                  />
                </div>
              );
            })}
          </div>
        )}
      </section>
  );

}
