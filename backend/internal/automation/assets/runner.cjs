const fs = require('fs');
const http = require('http');
const https = require('https');
const path = require('path');
const util = require('util');
const { pathToFileURL } = require('url');

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

function buildLaunchRequestBody(defaultSelector, options) {
  const launchOptions = options && typeof options === 'object' ? options : {};
  const body = {};

  for (const key of [
    'code',
    'key',
    'profileId',
    'profileName',
    'keyword',
    'keywords',
    'tag',
    'tags',
    'groupId',
    'matchMode',
    'proxyId',
    'proxyConfig',
    'launchArgs',
    'startUrls',
    'skipDefaultStartUrls',
  ]) {
    if (Object.prototype.hasOwnProperty.call(launchOptions, key)) {
      body[key] = launchOptions[key];
    }
  }

  const selector =
    launchOptions.selector &&
    typeof launchOptions.selector === 'object' &&
    !Array.isArray(launchOptions.selector)
      ? launchOptions.selector
      : defaultSelector;
  if (selector && typeof selector === 'object' && !Array.isArray(selector) && Object.keys(selector).length > 0) {
    body.selector = selector;
  }

  return body;
}

async function loadScriptModule(scriptPath) {
  const resolvedPath = path.resolve(String(scriptPath || ''));
  if (!resolvedPath) {
    throw new Error('scriptPath is required');
  }

  let requiredModule = null;
  let requireError = null;
  try {
    requiredModule = require(resolvedPath);
  } catch (error) {
    requireError = error;
  }

  const imported = async () => {
    const moduleURL = pathToFileURL(resolvedPath).href;
    return await import(`${moduleURL}?t=${Date.now()}`);
  };

  if (requiredModule && typeof requiredModule.run === 'function') {
    return requiredModule;
  }
  if (typeof requiredModule === 'function') {
    return { run: requiredModule };
  }
  if (requiredModule && requiredModule.default && typeof requiredModule.default.run === 'function') {
    return requiredModule.default;
  }

  try {
    const importedModule = await imported();
    if (importedModule && typeof importedModule.run === 'function') {
      return importedModule;
    }
    if (importedModule && typeof importedModule.default === 'function') {
      return { run: importedModule.default };
    }
    if (
      importedModule &&
      importedModule.default &&
      typeof importedModule.default.run === 'function'
    ) {
      return importedModule.default;
    }
  } catch (importError) {
    if (requireError) {
      throw requireError;
    }
    throw importError;
  }

  if (requireError) {
    throw requireError;
  }
  throw new Error('script must export run()');
}

