// 有界并发执行：以固定并发数依次处理 items（worker-pool）。
// 从 BrowserListPage 提取为共享工具，供批量启动/停止/测试等复用。
export const BATCH_OP_CONCURRENCY = 5

export async function runWithConcurrency<T>(
  items: T[],
  limit: number,
  worker: (item: T) => Promise<void>,
): Promise<void> {
  let index = 0
  const size = Math.max(1, Math.min(limit, items.length))
  const runners = Array.from({ length: size }, async () => {
    while (index < items.length) {
      const current = items[index++]
      await worker(current)
    }
  })
  await Promise.all(runners)
}
