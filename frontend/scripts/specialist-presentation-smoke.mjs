import assert from 'node:assert/strict'
import { readFile, rm } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { spawnSync } from 'node:child_process'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const outDir = resolve(root, '.tmp', 'specialist-presentation-smoke')
const tsc = resolve(root, 'node_modules', 'typescript', 'bin', 'tsc')

await rm(outDir, { recursive: true, force: true })

const compile = spawnSync(
  process.execPath,
  [
    tsc,
    '--target',
    'ES2020',
    '--module',
    'NodeNext',
    '--moduleResolution',
    'NodeNext',
    '--strict',
    '--skipLibCheck',
    '--outDir',
    outDir,
    '--rootDir',
    resolve(root, 'src'),
    resolve(root, 'src', 'modules', 'specialist', 'presentation.ts'),
  ],
  { cwd: root, stdio: 'inherit' },
)

if (compile.status !== 0) {
  process.exit(compile.status ?? 1)
}

const presentation = await import(
  pathToFileURL(resolve(outDir, 'modules', 'specialist', 'presentation.js')).href
)

assert.equal(presentation.specialistTaskStatusLabel('pending'), '待开始')
assert.equal(presentation.specialistTaskStatusLabel('submitted_pending_validation'), '已提交待校验')
assert.equal(presentation.specialistTaskStatusLabel('appeal_in_review'), '申诉中')
assert.equal(presentation.specialistTaskPriorityLabel('critical'), '紧急')
assert.equal(presentation.specialistTaskPriorityLabel('high'), '高')
assert.equal(presentation.specialistTaskPriorityLabel('medium'), '中')
assert.equal(presentation.specialistTaskPriorityLabel('low'), '低')

const past = new Date(Date.now() - 60_000).toISOString()
assert.equal(presentation.specialistTaskDeadlineTone(past), 'danger')

assert.equal(
  presentation.requiredStepSummary([
    { required: true, status: 'done' },
    { required: true, status: 'not_started' },
    { required: false, status: 'not_started' },
  ]),
  '1/2',
)

const pageSource = await readFile(
  resolve(root, 'src', 'modules', 'specialist', 'pages', 'SpecialistTaskPanelPage.tsx'),
  'utf8',
)
const drawerSource = await readFile(
  resolve(root, 'src', 'modules', 'specialist', 'components', 'SpecialistTaskDrawer.tsx'),
  'utf8',
)

assert.match(pageSource, /from 'antd'/, 'specialist task page should use Ant Design components')
assert.match(drawerSource, /from 'antd'/, 'specialist task drawer should use Ant Design components')
assert.doesNotMatch(
  pageSource,
  /shared\/components/,
  'specialist task page must not use custom shared business controls',
)
assert.doesNotMatch(
  drawerSource,
  /shared\/components/,
  'specialist task drawer must not use custom shared business controls',
)
assert.doesNotMatch(
  pageSource,
  /requiredStepSummary\(row\.sopSteps\)/,
  'task list must not claim SOP progress when list responses omit SOP steps',
)

await rm(outDir, { recursive: true, force: true })
console.log('specialist presentation smoke passed')
