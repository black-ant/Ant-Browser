// 从 build/appicon.svg 生成 ZwBrowser 全部图标资源。
// 用法(在本目录): npm i && node gen-icons.mjs
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { Resvg } from '@resvg/resvg-js'
import pngToIco from 'png-to-ico'

const here = dirname(fileURLToPath(import.meta.url))
const repo = resolve(here, '../..') // Ant-Browser-Plus 根目录
const svg = readFileSync(resolve(repo, 'build/appicon.svg'))

function renderPng(size) {
  const r = new Resvg(svg, { fitTo: { mode: 'width', value: size } })
  return r.render().asPng()
}

function writePng(relPath, size) {
  const out = resolve(repo, relPath)
  mkdirSync(dirname(out), { recursive: true })
  writeFileSync(out, renderPng(size))
  console.log(`png  ${relPath}  (${size}x${size})`)
}

async function writeIco(relPath, sizes) {
  const ico = await pngToIco(sizes.map(renderPng))
  const out = resolve(repo, relPath)
  mkdirSync(dirname(out), { recursive: true })
  writeFileSync(out, ico)
  console.log(`ico  ${relPath}  (${sizes.join(',')})`)
}

writePng('build/appicon.png', 1024) // Wails 主图标(macOS .icns 由 wails build 从它生成)
writePng('frontend/public/favicon.png', 64) // 浏览器标签 / index.html
writePng('frontend/src/resources/images/logo.png', 128) // 侧边栏品牌
await writeIco('build/windows/icon.ico', [16, 32, 48, 64, 128, 256]) // Windows 应用图标
await writeIco('backend/internal/tray/icon.ico', [16, 32, 48, 64]) // 系统托盘(Go embed)
console.log('done')
