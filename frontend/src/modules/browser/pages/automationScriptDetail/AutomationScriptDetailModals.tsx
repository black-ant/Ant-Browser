import { Button, Modal } from "../../../../shared/components";
import type { ScriptParamsHelpContent } from "./paramsHelp";

interface AutomationScriptDetailModalsProps {
  paramsHelp: ScriptParamsHelpContent | null;
  paramsHelpOpen: boolean;
  onCloseParamsHelp: () => void;
}

export function AutomationScriptDetailModals({
  paramsHelp,
  paramsHelpOpen,
  onCloseParamsHelp,
}: AutomationScriptDetailModalsProps) {
  return (
    <>
      {paramsHelp ? (
        <Modal
          open={paramsHelpOpen}
          onClose={onCloseParamsHelp}
          title={paramsHelp.title}
          width="960px"
          footer={
            <Button variant="secondary" onClick={onCloseParamsHelp}>
              关闭
            </Button>
          }
        >
          <div className="space-y-4">
            <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-4 py-3 text-sm text-[var(--color-text-secondary)]">
              {paramsHelp.summary}
            </div>

            <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
              {paramsHelp.docs.map((doc) => (
                <div
                  key={doc.name}
                  className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-4 py-3"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <code className="text-sm font-semibold text-[var(--color-text-primary)]">
                      {doc.name}
                    </code>
                    {doc.defaultValue ? (
                      <span className="text-xs text-[var(--color-text-muted)]">
                        默认 {doc.defaultValue}
                      </span>
                    ) : null}
                  </div>
                  <div className="mt-2 text-sm leading-6 text-[var(--color-text-secondary)]">
                    {doc.description}
                  </div>
                  {doc.note ? (
                    <div className="mt-2 text-xs leading-5 text-[var(--color-text-muted)]">
                      {doc.note}
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
          </div>
        </Modal>
      ) : null}

    </>
  );
}
