import { Copy } from 'lucide-react'
import { Badge, Button, Card } from '../../../../../shared/components'
import type { AutomationDemoSession } from '../../../demoSession'
import { LaunchDocsCodeBlock } from '../LaunchDocsCodeBlock'

interface LaunchResponsePanelProps {
  demoSession: AutomationDemoSession
  demoResponseText: string
  onCopyResponse: () => void
}

export function LaunchResponsePanel({
  demoSession,
  demoResponseText,
  onCopyResponse,
}: LaunchResponsePanelProps) {
  const displayResponseText = demoResponseText || '{\n  "hint": "先运行一次右侧案例"\n}'

  return (
    <Card
      title="最近响应"
      className="bg-[var(--color-bg-elevated)] shadow-[var(--shadow-sm)]"
      actions={(
        <Button
          size="sm"
          variant="secondary"
          onClick={onCopyResponse}
          disabled={!demoResponseText}
        >
          <Copy className="w-3.5 h-3.5" />
          复制 JSON
        </Button>
      )}
    >
      <div className="space-y-2">
        <div className="flex items-center justify-between gap-2">
          <p className="text-sm font-medium text-[var(--color-text-primary)]">
            最近一次联动结果
          </p>
          {demoSession.lastResult && (
            <Badge variant={demoSession.lastResult.ok ? 'success' : 'error'} size="sm" dot>
              HTTP {demoSession.lastResult.status || '-'}
            </Badge>
          )}
        </div>

        <LaunchDocsCodeBlock
          language="json"
          code={displayResponseText}
          maxHeightClassName="max-h-[320px] overflow-y-auto"
          showCopyButton={false}
          className="my-0"
        />
      </div>
    </Card>
  )
}
