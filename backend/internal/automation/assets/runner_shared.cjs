const fs = require('fs');
const http = require('http');
const https = require('https');
const path = require('path');
const util = require('util');

const ALLOWED_WAIT_UNTIL = new Set(['load', 'domcontentloaded', 'networkidle', 'commit']);

function normalizeTimeout(value, fallback) {
  const parsed = Number(value);
  if (Number.isFinite(parsed) && parsed > 0) {
    return Math.round(parsed);
  }
  return fallback;
}

function isPlainObject(value) {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value));
}

function hasOwnProperty(value, key) {
  return Object.prototype.hasOwnProperty.call(value, key);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function writeStream(stream, text) {
  return new Promise((resolve, reject) => {
    stream.write(text, (error) => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
  });
}

async function closeBrowserConnection(browser) {
  if (!browser || typeof browser.close !== 'function') {
    return;
  }
  await browser.close({ reason: 'automation task finished' }).catch(() => {});
}

function normalizeEndpointCandidate(value) {
  const normalized = String(value || '').trim();
  if (!normalized) {
    return '';
  }

  try {
    const parsed = new URL(normalized);
    if (!['http:', 'https:', 'ws:', 'wss:'].includes(parsed.protocol)) {
      return '';
    }
    if (parsed.port === '0') {
      return '';
    }
    if ((parsed.protocol === 'http:' || parsed.protocol === 'https:') && (!parsed.pathname || parsed.pathname === '/') && !parsed.search && !parsed.hash) {
      return parsed.origin;
    }
    return parsed.toString();
  } catch {
    return '';
  }
}

function buildConnectEndpoints(payload, session) {
  const candidates = [];
  const seen = new Set();

  const pushCandidate = (value) => {
    const endpoint = normalizeEndpointCandidate(value);
    if (!endpoint || seen.has(endpoint)) {
      return;
    }
    seen.add(endpoint);
    candidates.push(endpoint);
  };

  pushCandidate(session && session.cdpUrl);

  const debugPort = Number(session && session.debugPort);
  if (Number.isFinite(debugPort) && debugPort > 0) {
    pushCandidate(`http://127.0.0.1:${Math.round(debugPort)}`);
  }

  pushCandidate(payload && payload.launchBaseUrl);
  return candidates;
}

function normalizePathUnderRoot(rootDir, targetName) {
  const normalizedName = String(targetName || '').trim();
  const resolvedRoot = path.resolve(String(rootDir || ''));
  if (!resolvedRoot) {
    throw new Error('artifactDir is required');
  }

  const candidate = normalizedName ? path.resolve(resolvedRoot, normalizedName) : resolvedRoot;
  if (candidate !== resolvedRoot && !candidate.startsWith(`${resolvedRoot}${path.sep}`)) {
    throw new Error('artifact path escapes root directory');
  }
  return candidate;
}

async function requestJSON(method, requestURL, body, headers = {}) {
  const target = new URL(requestURL);
  const transport = target.protocol === 'https:' ? https : http;
  const payload = body == null ? '' : JSON.stringify(body);

  return await new Promise((resolve, reject) => {
    const req = transport.request(
      {
        protocol: target.protocol,
        hostname: target.hostname,
        port: target.port,
        path: `${target.pathname}${target.search}`,
        method,
        headers: {
          Accept: 'application/json',
          ...(payload
            ? {
                'Content-Type': 'application/json',
                'Content-Length': Buffer.byteLength(payload),
              }
            : {}),
          ...headers,
        },
      },
      (res) => {
        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => {
          const rawText = Buffer.concat(chunks).toString('utf8').trim();
          let responseBody = {};
          if (rawText) {
            try {
              responseBody = JSON.parse(rawText);
            } catch {
              responseBody = { rawBody: rawText };
            }
          }
          resolve({
            status: res.statusCode || 0,
            body: responseBody,
          });
        });
      }
    );

    req.on('error', reject);
    if (payload) {
      req.write(payload);
    }
    req.end();
  });
}

function inspectValue(value) {
  return util.inspect(value, {
    depth: 4,
    breakLength: 120,
    maxArrayLength: 20,
    compact: false,
  });
}

function toSerializable(value, seen = new WeakSet()) {
  if (value == null) {
    return value;
  }
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return value;
  }
  if (typeof value === 'bigint') {
    return value.toString();
  }
  if (value instanceof Date) {
    return value.toISOString();
  }
  if (value instanceof Error) {
    return {
      name: value.name,
      message: value.message,
      stack: value.stack,
    };
  }
  if (Buffer.isBuffer(value)) {
    return value.toString('utf8');
  }
  if (Array.isArray(value)) {
    return value.map((item) => toSerializable(item, seen));
  }
  if (typeof value === 'function') {
    return `[Function ${value.name || 'anonymous'}]`;
  }
  if (typeof value !== 'object') {
    return inspectValue(value);
  }
  if (seen.has(value)) {
    return '[Circular]';
  }
  seen.add(value);

  const prototype = Object.getPrototypeOf(value);
  if (prototype === Object.prototype || prototype === null) {
    const result = {};
    for (const [key, entry] of Object.entries(value)) {
      result[key] = toSerializable(entry, seen);
    }
    return result;
  }

  return inspectValue(value);
}


module.exports = {
  normalizeTimeout,
  isPlainObject,
  hasOwnProperty,
  sleep,
  writeStream,
  closeBrowserConnection,
  buildConnectEndpoints,
  normalizePathUnderRoot,
  requestJSON,
  toSerializable,
};