import { 
  BarChartExample, 
  LineChartExample, 
  PieChartExample, 
  AreaChartExample, 
  ComposedChartExample 
} from './components';
import { Card } from '../../shared/components';

export function ChartsPage() {
  return (
    <div className="space-y-4">
      <Card padding="none" className="shadow-[var(--shadow-sm)]">
        <div className="px-4 py-3">
          <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">图表案例展示</h1>
        </div>
      </Card>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* 柱状图 */}
        <div className="bg-[var(--color-bg-surface)] p-3 rounded-lg border border-[var(--color-border-default)] shadow-sm">
          <h2 className="text-lg font-medium mb-3 text-[var(--color-text-primary)]">柱状图示例</h2>
          <div className="h-[300px]">
            <BarChartExample />
          </div>
        </div>

        {/* 折线图 */}
        <div className="bg-[var(--color-bg-surface)] p-3 rounded-lg border border-[var(--color-border-default)] shadow-sm">
          <h2 className="text-lg font-medium mb-3 text-[var(--color-text-primary)]">折线图示例</h2>
          <div className="h-[300px]">
            <LineChartExample />
          </div>
        </div>

        {/* 饼图 */}
        <div className="bg-[var(--color-bg-surface)] p-3 rounded-lg border border-[var(--color-border-default)] shadow-sm">
          <h2 className="text-lg font-medium mb-3 text-[var(--color-text-primary)]">饼图示例</h2>
          <div className="h-[300px]">
            <PieChartExample />
          </div>
        </div>

        {/* 面积图 */}
        <div className="bg-[var(--color-bg-surface)] p-3 rounded-lg border border-[var(--color-border-default)] shadow-sm">
          <h2 className="text-lg font-medium mb-3 text-[var(--color-text-primary)]">面积图示例</h2>
          <div className="h-[300px]">
            <AreaChartExample />
          </div>
        </div>

        {/* 组合图表 */}
        <div className="bg-[var(--color-bg-surface)] p-3 rounded-lg border border-[var(--color-border-default)] shadow-sm col-span-1 md:col-span-2">
          <h2 className="text-lg font-medium mb-3 text-[var(--color-text-primary)]">复合图表示例</h2>
          <div className="h-[400px]">
            <ComposedChartExample />
          </div>
        </div>
      </div>
    </div>
  );
}
