export const projectConfig = {
  name: 'ZwBrowser',
  shortName: 'Zw',
  // 版本号:构建期由 vite.config.ts 从 wails.json 的 info.productVersion 注入(单一事实来源),
  // 后端也从同一处编译期嵌入。不要在此写死 —— 写死会与实际发版版本脱节。
  version: __APP_VERSION__,
  description: '面向多账号隔离、代理绑定和本地环境管理的桌面浏览器工具',
  primaryColor: 'primary',
}
