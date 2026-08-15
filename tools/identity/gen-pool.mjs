// 生成真机分布指纹池(完整版),输出到 backend/internal/identity/data/pool.json。
// 运行期 Go 端用 go:embed 内嵌该文件并加权采样,保证每环境唯一且自洽。
//
// 用法(需联网一次):
//   cd tools/identity && npm init -y && npm i fingerprint-generator
//   node gen-pool.mjs 10000
//
// 说明:fingerprint-generator(Apify,Apache-2.0)基于真机遥测的贝叶斯网络采样,
// 组合天然自洽。字段命名以其当前版本为准,如有出入按下方 map 调整即可。

import { FingerprintGenerator } from 'fingerprint-generator';
import { writeFileSync } from 'node:fs';

const COUNT = Number(process.argv[2] || 10000);
const generator = new FingerprintGenerator();

function toPlatform(nav, ua) {
  const s = `${nav?.platform || nav?.oscpu || ''} ${ua}`.toLowerCase();
  if (s.includes('mac')) return 'macos';
  if (s.includes('linux') && !s.includes('android')) return 'linux';
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

  const langs = Array.isArray(nav.languages) && nav.languages.length ? nav.languages : ['en-US', 'en'];
  const brandVersion = (ua.match(/Chrome\/([\d.]+)/) || [, ''])[1];
  const availW = scr.availWidth || scr.width;
  const availH = scr.availHeight || scr.height;

  records.push({
    platform: toPlatform(nav, ua),
    platformVersion: '',
    brandVersion,
    uaFull: ua,
    hardwareConcurrency: nav.hardwareConcurrency || 8,
    deviceMemory: nav.deviceMemory || 8,
    screen: {
      width: scr.width,
      height: scr.height,
      dpr: scr.devicePixelRatio || 1,
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