async function runScriptTask(payload, chromium) {
  const scriptModule = await loadScriptModule(payload.scriptPath);
  if (!scriptModule || typeof scriptModule.run !== 'function') {
    throw new Error('script must export run()');
  }

  const logs = [];
  const artifacts = [];
  const connectedBrowsers = new Set();
  const selector = payload.selector && typeof payload.selector === 'object' ? payload.selector : {};
  const params = payload.params && typeof payload.params === 'object' ? payload.params : {};
  const timeout = normalizeTimeout(params.timeoutMs, 30000);
  const startedAt = new Date().toISOString();

  const log = (...entries) => {
    logs.push({
      time: new Date().toISOString(),
      values: entries.map((entry) => toSerializable(entry)),
    });
  };

  const artifact = (name) => {
    const fileName = String(name || '').trim() || `artifact-${Date.now()}`;
    const targetPath = normalizePathUnderRoot(payload.artifactDir, fileName);
    fs.mkdirSync(path.dirname(targetPath), { recursive: true });
    artifacts.push(targetPath);
    return targetPath;
  };

  const launchHeaders = {};
  if (payload.launchAuthHeader && payload.launchAuthValue) {
    launchHeaders[payload.launchAuthHeader] = payload.launchAuthValue;
  }

  const launch = async (options = {}) => {
    const body = buildLaunchRequestBody(selector, options);

    const response = await requestJSON(
      'POST',
      `${String(payload.launchBaseUrl || '').replace(/\/$/, '')}/api/launch`,
      body,
      launchHeaders
    );

    if (!(response.status >= 200 && response.status < 300) || response.body.ok === false) {
      const errorText =
        (response.body && response.body.error && String(response.body.error).trim()) ||
        `launch api returned http ${response.status}`;
      throw new Error(errorText);
    }

    return response.body;
  };

  const connect = async (session = {}, options = {}) => {
    const connectOptions =
      options && typeof options === 'object' && !Array.isArray(options) ? options : {};
    const endpoints = buildConnectEndpoints(payload, session);
    if (endpoints.length === 0) {
      throw new Error(
        `launch session does not contain a valid cdp endpoint (cdpUrl=${String(
          session && session.cdpUrl ? session.cdpUrl : ''
        )}, debugPort=${String(session && session.debugPort ? session.debugPort : '')})`
      );
    }

    const connectTimeout = normalizeTimeout(connectOptions.timeoutMs, timeout);
    const deadline = Date.now() + connectTimeout;
    let lastError = null;

    while (Date.now() <= deadline) {
      for (const endpoint of endpoints) {
        const remaining = deadline - Date.now();
        if (remaining <= 0) {
          break;
        }

        try {
          const browser = await chromium.connectOverCDP(endpoint, {
            timeout: Math.max(1000, Math.min(remaining, connectTimeout)),
          });
          connectedBrowsers.add(browser);
          const context = browser.contexts()[0] || null;
          const page = context && context.pages().length > 0 ? context.pages()[0] : null;
          return {
            browser,
            context,
            page,
            session: {
              ...session,
              cdpUrl: endpoint,
            },
          };
        } catch (error) {
          lastError = error;
        }
      }

      if (Date.now() >= deadline) {
        break;
      }

      await sleep(Math.min(500, Math.max(100, deadline - Date.now())));
    }

    const lastMessage =
      lastError && lastError.message ? lastError.message : String(lastError || 'unknown error');
    throw new Error(
      `cdp endpoint is not ready after ${connectTimeout} ms (endpoints: ${endpoints.join(', ')}): ${lastMessage}`
    );
  };

  const resolveConnectionContext = async (connection) => {
    const browser = connection && connection.browser ? connection.browser : null;
    if (!browser) {
      throw new Error('browser connection is unavailable');
    }

    const context =
      connection.context ||
      browser.contexts()[0] ||
      (typeof browser.newContext === 'function' ? await browser.newContext() : null);
    if (!context) {
      throw new Error('browser context is unavailable');
    }

    return {
      browser,
      context,
    };
  };

  const grantPermissions = async (target, options = {}) => {
    const permissionOptions =
      options && typeof options === 'object' && !Array.isArray(options) ? options : {};
    const permissions = normalizePermissionList(permissionOptions.permissions);
    const origin = normalizeOrigin(permissionOptions.origin);

    let context = null;
    if (target && typeof target.grantPermissions === 'function') {
      context = target;
    } else if (target && typeof target === 'object') {
      context = target.context || null;
      if (!context && target.browser) {
        const resolved = await resolveConnectionContext(target);
        context = resolved.context;
      }
    }

    if (!context) {
      return {
        applied: false,
        permissions,
        origin,
        reason: 'browser context is unavailable',
      };
    }
    if (!origin) {
      return {
        applied: false,
        permissions,
        origin: '',
        reason: 'origin is required',
      };
    }
    if (permissions.length === 0) {
      return {
        applied: false,
        permissions,
        origin,
        reason: 'permissions are required',
      };
    }
    if (typeof context.grantPermissions !== 'function') {
      return {
        applied: false,
        permissions,
        origin,
        reason: 'grantPermissions is unavailable',
      };
    }

    try {
      await context.grantPermissions(permissions, { origin });
      return {
        applied: true,
        permissions,
        origin,
        strategy: 'grantPermissions',
      };
    } catch (error) {
      return {
        applied: false,
        permissions,
        origin,
        reason: error && error.message ? error.message : String(error),
      };
    }
  };

  const openPage = async (connection, options = {}) => {
    const openOptions =
      options && typeof options === 'object' && !Array.isArray(options) ? options : {};
    const { browser, context } = await resolveConnectionContext(connection);
    const shouldReuseCurrentPage = openOptions.reuseCurrentPage === true;

    let page = null;
    if (
      shouldReuseCurrentPage &&
      connection &&
      connection.page &&
      typeof connection.page.isClosed === 'function' &&
      !connection.page.isClosed()
    ) {
      page = connection.page;
    }
    if (!page) {
      page = await context.newPage();
    }

    if (typeof page.bringToFront === 'function' && openOptions.bringToFront !== false) {
      await page.bringToFront().catch(() => {});
    }

    const permissionResult =
      openOptions.permissions !== undefined
        ? await grantPermissions(context, {
            origin:
              typeof openOptions.permissionOrigin === 'string' && openOptions.permissionOrigin.trim()
                ? openOptions.permissionOrigin
                : openOptions.url,
            permissions: openOptions.permissions,
          })
        : {
            applied: false,
            permissions: [],
            origin: '',
            reason: '',
          };

    const targetURL = String(openOptions.url || '').trim();
    if (targetURL) {
      const waitUntil = ALLOWED_WAIT_UNTIL.has(String(openOptions.waitUntil || '').trim())
        ? String(openOptions.waitUntil).trim()
        : 'domcontentloaded';
      await page.goto(targetURL, {
        waitUntil,
        timeout: normalizeTimeout(openOptions.timeoutMs, timeout),
      });
    }

    return {
      browser,
      context,
      page,
      permissionResult,
      reusedPage: page === (connection && connection.page ? connection.page : null),
    };
  };

  const resolvePageTarget = (target) => {
    if (target && typeof target.evaluate === 'function') {
      return target;
    }
    if (target && target.page && typeof target.page.evaluate === 'function') {
      return target.page;
    }
    throw new Error('page api target must be a Playwright page or an object containing page');
  };

  const callPageAPI = async (target, urlOrRequest, options = {}) => {
    const page = resolvePageTarget(target);
    const request = normalizePageAPIRequest(urlOrRequest, options);
    const response = await page.evaluate(executePageAPIRequest, request);

    if (request.throwOnError && (!response || response.ok !== true)) {
      const status = response && response.status ? response.status : 0;
      const message =
        (response && typeof response.error === 'string' && response.error.trim()) ||
        (status ? `page api returned http ${status}` : 'page api request failed');
      throw new Error(message);
    }

    return response;
  };

  const browserFetch = callPageAPI;
  const pageAPI = callPageAPI;

  const useBrowser = async (options = {}) => {
    const runOptions = options && typeof options === 'object' && !Array.isArray(options) ? options : {};
    const launchOptions =
      runOptions.launch && typeof runOptions.launch === 'object' && !Array.isArray(runOptions.launch)
        ? runOptions.launch
        : runOptions;
    const connectOptions =
      runOptions.connect && typeof runOptions.connect === 'object' && !Array.isArray(runOptions.connect)
        ? runOptions.connect
        : {};
    const openOptions =
      runOptions.open && typeof runOptions.open === 'object' && !Array.isArray(runOptions.open)
        ? runOptions.open
        : {
            url: runOptions.url,
            waitUntil: runOptions.waitUntil,
            timeoutMs: runOptions.timeoutMs,
            permissions: runOptions.permissions,
            permissionOrigin: runOptions.permissionOrigin,
            reuseCurrentPage: runOptions.reuseCurrentPage,
            bringToFront: runOptions.bringToFront,
          };

    const session = await launch(launchOptions);
    const connection = await connect(session, connectOptions);
    const opened = await openPage(connection, openOptions);
    return {
      session,
      connection,
      ...opened,
    };
  };

  const api = {
    chromium,
    launch,
    connect,
    grantPermissions,
    openPage,
    useBrowser,
    callPageAPI,
    pageAPI,
    browserFetch,
    selector,
    params,
    log,
    artifact,
    artifactsDir: payload.artifactDir || '',
  };

  try {
    const rawResult = await scriptModule.run(api);
    const normalizedResult = toSerializable(rawResult);
    const ok = !(normalizedResult && typeof normalizedResult === 'object' && normalizedResult.ok === false);
    const summary =
      normalizedResult &&
      typeof normalizedResult === 'object' &&
      typeof normalizedResult.summary === 'string'
        ? normalizedResult.summary.trim()
        : ok
          ? '脚本执行完成'
          : '脚本执行失败';
    const error =
      normalizedResult &&
      typeof normalizedResult === 'object' &&
      typeof normalizedResult.error === 'string'
        ? normalizedResult.error.trim()
        : '';

    return {
      ok,
      summary,
      error,
      title:
        normalizedResult &&
        typeof normalizedResult === 'object' &&
        typeof normalizedResult.title === 'string'
          ? normalizedResult.title
          : '',
      url:
        normalizedResult &&
        typeof normalizedResult === 'object' &&
        typeof normalizedResult.url === 'string'
          ? normalizedResult.url
          : '',
      startedAt,
      finishedAt: new Date().toISOString(),
      isolatedPage: false,
      logs,
      artifacts: Array.from(new Set(artifacts)),
      result: normalizedResult,
    };
  } finally {
    await Promise.all(Array.from(connectedBrowsers, (browser) => closeBrowserConnection(browser)));
  }
}

async function main() {
  const payloadPath = process.argv[2];
  if (!payloadPath) {
    throw new Error('payload path is required');
  }

  const payload = JSON.parse(fs.readFileSync(payloadPath, 'utf8'));
  const runtimeDir = path.resolve(String(payload.runtimeDir || ''));
  if (!runtimeDir) {
    throw new Error('runtimeDir is required');
  }

  const { chromium } = require(path.join(runtimeDir, 'node_modules', 'playwright-core'));
  const taskType = String(payload.taskType || 'script').trim() || 'script';
  if (taskType !== 'script') {
    throw new Error(`unsupported automation task type: ${taskType}`);
  }

  const result = await runScriptTask(payload, chromium);
  await writeStream(process.stdout, JSON.stringify(result));
  process.exit(0);
}

main().catch(async (error) => {
  const message = error && error.message ? error.message : String(error);
  try {
    await writeStream(process.stderr, message);
  } finally {
    process.exit(1);
  }
});
