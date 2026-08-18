// 生成真机分布指纹池(完整版),输出到 backend/internal/identity/data/pool.json。
// 运行期 Go 端用 go:embed 内嵌该文件并加权采样,保证每环境唯一且自洽。
//
// 用法(需联网一次):
//   cd tools/identity && npm init -y && npm i fingerprint-generator
//   node gen-pool.mjs 10000
//
// 说明:fingerprint-generator(Apify,Apache-2.0)基于真机遥测的贝叶斯网络采样,
// 组合天然自洽。字段命名以其当前版本为准,如有出入按下方 map 调整即可。
//
// 规范化(与 backend/internal/identity/validator.go 一致):
//   - hardwareConcurrency 就近映射到真实偶数集合 {2,4,6,8,10,12,14,16,20,24},
//     剔除奇数与 36/40/44/64/640 等(消费级 x86 因超线程几乎都是偶数)。
//   - deviceMemory 钳到 ≤8,取 {4,8}(W3C navigator.deviceMemory 上限就是 8)。
//   - dpr 归一到干净集合 {1,1.25,1.5,1.75,2,2.5}。
//   - UA 版本按 148:144=70:30 加权重写为 Reduction 形式(只换虚构版本号,保留硬件多样性)。
//   - 平台仅 windows/macos(不收 linux)。

import { FingerprintGenerator } from 'fingerprint-generator';
import { writeFileSync } from 'node:fs';

const COUNT = Number(process.argv[2] || 10000);
const generator = new FingerprintGenerator();

const HC_SET = [2, 4, 6, 8, 10, 12, 14, 16, 20, 24];
const DPR_SET = [1, 1.25, 1.5, 1.75, 2, 2.5];

function normalizeHC(n) {
  n = Math.max(2, Math.min(24, n || 8));
  if (n % 2 !== 0) n -= 1;
  return HC_SET.reduce((best, v) => (Math.abs(v - n) < Math.abs(best - n) ? v : best), HC_SET[0]);
}
function normalizeDM(n) { return (!n || n < 4) ? 4 : (n >= 8 ? 8 : 4); }
function normalizeDPR(d) {
  d = Math.max(1, Math.min(2.5, d || 1));
  return DPR_SET.reduce((best, v) => (Math.abs(v - d) < Math.abs(best - d) ? v : best), 1);
}
function reductionUA(platform, major) {
  const chrome = `Chrome/${major}.0.0.0`;
  return platform === 'macos'
    ? `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) ${chrome} Safari/537.36`
    : `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) ${chrome} Safari/537.36`;
}
function majorForIndex(i) { return (i % 100) < 70 ? 148 : 144; }

function toPlatform(nav, ua) {
  const s = `${nav?.platform || nav?.oscpu || ''} ${ua}`.toLowerCase();
  if (s.includes('mac')) return 'macos';
  if (s.includes('linux') && !s.includes('android')) return 'linux'; // 会被下游丢弃
  return 'windows';
}

const seen = new Set();
const records = [];
for (let i = 0; records.length < COUNT && i < COUNT * 4; i++) {
  const { fingerprint } = generator.getFingerprint({
    devices: ['desktop'],
    browsers: ['chrome'],
    operatingSystems: ['windows', 'macos', 'linux'],
  });
  const nav = fingerprint.navigator || {};
  const scr = fingerprint.screen || {};
  const ua = nav.userAgent;
  if (!ua || !scr.width || !scr.height) continue;

  const key = `${ua}|${scr.width}x${scr.height}|${nav.hardwareConcurrency}`;
  if (seen.has(key)) continue;
  seen.add(key);

  const platform = toPlatform(nav, ua);
  if (platform === 'linux') continue; // 不收 linux 平台
  if (/CCleaner|Edg\/|OPR\/|Brave/i.test(ua)) continue;

  const major = majorForIndex(records.length);
  const langs = Array.isArray(nav.languages) && nav.languages.length ? nav.languages : ['en-US', 'en'];
  const availW = scr.availWidth || scr.width;
  const availH = scr.availHeight || scr.height;

  records.push({
    platform,
    platformVersion: '',
    brandVersion: `${major}.0.0.0`,
    uaFull: reductionUA(platform, major),
    hardwareConcurrency: normalizeHC(nav.hardwareConcurrency || 8),
    deviceMemory: normalizeDM(nav.deviceMemory || 8),
    screen: {
      width: scr.width,
      height: scr.height,
      dpr: normalizeDPR(scr.devicePixelRatio || 1),
      colorDepth: scr.colorDepth || 24,
    },
    windowSize: `${Math.min(scr.width, availW)},${Math.min(scr.height, availH)}`,
    languages: langs,
    locale: langs[0],
    timezone: 'America/New_York', // 直连默认;绑定代理后由离线 GeoIP 覆盖
    weight: 1,
  });
}

const out = new URL('../../backend/internal/identity/data/pool.json', import.meta.url);
writeFileSync(out, JSON.stringify(records, null, 2));
console.log(`wrote ${records.length} records -> ${out.pathname}`);
