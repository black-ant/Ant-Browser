import { type KeyboardEvent, type ReactNode } from "react";
import { Link, Pencil, Play } from "lucide-react";
import { Button } from "../../../shared/components";
import type { AutomationCardPresentation } from "./AutomationPage.helpers";
import { copyToClipboard } from "./AutomationPage.helpers";

function ScriptCardField({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="flex min-h-9 items-center gap-2 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-3 py-2 shadow-[var(--shadow-sm)]">
      <div className="shrink-0 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-text-muted)]">
        {label}
      </div>
      <div className="min-w-0 flex-1 text-[12px] font-medium leading-4 text-[var(--color-text-primary)]">
        {children}
      </div>
    </div>
  );
}

export function AutomationScriptSummaryCard({
  card,
  onOpen,
  onRunScript,
  onRunAPI,
  selected = false,
  onSelectedChange,
}: {
  card: AutomationCardPresentation;
  onOpen?: () => void;
  onRunScript?: () => void;
  onRunAPI?: () => void;
  selected?: boolean;
  onSelectedChange?: (selected: boolean) => void;
}) {
  const interactive = typeof onOpen === "function";
  const selectable = typeof onSelectedChange === "function";
  const isInterfaceModeCard = card.scriptType === "launch-api";
  const actionButtonClassName =
    "!h-7 !w-full min-w-0 justify-center whitespace-nowrap !rounded-md !border !border-black !bg-black !px-2 !text-xs !font-medium !leading-none !text-white !shadow-none hover:!border-[#1f1f1f] hover:!bg-[#1f1f1f] focus-visible:!ring-black disabled:!border-[#6b7280] disabled:!bg-[#6b7280] disabled:!text-white";
  const headerCopyButtonClassName =
    "!h-7 !w-full min-w-0 justify-center whitespace-nowrap !rounded-md !border !border-black !bg-white !px-2 !text-xs !font-medium !leading-none !text-black !shadow-none hover:!border-black hover:!bg-[#f3f4f6] hover:!text-black focus-visible:!ring-black disabled:!border-[#6b7280] disabled:!bg-white disabled:!text-[#6b7280]";
  const scriptButtonClassName = actionButtonClassName;
  const apiSetupButtonClassName = actionButtonClassName;
  const interfaceExecuteButtonClassName = actionButtonClassName;
  const editButtonClassName = actionButtonClassName;

  const cardClickable = selectable || interactive;

  const handleCardClick = () => {
    if (selectable) {
      onSelectedChange?.(!selected);
      return;
    }
    onOpen?.();
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!cardClickable) {
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      handleCardClick();
    }
  };

  return (
    <div
      role={selectable ? "checkbox" : interactive ? "button" : undefined}
      aria-checked={selectable ? selected : undefined}
      tabIndex={cardClickable ? 0 : undefined}
      onClick={cardClickable ? handleCardClick : undefined}
      onKeyDown={cardClickable ? handleKeyDown : undefined}
      className={`group relative flex h-full flex-col rounded-[22px] border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] pb-3 pl-7 pr-3.5 pt-3 text-left shadow-[var(--shadow-xs)] transition-all duration-200 ${
        cardClickable
          ? "cursor-pointer hover:border-[var(--color-border-strong)] hover:shadow-[var(--shadow-md)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2"
          : ""
      } ${selected ? "border-[var(--color-border-strong)] ring-2 ring-[var(--color-border-strong)] ring-offset-1" : ""}`}
    >
      <div
        aria-hidden="true"
        className={`absolute bottom-3 left-3 top-3 w-1 rounded-full ${card.railClassName}`}
      />

      <div className="relative min-w-0 pr-16">
        {selectable ? (
          <input
            type="checkbox"
            checked={selected}
            onClick={(event) => event.stopPropagation()}
            onChange={(event) => onSelectedChange?.(event.currentTarget.checked)}
            className="absolute left-0 top-0.5 h-4 w-4 rounded border-[var(--color-border-strong)] text-[var(--color-accent)] focus:ring-[var(--color-accent)]"
            aria-label={`选择 ${card.title}`}
          />
        ) : null}
        <div className={`min-w-0 text-[16px] font-semibold leading-5 text-[var(--color-text-primary)] ${selectable ? "pl-6" : ""}`}>
          {card.title}
        </div>
        <div className="absolute right-0 top-0 rounded-full border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-2 py-0.5 text-[10px] font-medium leading-4 text-[var(--color-text-secondary)]">
          {card.versionLabel}
        </div>
      </div>

      <div
        className="mt-2 grid w-full max-w-[450px] grid-cols-2 justify-start gap-1.5 overflow-hidden sm:grid-cols-5"
      >
        <Button
          type="button"
          size="sm"
          className={headerCopyButtonClassName}
          style={{ border: "1px solid #000000", backgroundColor: "#ffffff", color: "#000000" }}
          onClick={(event) => {
            event.stopPropagation();
            void copyToClipboard(
              card.primaryActionText,
              card.primaryActionSuccessMessage,
            );
          }}
        >
          {card.primaryActionLabel}
        </Button>
        <Button
          type="button"
          size="sm"
          className={headerCopyButtonClassName}
          style={{ border: "1px solid #000000", backgroundColor: "#ffffff", color: "#000000" }}
          onClick={(event) => {
            event.stopPropagation();
            void copyToClipboard(
              card.secondaryActionText,
              card.secondaryActionSuccessMessage,
            );
          }}
        >
          {card.secondaryActionLabel}
        </Button>
        {isInterfaceModeCard ? (
          typeof onRunAPI === "function" || typeof onRunScript === "function" ? (
            <Button
              type="button"
              size="sm"
              className={interfaceExecuteButtonClassName}
              onClick={(event) => {
                event.stopPropagation();
                if (typeof onRunAPI === "function") {
                  onRunAPI();
                  return;
                }
                onRunScript?.();
              }}
              aria-label={`执行 ${card.title}`}
              title="执行"
            >
              <Play className="h-3.5 w-3.5" />
              执行
            </Button>
          ) : null
        ) : (
          <>
            {typeof onRunScript === "function" ? (
              <Button
                type="button"
                size="sm"
                className={scriptButtonClassName}
                onClick={(event) => {
                  event.stopPropagation();
                  onRunScript();
                }}
                aria-label={`执行脚本 ${card.title}`}
                title="执行脚本"
              >
                <Play className="h-3.5 w-3.5" />
                脚本
              </Button>
            ) : null}
            {typeof onRunAPI === "function" ? (
              <Button
                type="button"
                size="sm"
                variant="secondary"
                className={
                  card.publicAPIEnabled
                    ? scriptButtonClassName
                    : apiSetupButtonClassName
                }
                onClick={(event) => {
                  event.stopPropagation();
                  onRunAPI();
                }}
                aria-label={`${card.publicAPIEnabled ? "执行接口" : "配置接口"} ${card.title}`}
                title={card.publicAPIEnabled ? "执行接口" : "配置接口"}
              >
                {card.publicAPIEnabled ? (
                  <Play className="h-3.5 w-3.5" />
                ) : (
                  <Link className="h-3.5 w-3.5" />
                )}
                {card.publicAPIEnabled ? "接口" : "配置"}
              </Button>
            ) : null}
          </>
        )}
        {interactive ? (
          <Button
            type="button"
            size="sm"
            className={editButtonClassName}
            onClick={(event) => {
              event.stopPropagation();
              onOpen?.();
            }}
            aria-label={`编辑 ${card.title}`}
            title="编辑"
          >
            <Pencil className="h-3.5 w-3.5" />
            编辑
          </Button>
        ) : null}
      </div>

      <div className="mt-3 grid grid-cols-1 gap-2 md:grid-cols-[136px_minmax(0,1fr)]">
        <ScriptCardField label="类型">
          <span className="inline-flex items-center gap-1.5">
            <span className={`h-1.5 w-1.5 rounded-full ${card.modeToneClass}`} />
            <span>{card.modeLabel}</span>
          </span>
        </ScriptCardField>
        <ScriptCardField label="Code 码">
          <code className="block truncate whitespace-nowrap font-mono text-[10.5px] leading-4 tracking-[0.04em] text-[var(--color-text-primary)]">
            {card.codeDisplay}
          </code>
        </ScriptCardField>
      </div>
    </div>
  );
}
