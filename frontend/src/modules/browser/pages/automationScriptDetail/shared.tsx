import type { ReactNode } from "react";
import { FormItem, Input, Select, Textarea } from "../../../../shared/components";
import {
  formatAutomationTargetIdentity,
  type AutomationScriptTargetSelector,
} from "../../automationScripts";
import type { BrowserProfile } from "../../types";
import {
  findMatchedProfileId,
  formatSelectorTerms,
  parseSelectorTerms,
  type SelectorSuggestion,
} from "./helpers";

export interface TargetSelectorEditorProps {
  selector: AutomationScriptTargetSelector;
  onChange: (patch: Partial<AutomationScriptTargetSelector>) => void;
  codeSuggestions: SelectorSuggestion[];
  profileIdSuggestions: SelectorSuggestion[];
  profileNameSuggestions: SelectorSuggestion[];
  groupOptions: Array<{ value: string; label: string }>;
  disabled?: boolean;
}

export function TargetSelectorEditor({
  selector,
  onChange,
  codeSuggestions,
  profileIdSuggestions,
  profileNameSuggestions,
  groupOptions,
  disabled = false,
}: TargetSelectorEditorProps) {
  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <FormItem label="Code">
        <Input
          value={selector.code}
          onChange={(event) =>
            onChange({ code: event.target.value.trim().toUpperCase() })
          }
          placeholder="优先推荐，例如 BUYER_001"
          list="automation-script-target-code-options"
          disabled={disabled}
        />
        {codeSuggestions.length > 0 ? (
          <datalist id="automation-script-target-code-options">
            {codeSuggestions.map((item) => (
              <option key={item.key} value={item.value}>
                {item.label}
              </option>
            ))}
          </datalist>
        ) : null}
      </FormItem>

      <FormItem label="实例 ID（高级）">
        <Input
          value={selector.profileId}
          onChange={(event) =>
            onChange({ profileId: event.target.value.trim() })
          }
          placeholder="内部主键，通常不需要手填"
          list="automation-script-target-profile-id-options"
          disabled={disabled}
        />
        {profileIdSuggestions.length > 0 ? (
          <datalist id="automation-script-target-profile-id-options">
            {profileIdSuggestions.map((item) => (
              <option key={item.key} value={item.value}>
                {item.label}
              </option>
            ))}
          </datalist>
        ) : null}
      </FormItem>

      <FormItem label="实例名称">
        <Input
          value={selector.profileName}
          onChange={(event) =>
            onChange({ profileName: event.target.value.trim() })
          }
          placeholder="精确匹配实例名称"
          list="automation-script-target-profile-name-options"
          disabled={disabled}
        />
        {profileNameSuggestions.length > 0 ? (
          <datalist id="automation-script-target-profile-name-options">
            {profileNameSuggestions.map((item) => (
              <option key={item.key} value={item.value}>
                {item.label}
              </option>
            ))}
          </datalist>
        ) : null}
      </FormItem>

      <FormItem label="分组 ID">
        <Select
          value={selector.groupId}
          onChange={(event) =>
            onChange({ groupId: event.target.value.trim() })
          }
          options={groupOptions}
          disabled={disabled}
        />
      </FormItem>

      <FormItem label="标签">
        <Textarea
          rows={3}
          value={formatSelectorTerms(selector.tags)}
          onChange={(event) =>
            onChange({ tags: parseSelectorTerms(event.target.value) })
          }
          placeholder={"sales-us\nbuyer"}
          disabled={disabled}
        />
      </FormItem>

      <FormItem label="关键字">
        <Textarea
          rows={3}
          value={formatSelectorTerms(selector.keywords)}
          onChange={(event) =>
            onChange({ keywords: parseSelectorTerms(event.target.value) })
          }
          placeholder={"buyer-001\nwarm-account"}
          disabled={disabled}
        />
      </FormItem>
    </div>
  );
}

export interface DetailPanelProps {
  title: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}

export function DetailPanel({
  title,
  actions,
  children,
  className = "",
}: DetailPanelProps) {
  return (
    <section
      className={`rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4 ${className}`}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <h2 className="text-sm font-semibold text-[var(--color-text-primary)]">
          {title}
        </h2>
        {actions ? <div className="flex items-center gap-2">{actions}</div> : null}
      </div>
      <div className="mt-4 space-y-3">{children}</div>
    </section>
  );
}

export function CompactMetaField({
  label,
  value,
}: {
  label: string;
  value: ReactNode;
}) {
  return (
    <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
      <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--color-text-muted)]">
        {label}
      </div>
      <div className="mt-1 text-sm font-medium text-[var(--color-text-primary)]">
        {value}
      </div>
    </div>
  );
}

export function StructuredInfoCell({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="px-4 py-3">
      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-text-muted)]">
        {label}
      </div>
      <div className="mt-2 text-sm font-medium text-[var(--color-text-primary)]">
        {children}
      </div>
    </div>
  );
}

export function FieldLabelWithHelp({
  label,
  onOpen,
}: {
  label: string;
  onOpen: () => void;
}) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span>{label}</span>
      <button
        type="button"
        aria-label={`${label} 字段说明`}
        className="inline-flex h-5 w-5 items-center justify-center rounded-full border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] text-[11px] font-semibold text-[var(--color-text-secondary)] transition-colors duration-150 hover:border-[var(--color-border-strong)] hover:text-[var(--color-text-primary)]"
        onClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
          onOpen();
        }}
      >
        ?
      </button>
    </span>
  );
}

export function ExactTargetSummary({
  title,
  selector,
  profiles,
}: {
  title: string;
  selector: AutomationScriptTargetSelector;
  profiles: BrowserProfile[];
}) {
  const profileId = findMatchedProfileId(selector, profiles);
  const targetLabel = formatAutomationTargetIdentity(selector, profiles, {
    fallback: "未匹配到实例",
  });

  return (
    <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
      <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-text-muted)]">
        {title}
      </div>
      <div className="mt-2 text-sm font-medium text-[var(--color-text-primary)]">
        {targetLabel}
      </div>
      {profileId ? (
        <div className="mt-1 break-all text-xs text-[var(--color-text-muted)]">
          实例 ID {profileId}
        </div>
      ) : null}
    </div>
  );
}
