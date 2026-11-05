import { AutomationRuntimeSnapshot } from '../../../components/AutomationRuntimeSnapshot'

export function LaunchRuntimePanel() {
  return (
    <AutomationRuntimeSnapshot
      title="运行时快照"
      className="bg-[var(--color-bg-elevated)] shadow-[var(--shadow-sm)]"
      showSettingsAction={false}
    />
  )
}
