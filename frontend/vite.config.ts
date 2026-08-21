import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'

const defaultDevPort = 5218

// 版本号单一事实来源:wails.json 的 info.productVersion(后端也从这里编译期嵌入)。
// 前端曾经在 projectBase.config.ts 里手写版本号,结果长期与实际发版版本脱节 —— 改为构建期注入。
function resolveProductVersion() {
  const currentDir = dirname(fileURLToPath(import.meta.url))
  try {
    const raw = readFileSync(resolve(currentDir, '../wails.json'), 'utf-8')
    const version = String(JSON.parse(raw)?.info?.productVersion ?? '').trim()
    if (version) {
      return version
    }
    console.warn('[vite] wails.json 未配置 info.productVersion，版本号回退为 unknown')
  } catch (error) {
    console.warn('[vite] 读取 wails.json 失败，版本号回退为 unknown:', error)
  }
  return 'unknown'
}

function resolveBoolean(rawValue: string | undefined, fallbackValue: boolean) {
  const raw = String(rawValue ?? '').trim().toLowerCase()
  if (!raw) {
    return fallbackValue
  }
  if (raw === '1' || raw === 'true' || raw === 'yes' || raw === 'on') {
    return true
  }
  if (raw === '0' || raw === 'false' || raw === 'no' || raw === 'off') {
    return false
  }
  return fallbackValue
}

function resolveDevPort() {
  const raw = Number.parseInt(process.env.FRONTEND_PORT || '', 10)
  if (Number.isInteger(raw) && raw > 0 && raw <= 65535) {
    return raw
  }
  return defaultDevPort
}

const devPort = resolveDevPort()
const disableHmr = resolveBoolean(process.env.FRONTEND_DISABLE_HMR, false)
const cleanDist = resolveBoolean(process.env.FRONTEND_CLEAN_DIST, false)

export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify(resolveProductVersion()),
  },
  server: {
    port: devPort,
    strictPort: true,
    host: '127.0.0.1',
    cors: true,
    hmr: disableHmr
      ? false
      : {
          host: '127.0.0.1',
          protocol: 'ws',
        },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    emptyOutDir: cleanDist,
    rollupOptions: {
      output: {
        manualChunks: {
          'react-vendor': ['react', 'react-dom', 'react-router-dom'],
        },
      },
    },
  },
})

