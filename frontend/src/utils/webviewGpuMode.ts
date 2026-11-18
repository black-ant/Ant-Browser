let applied = false

/**
 * 把后端解析出来的 WebView 硬件加速状态落到 `<html data-webview-gpu>`。
 *
 * - `gpu`：硬件加速，保留磨砂动效；
 * - `software`：软件渲染，index.css 会关掉逐帧 backdrop-filter 与遮罩层动画，
 *   避免 CPU 每帧重算全屏模糊（app.webview_gpu: false 时的主要掉帧来源）。
 *
 * 平台差异：`applicable === false`（macOS，WKWebView 由系统托管 GPU）时该开关不生效，
 * 按 `gpu` 处理，避免在 macOS 上误降级视觉。
 *
 * 读取失败时按默认值 `gpu` 处理，不阻塞启动流程。
 */
export async function applyWebviewGpuMode() {
  if (applied) return
  applied = true

  try {
    const bindings: any = await import('../wailsjs/go/main/App')
    const status = await bindings.GetWebviewGPUStatus?.()
    const enabled = status?.applicable === false ? true : status?.enabled !== false
    document.documentElement.dataset.webviewGpu = enabled ? 'gpu' : 'software'
  } catch {
    document.documentElement.dataset.webviewGpu = 'gpu'
  }
}
