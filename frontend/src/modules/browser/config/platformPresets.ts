// 平台预设（配置化，不再写死在页面组件内）。
// 新增/调整平台只需改本文件。

export interface PlatformPreset {
  value: string
  label: string
  url: string
  icon: string
}

export const PLATFORM_PRESETS: PlatformPreset[] = [
  { value: 'facebook', label: 'Facebook', url: 'https://www.facebook.com/', icon: '📘' },
  { value: 'chatgpt', label: 'ChatGPT', url: 'https://www.chatgpt.com/', icon: '🤖' },
  { value: 'google', label: 'Google', url: 'https://gemini.google.com/app', icon: '🔍' },
  { value: 'x', label: 'X (Twitter)', url: 'https://x.com', icon: '🐦' },
  { value: 'shopify', label: 'Shopify', url: 'https://www.shopify.com/', icon: '🛍️' },
  { value: 'amazon', label: 'Amazon', url: 'https://www.amazon.com/', icon: '📦' },
  { value: 'other', label: '其他', url: '', icon: '🌐' },
]

export function getPlatformPreset(value: string): PlatformPreset | undefined {
  return PLATFORM_PRESETS.find(p => p.value === value)
}

export function platformIcon(value: string): string {
  return getPlatformPreset(value)?.icon || '🌐'
}
