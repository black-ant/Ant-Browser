/// <reference types="vite/client" />

// 由 vite.config.ts 的 define 在构建期注入,取自 wails.json 的 info.productVersion。
declare const __APP_VERSION__: string
