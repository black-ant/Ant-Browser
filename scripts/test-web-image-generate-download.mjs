import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { createRequire } from 'node:module'

const require = createRequire(import.meta.url)
const imageScript = require('../backend/internal/automation/demo-library/web-image-generate-download/index.cjs')
const artifactRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'ant-gateway-image-script-'))
const images = []
let imageCounter = 0

function generateImage() {
  imageCounter += 1
  images.push({ src: `https://cdn.test/generated-${imageCounter}.png`, marker: '' })
}

class FakeLocator {
  constructor(selector, index = -1) {
    this.selector = selector
    this.index = index
  }

  async count() {
    if (this.selector === 'body') return 1
    if (this.selector.includes('prompt-textarea')) return 1
    if (this.selector.includes('send-button')) return 1
    if (this.selector.startsWith('[data-ant-gateway-image-candidate=')) {
      return images.filter((image) => this.selector.includes(`"${image.marker}"`)).length
    }
    if (this.selector.includes('img[')) return images.length
    return 0
  }

  nth(index) {
    return new FakeLocator(this.selector, index)
  }

  first() {
    return this.nth(0)
  }

  async isVisible() {
    return true
  }

  async isEnabled() {
    return true
  }

  async click() {
    if (this.selector.includes('send-button')) generateImage()
  }

  async fill() {}

  async press() {}

  async innerText() {
    return ''
  }

  async evaluate(fn, value) {
    const source = String(fn)
    if (this.selector === 'body') return ''
    if (this.selector.includes('prompt-textarea')) {
      return source.includes('isContentEditable') ? false : 'textarea'
    }

    let image = null
    if (this.selector.startsWith('[data-ant-gateway-image-candidate=')) {
      image = images.find((item) => this.selector.includes(`"${item.marker}"`))
    } else if (this.selector.includes('img[')) {
      image = images[this.index]
    }
    if (!image) return ''
    if (value !== undefined) {
      image.marker = value
      return undefined
    }
    if (source.includes('getBoundingClientRect')) {
      return { tagName: 'img', src: image.src, width: 1024, height: 1024 }
    }
    return image.src
  }
}

const page = {
  url: () => 'https://chatgpt.com/',
  waitForTimeout: async () => {},
  locator: (selector) => new FakeLocator(selector),
  keyboard: { press: async () => {}, type: async () => {} },
  evaluate: async () => ({
    ok: true,
    status: 200,
    statusText: 'OK',
    contentType: 'image/png',
    bytes: [137, 80, 78, 71],
  }),
}

const useBrowser = async () => ({ page, session: {} })
const artifact = (name) => path.join(artifactRoot, name)

const run = (prompt, outputFileName) => imageScript.run({
  useBrowser,
  selector: { code: 'TEST_CODE' },
  params: {
    prompt,
    timeoutMs: 5000,
    waitAfterLoadMs: 0,
    settleMs: 0,
    outputFileName,
  },
  artifact,
})

const first = await run('first', 'first.png')
const second = await run('second', 'second.png')
const firstSource = first.steps.find((step) => step.step === 'wait_image').imageInfo.src
const secondSource = second.steps.find((step) => step.step === 'wait_image').imageInfo.src

assert.equal(first.ok, true)
assert.equal(second.ok, true)
assert.notEqual(firstSource, secondSource)
assert.equal(fs.readFileSync(first.downloadAddress).length, 4)
assert.equal(fs.readFileSync(second.downloadAddress).length, 4)
console.log(JSON.stringify({ first: firstSource, second: secondSource, status: 'ok' }))
