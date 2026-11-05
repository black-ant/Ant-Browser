const {
  normalizeTimeout,
  isPlainObject,
  hasOwnProperty,
  requestJSON,
} = require('./runner_shared.cjs');

function normalizeOrigin(value) {
  const normalized = String(value || '').trim();
  if (!normalized) {
    return '';
  }

  try {
    const parsed = new URL(normalized);
    if (!['http:', 'https:'].includes(parsed.protocol)) {
      return '';
    }
    return parsed.origin;
  } catch {
    return '';
  }
}

function normalizePermissionList(value) {
  const source = Array.isArray(value) ? value : value == null ? [] : [value];
  const result = [];
  const seen = new Set();

  for (const item of source) {
    const normalized = String(item || '').trim();
    if (!normalized || seen.has(normalized)) {
      continue;
    }
    seen.add(normalized);
    result.push(normalized);
  }

  return result;
}

function normalizePageAPIHeaders(value) {
  const headers = {};
  if (!value) {
    return headers;
  }

  if (typeof value.forEach === 'function') {
    value.forEach((entryValue, entryKey) => {
      const key = String(entryKey || '').trim();
      if (key) {
        headers[key] = String(entryValue);
      }
    });
    return headers;
  }

  if (Array.isArray(value)) {
    for (const entry of value) {
      if (!Array.isArray(entry) || entry.length < 2) {
        continue;
      }
      const key = String(entry[0] || '').trim();
      if (key) {
        headers[key] = String(entry[1]);
      }
    }
    return headers;
  }

  if (isPlainObject(value)) {
    for (const [key, entryValue] of Object.entries(value)) {
      const normalizedKey = String(key || '').trim();
      if (normalizedKey && entryValue !== undefined && entryValue !== null) {
        headers[normalizedKey] = String(entryValue);
      }
    }
  }
  return headers;
}

function setPageAPIHeaderIfAbsent(headers, key, value) {
  const normalizedKey = String(key || '').trim();
  if (!normalizedKey) {
    return;
  }
  const lowerKey = normalizedKey.toLowerCase();
  if (Object.keys(headers).some((existingKey) => existingKey.toLowerCase() === lowerKey)) {
    return;
  }
  headers[normalizedKey] = value;
}

function appendPageAPIQuery(rawURL, query) {
  if (!isPlainObject(query) && !Array.isArray(query)) {
    return rawURL;
  }

  const searchParams = new URLSearchParams();
  const appendEntry = (key, value) => {
    const normalizedKey = String(key || '').trim();
    if (!normalizedKey || value === undefined || value === null) {
      return;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        appendEntry(normalizedKey, item);
      }
      return;
    }
    searchParams.append(normalizedKey, String(value));
  };

  if (Array.isArray(query)) {
    for (const entry of query) {
      if (Array.isArray(entry) && entry.length >= 2) {
        appendEntry(entry[0], entry[1]);
      }
    }
  } else {
    for (const [key, value] of Object.entries(query)) {
      appendEntry(key, value);
    }
  }

  const queryText = searchParams.toString();
  if (!queryText) {
    return rawURL;
  }

  const hashIndex = rawURL.indexOf('#');
  const baseURL = hashIndex >= 0 ? rawURL.slice(0, hashIndex) : rawURL;
  const hash = hashIndex >= 0 ? rawURL.slice(hashIndex) : '';
  const separator = baseURL.includes('?')
    ? baseURL.endsWith('?') || baseURL.endsWith('&')
      ? ''
      : '&'
    : '?';
  return `${baseURL}${separator}${queryText}${hash}`;
}

function normalizePageAPICredentials(value) {
  const normalized = String(value || '').trim();
  if (['include', 'same-origin', 'omit'].includes(normalized)) {
    return normalized;
  }
  return 'include';
}

function normalizePageAPIBody(source, headers) {
  if (hasOwnProperty(source, 'bodyText')) {
    return source.bodyText == null ? null : String(source.bodyText);
  }

  if (hasOwnProperty(source, 'json')) {
    setPageAPIHeaderIfAbsent(headers, 'Content-Type', 'application/json');
    return JSON.stringify(source.json == null ? null : source.json);
  }

  if (!hasOwnProperty(source, 'body')) {
    return null;
  }

  const body = source.body;
  if (body == null) {
    return null;
  }
  if (typeof body === 'string') {
    return body;
  }
  setPageAPIHeaderIfAbsent(headers, 'Content-Type', 'application/json');
  return JSON.stringify(body);
}

function normalizePageAPIRequest(urlOrRequest, options = {}) {
  const base = isPlainObject(urlOrRequest) ? urlOrRequest : { url: urlOrRequest };
  const source = {
    ...base,
    ...(isPlainObject(options) ? options : {}),
  };
  const headers = normalizePageAPIHeaders(source.headers);
  const bodyText = normalizePageAPIBody(source, headers);
  const method = String(
    source.method || (bodyText == null ? 'GET' : 'POST')
  )
    .trim()
    .toUpperCase();
  const url = appendPageAPIQuery(String(source.url || '').trim(), source.query || source.searchParams);

  if (!url) {
    throw new Error('page api url is required');
  }
  if ((method === 'GET' || method === 'HEAD') && bodyText != null) {
    throw new Error(`${method} page api request cannot include a body`);
  }

  return {
    url,
    method,
    headers,
    credentials: normalizePageAPICredentials(source.credentials),
    bodyText,
    timeoutMs: normalizeTimeout(source.timeoutMs, 30000),
    parseJSON: source.parseJSON !== false,
    throwOnError: source.throwOnError === true || source.throwOnHTTPError === true,
  };
}

async function executePageAPIRequest(request) {
  const headers = request && request.headers && typeof request.headers === 'object'
    ? request.headers
    : {};
  const init = {
    method: request.method || 'GET',
    headers,
    credentials: request.credentials || 'include',
  };

  let timeoutID = null;
  if (request.timeoutMs > 0 && typeof AbortController !== 'undefined') {
    const controller = new AbortController();
    init.signal = controller.signal;
    timeoutID = setTimeout(() => controller.abort(), request.timeoutMs);
  }

  if (request.bodyText !== null && request.bodyText !== undefined) {
    init.body = request.bodyText;
  }

  try {
    const response = await fetch(request.url, init);
    const responseHeaders = {};
    if (response.headers && typeof response.headers.forEach === 'function') {
      response.headers.forEach((value, key) => {
        responseHeaders[key] = value;
      });
    }

    const bodyText = await response.text();
    let bodyJSON = null;
    let hasBodyJSON = false;
    if (request.parseJSON !== false && String(bodyText || '').trim()) {
      try {
        bodyJSON = JSON.parse(bodyText);
        hasBodyJSON = true;
      } catch {}
    }

    return {
      ok: response.ok,
      status: response.status,
      statusText: response.statusText,
      url: response.url,
      headers: responseHeaders,
      bodyText,
      bodyJSON: hasBodyJSON ? bodyJSON : null,
      json: hasBodyJSON ? bodyJSON : null,
      error: response.ok ? '' : response.statusText || `HTTP ${response.status}`,
    };
  } catch (error) {
    const message = error && error.message ? error.message : String(error);
    return {
      ok: false,
      status: 0,
      statusText: '',
      url: request.url,
      headers: {},
      bodyText: '',
      bodyJSON: null,
      json: null,
      error: message,
    };
  } finally {
    if (timeoutID) {
      clearTimeout(timeoutID);
    }
  }
}

module.exports = {
  normalizeOrigin,
  normalizePermissionList,
  normalizePageAPIRequest,
  executePageAPIRequest,
};