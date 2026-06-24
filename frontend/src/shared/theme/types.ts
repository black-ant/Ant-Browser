// 主题类型定义
export type ThemeType = 'dark' | 'light' | 'cream' | 'mint' | 'ocean'

export interface ThemeConfig {
  id: ThemeType
  name: string
  description: string
}

export const themeConfigs: ThemeConfig[] = [
  { id: 'dark', name: '深色主题', description: '沉稳专业的深色风格' },
  { id: 'light', name: '浅色主题', description: '简洁明亮的浅色风格' },
  { id: 'cream', name: '奶油主题', description: '温暖柔和的奶油色调' },
  { id: 'mint', name: '薄荷主题', description: '清新自然的浅绿风格' },
  { id: 'ocean', name: '海洋主题', description: '深邃宁静的蓝色风格' },
]

export const DEFAULT_THEME: ThemeType = 'dark'
