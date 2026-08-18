// 规范化现有身份池:就地修正字段,保留全部记录(真机硬件/屏幕/时区多样性宝贵),
// 只把虚构的 UA 版本重写到已内置内核版本 {148,144}。不依赖网络/npm。
// 运行: node tools/identity/normalize-pool.mjs
//
// 规范(与 backend/internal/identity/validator.go 新规则一致):
//   - hardwareConcurrency:仅保留偶数;就近映射到真实加权集合
//     主 {4,6,8,12,16}、尾 {2,10,14,20,24};剔除奇数与 36/40/44/64/640 等。
//   - deviceMemory:钳到 ≤8,取 {4,8}(W3C navigator.deviceMemory 上限就是 8)。
//   - dpr:归一到干净集合 {1,1.25,1.5,1.75,2,2.5}。
//   - UA:丢弃含 CCleaner/非 Chrome brand 的记录;版本按 148:144=70:30 加权重写为
//     Reduction 形式(保留记录的硬件/屏幕/时区多样性,只换掉虚构版本号)。
//   - 平台:仅保留 windows/macos。
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const poolPath = join(here, '../../backend/internal/identity/data/pool.json');

const HC_SET = [2, 4, 6, 8, 10, 12, 14, 16, 20, 24]; // 合法偶数集合
function normalizeHC(n) {
  if (!Number.isFinite(n) || n <= 0) return 8;
  if (n % 2 !== 0) {
    // 奇数就近向下到偶数,但避开 0
    n = n - 1;
  }
  if (n > 24) n = 24; // 36/40/44/64/640 全压到 24(高端桌面真实上限)
  if (n < 2) n = 2;
  // 就近映射到集合(取距离最小的;并列取较小)
  let best = HC_SET[0];
  for (const v of HC_SET) {
    if (Math.abs(v - n) < Math.abs(best - n) || (Math.abs(v - n) === Math.abs(best - n) && v < best)) {
      best = v;
    }
  }
  return best;
}

function normalizeDM(n) {
  if (!Number.isFinite(n) || n <= 0) return 8;
  if (n >= 8) return 8;
  if (n >= 4) return 4;
  return 4; // 1/2 也并到 4(极少见且偏离主流)
}

const DPR_SET = [1, 1.25, 1.5, 1.75, 2, 2.5];
function normalizeDPR(d) {
  if (!Number.isFinite(d) || d <= 0) return 1;
  if (d > 3) d = 2.5;
  let best = DPR_SET[0];
  for (const v of DPR_SET) {
    if (Math.abs(v - d) < Math.abs(best - d)) best = v;
  }
  return best;
}

function reductionUAForPlatform(platform, major) {
  const chrome = `Chrome/${major}.0.0.0`;
  if (platform === 'macos') {
    return `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) ${chrome} Safari/537.36`;
  }
  return `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) ${chrome} Safari/537.36`;
}

// 确定性加权:按 index 落到 148:144 = 70:30 累计区间(与后端 core_distribution 同算法)。
function majorForIndex(i) {
  // 70% 148 / 30% 144
  return (i % 100) < 70 ? 148 : 144;
}

const raw = JSON.parse(readFileSync(poolPath, 'utf8'));
const out = [];
let dropped = 0;
let idx = 0;
for (const r of raw || []) {
  if (r.platform !== 'windows' && r.platform !== 'macos') { dropped++; continue; }
  // 丢弃含异常 brand 的记录(CCleaner 等),其余保留。
  if (typeof r.uaFull === 'string' && /CCleaner|Edg\/|OPR\/|Brave/i.test(r.uaFull)) { dropped++; continue; }
  const major = majorForIndex(idx++);
  out.push({
    platform: r.platform,
    platformVersion: '',
    brandVersion: `${major}.0.0.0`,
    uaFull: reductionUAForPlatform(r.platform, major),
    hardwareConcurrency: normalizeHC(r.hardwareConcurrency),
    deviceMemory: normalizeDM(r.deviceMemory),
    screen: {
      width: r.screen?.width || 1920,
      height: r.screen?.height || 1080,
      dpr: normalizeDPR(r.screen?.dpr),
      colorDepth: 32,
    },
    windowSize: r.windowSize || `${r.screen?.width || 1920},${(r.screen?.height || 1080) - 48}`,
    languages: Array.isArray(r.languages) && r.languages.length ? r.languages : ['en-US'],
    locale: r.locale || 'en-US',
    timezone: r.timezone || 'America/New_York',
    weight: 1,
  });
}

writeFileSync(poolPath, JSON.stringify(out, null, 2));
console.log(`normalized: ${out.length} kept, ${dropped} dropped -> ${poolPath}`);

// 自检:断言 0 条违规
let bad = 0;
for (const r of out) {
  if (r.hardwareConcurrency % 2 !== 0 || r.hardwareConcurrency < 2 || r.hardwareConcurrency > 24) { console.error('BAD hc', r.hardwareConcurrency); bad++; }
  if (r.deviceMemory > 8) { console.error('BAD dm', r.deviceMemory); bad++; }
  if (!DPR_SET.includes(r.screen.dpr)) { console.error('BAD dpr', r.screen.dpr); bad++; }
  const m = Number((r.uaFull.match(/Chrome\/(\d+)\./) || [, 0])[1]);
  if (m !== 148 && m !== 144) { console.error('BAD ua major', m); bad++; }
}
console.log(bad === 0 ? 'ASSERT-OK: 0 violations' : `ASSERT-FAIL: ${bad} violations`);
