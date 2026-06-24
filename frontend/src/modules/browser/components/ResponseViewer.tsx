import { useState } from 'react'
import { Button } from '../../../shared/components'

interface ResponseData {
  raw: string
  type: string
  structured: any
  preview: string
  size: number
  encoding?: string
  error?: string
  metadata?: Record<string, any>
}

interface ResponseViewerProps {
  data: ResponseData | null
  responseBody?: string
  mimeType?: string
}

type ViewMode = 'auto' | 'raw' | 'formatted' | 'preview' | 'hex'

export function ResponseViewer({ data, responseBody, mimeType }: ResponseViewerProps) {
  const [viewMode, setViewMode] = useState<ViewMode>('auto')
  const [jsonExpanded, setJsonExpanded] = useState<Set<string>>(new Set())

  if (!data && !responseBody) {
    return <div className="text-sm text-gray-400 p-4">暂无响应数据</div>
  }

  // 兼容旧数据：如果没有 parsedData，降级显示原始内容
  const content = data ? data.raw : responseBody || ''
  const dataType = data?.type || 'text'

  const renderContent = () => {
    const mode = viewMode === 'auto' ? getBestViewMode(dataType) : viewMode

    switch (mode) {
      case 'formatted':
        return renderFormatted()
      case 'preview':
        return renderPreview()
      case 'hex':
        return renderHex()
      default:
        return renderRaw()
    }
  }

  const getBestViewMode = (type: string): ViewMode => {
    switch (type) {
      case 'json':
      case 'xml':
      case 'graphql':
        return 'formatted'
      case 'image':
      case 'video':
        return 'preview'
      case 'binary':
        return 'hex'
      default:
        return 'raw'
    }
  }

  const renderFormatted = () => {
    if (dataType === 'json' || dataType === 'graphql') {
      return <JSONViewer data={data?.structured} expanded={jsonExpanded} onToggle={setJsonExpanded} />
    }

    if (dataType === 'xml' || dataType === 'html') {
      return <CodeViewer code={content} language={dataType} />
    }

    if (dataType === 'form') {
      return <FormDataViewer data={data?.structured} />
    }

    // 其他格式高亮显示
    return <CodeViewer code={content} language={detectLanguage(dataType, mimeType)} />
  }

  const renderPreview = () => {
    if (dataType === 'image') {
      const imageData = data?.structured as any
      const base64 = imageData?.base64 || ''
      const contentType = imageData?.contentType || mimeType || 'image/png'
      return (
        <div className="p-4">
          <img
            src={`data:${contentType};base64,${base64}`}
            alt="响应图片"
            className="max-w-full border border-gray-300 rounded"
            style={{ maxHeight: '600px' }}
          />
          <div className="text-xs text-gray-500 mt-2">
            格式: {imageData?.format || 'unknown'} | 大小: {formatBytes(data?.size || 0)}
          </div>
        </div>
      )
    }

    if (dataType === 'video' || dataType === 'audio') {
      return (
        <div className="p-4">
          <div className="text-sm text-gray-400">
            {dataType === 'video' ? '视频' : '音频'}文件无法直接预览，请下载查看
          </div>
          <div className="text-xs text-gray-500 mt-2">
            MIME: {mimeType} | 大小: {formatBytes(data?.size || 0)}
          </div>
        </div>
      )
    }

    return <div className="p-4 text-sm text-gray-400">此类型暂不支持预览</div>
  }

  const renderRaw = () => {
    return (
      <pre className="p-4 text-xs font-mono overflow-auto bg-gray-50 dark:bg-gray-900" style={{ maxHeight: '600px' }}>
        {content}
      </pre>
    )
  }

  const renderHex = () => {
    return <HexViewer data={content} />
  }

  const viewModes: { key: ViewMode; label: string }[] = [
    { key: 'auto', label: '自动' },
    { key: 'formatted', label: '格式化' },
    { key: 'raw', label: '原始' },
    { key: 'preview', label: '预览' },
    { key: 'hex', label: '十六进制' },
  ]

  return (
    <div className="border border-gray-200 dark:border-gray-700 rounded">
      {/* 工具栏 */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800">
        <div className="flex items-center gap-2">
          <span className="text-xs font-semibold text-gray-700 dark:text-gray-300">
            {data?.type ? getTypeLabel(data.type) : '响应体'}
          </span>
          {data?.metadata && Object.keys(data.metadata).length > 0 && (
            <span className="text-xs text-gray-500">
              {formatMetadata(data.metadata)}
            </span>
          )}
          {data?.error && (
            <span className="text-xs text-red-500">解析错误: {data.error}</span>
          )}
        </div>
        <div className="flex items-center gap-1">
          {viewModes.map(m => (
            <Button
              key={m.key}
              size="sm"
              variant={viewMode === m.key ? undefined : 'ghost'}
              onClick={() => setViewMode(m.key)}
            >
              {m.label}
            </Button>
          ))}
          <Button size="sm" variant="ghost" onClick={() => copyToClipboard(content)}>
            复制
          </Button>
        </div>
      </div>

      {/* 内容区 */}
      <div className="overflow-auto" style={{ maxHeight: '600px' }}>
        {renderContent()}
      </div>
    </div>
  )
}

// ── JSON 树状查看器 ──
function JSONViewer({ data, expanded, onToggle }: { data: any; expanded: Set<string>; onToggle: (s: Set<string>) => void }) {
  const renderValue = (value: any, path: string, depth: number): JSX.Element => {
    const indent = depth * 20

    if (value === null) {
      return <span className="text-gray-500">null</span>
    }

    if (typeof value === 'boolean') {
      return <span className="text-blue-600">{value.toString()}</span>
    }

    if (typeof value === 'number') {
      return <span className="text-purple-600">{value}</span>
    }

    if (typeof value === 'string') {
      return <span className="text-green-600">"{value}"</span>
    }

    if (Array.isArray(value)) {
      const isExpanded = expanded.has(path)
      return (
        <div>
          <span
            className="cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800 px-1 rounded"
            onClick={() => togglePath(path)}
          >
            {isExpanded ? '▼' : '▶'} Array({value.length})
          </span>
          {isExpanded && (
            <div style={{ marginLeft: `${indent}px` }}>
              {value.map((item, idx) => (
                <div key={idx} className="border-l border-gray-300 dark:border-gray-600 pl-2">
                  <span className="text-gray-500">[{idx}]:</span> {renderValue(item, `${path}[${idx}]`, depth + 1)}
                </div>
              ))}
            </div>
          )}
        </div>
      )
    }

    if (typeof value === 'object') {
      const keys = Object.keys(value)
      const isExpanded = expanded.has(path)
      return (
        <div>
          <span
            className="cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800 px-1 rounded"
            onClick={() => togglePath(path)}
          >
            {isExpanded ? '▼' : '▶'} Object({keys.length})
          </span>
          {isExpanded && (
            <div style={{ marginLeft: `${indent}px` }}>
              {keys.map(key => (
                <div key={key} className="border-l border-gray-300 dark:border-gray-600 pl-2">
                  <span className="text-blue-700 dark:text-blue-400">{key}</span>:{' '}
                  {renderValue(value[key], `${path}.${key}`, depth + 1)}
                </div>
              ))}
            </div>
          )}
        </div>
      )
    }

    return <span>{String(value)}</span>
  }

  const togglePath = (path: string) => {
    const newExpanded = new Set(expanded)
    if (newExpanded.has(path)) {
      newExpanded.delete(path)
    } else {
      newExpanded.add(path)
    }
    onToggle(newExpanded)
  }

  return (
    <div className="p-4 text-sm font-mono overflow-auto">
      {renderValue(data, 'root', 0)}
    </div>
  )
}

// ── 表单数据查看器 ──
function FormDataViewer({ data }: { data: any }) {
  if (!data || typeof data !== 'object') {
    return <div className="p-4 text-sm text-gray-400">无效的表单数据</div>
  }

  return (
    <div className="p-4">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-gray-300">
            <th className="text-left py-2 px-2">字段名</th>
            <th className="text-left py-2 px-2">值</th>
          </tr>
        </thead>
        <tbody>
          {Object.entries(data).map(([key, value]) => (
            <tr key={key} className="border-b border-gray-200">
              <td className="py-2 px-2 font-semibold text-blue-600">{key}</td>
              <td className="py-2 px-2 text-gray-700 dark:text-gray-300">
                {Array.isArray(value) ? value.join(', ') : String(value)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ── 代码高亮查看器（简化版，可接入 highlight.js）──
function CodeViewer({ code, language }: { code: string; language: string }) {
  return (
    <pre className="p-4 text-xs font-mono overflow-auto bg-gray-50 dark:bg-gray-900">
      <code className={`language-${language}`}>{code}</code>
    </pre>
  )
}

// ── 十六进制查看器 ──
function HexViewer({ data }: { data: string }) {
  const bytes = new TextEncoder().encode(data)
  const lines: string[] = []

  for (let i = 0; i < bytes.length; i += 16) {
    const chunk = bytes.slice(i, i + 16)
    const hex = Array.from(chunk, b => b.toString(16).padStart(2, '0')).join(' ')
    const ascii = Array.from(chunk, b => (b >= 32 && b < 127 ? String.fromCharCode(b) : '.')).join('')
    lines.push(`${i.toString(16).padStart(8, '0')}  ${hex.padEnd(48, ' ')}  ${ascii}`)
  }

  return (
    <pre className="p-4 text-xs font-mono overflow-auto bg-gray-50 dark:bg-gray-900">
      {lines.join('\n')}
    </pre>
  )
}

// ── 辅助函数 ──
function getTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    json: 'JSON',
    xml: 'XML',
    html: 'HTML',
    image: '图片',
    video: '视频',
    audio: '音频',
    text: '文本',
    binary: '二进制',
    form: '表单',
    graphql: 'GraphQL',
    javascript: 'JavaScript',
    css: 'CSS',
    protobuf: 'Protobuf',
  }
  return labels[type] || type.toUpperCase()
}

function formatMetadata(metadata: Record<string, any>): string {
  const parts: string[] = []
  if (metadata.keys) parts.push(`${metadata.keys} 个键`)
  if (metadata.arrayLength) parts.push(`数组长度 ${metadata.arrayLength}`)
  if (metadata.fieldCount) parts.push(`${metadata.fieldCount} 个字段`)
  if (metadata.isMinified) parts.push('已压缩')
  return parts.join(' · ')
}

function detectLanguage(dataType: string, mimeType?: string): string {
  if (dataType === 'javascript') return 'javascript'
  if (dataType === 'css') return 'css'
  if (dataType === 'html') return 'html'
  if (dataType === 'xml') return 'xml'
  if (mimeType?.includes('javascript')) return 'javascript'
  if (mimeType?.includes('css')) return 'css'
  return 'text'
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text).catch(() => {
    console.error('复制失败')
  })
}
