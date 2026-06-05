import {
  AUTOMATION_SCRIPT_MANIFEST_VERSION,
  AUTOMATION_SCRIPT_PACKAGE_FORMAT,
  DUAL_INSTANCE_RUNTIME_SCRIPT_ID,
  type AutomationScriptRecord,
  type AutomationScriptType,
} from "./definitions";
import { createAutomationScriptPublicAPIConfig } from "./publicApi";
import {
  normalizeAutomationScriptTargetConfig,
  normalizeAutomationScriptTargetSelector,
} from "./targets";

const BACKEND_BUILTIN_SCRIPT_PLACEHOLDER = `module.exports.run = async () => {
  throw new Error('内置脚本源码由后端 demo-library 提供，请在桌面应用后端环境中加载或从脚本包导入。')
}`;

const DUAL_INSTANCE_DEFAULT_CODES = ["BUYER_001", "BUYER_002"] as const;
const DUAL_INSTANCE_DEFAULT_START_URLS = [
  "https://finance.sina.com.cn/",
  "https://map.baidu.com/",
] as const;

function nowIso(): string {
  return new Date().toISOString();
}

export function buildSelectorTemplate(type: AutomationScriptType): string {
  if (type === "launch-api") {
    return `{
  "code": "BUYER_001"
}`;
  }

  return "";
}

export function buildParamsTemplate(type: AutomationScriptType): string {
  if (type === "launch-api") {
    return `{
  "startUrls": ["https://example.com"],
  "skipDefaultStartUrls": true
}`;
  }

  return `{
  "url": "https://www.baidu.com",
  "keyword": "OpenAI",
  "timeoutMs": 30000,
  "waitAfterSearchMs": 1500,
  "captureScreenshot": true
}`;
}

export function buildScriptTemplate(type: AutomationScriptType): string {
  if (type === "launch-api") {
    return `export async function run({ baseUrl, apiKey, selector, params }) {
  const response = await fetch(\`\${baseUrl}/api/launch\`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(apiKey ? { 'X-Ant-Api-Key': apiKey } : {}),
    },
    body: JSON.stringify({
      selector,
      ...(params || {}),
    }),
  })

  if (!response.ok) {
    throw new Error(\`launch failed: \${response.status}\`)
  }

  return await response.json()
}`;
  }

  return `module.exports.run = async ({ useBrowser, browserFetch, selector, params, log, artifact }) => {
  const targetUrl =
    typeof params.url === 'string' && params.url.trim()
      ? params.url.trim()
      : 'https://www.baidu.com'
  const keyword =
    typeof params.keyword === 'string' && params.keyword.trim()
      ? params.keyword.trim()
      : 'OpenAI'
  const timeout =
    Number.isFinite(Number(params.timeoutMs)) && Number(params.timeoutMs) > 0
      ? Math.round(Number(params.timeoutMs))
      : 30000
  const waitAfterSearchMs =
    Number.isFinite(Number(params.waitAfterSearchMs)) && Number(params.waitAfterSearchMs) >= 0
      ? Math.round(Number(params.waitAfterSearchMs))
      : 1500

  const runtime = await useBrowser({
    selector,
    startUrls: params.startUrls || [targetUrl],
    skipDefaultStartUrls: true,
    url: targetUrl,
    timeoutMs: timeout,
    reuseCurrentPage: true,
  })
  const page = runtime.page

  const searchInput = page.locator('textarea[name="wd"], input[name="wd"]').first()
  await searchInput.waitFor({
    state: 'visible',
    timeout,
  })
  await searchInput.fill(keyword)
  await searchInput.press('Enter').catch(async () => {
    const submitButton = page.locator('#su, input[type="submit"]').first()
    await submitButton.click({ timeout })
  })
  await page.waitForURL(/wd=/, { timeout }).catch(() => {})

  if (waitAfterSearchMs > 0) {
    await page.waitForTimeout(waitAfterSearchMs)
  }

  if (params.captureScreenshot !== false) {
    await page.screenshot({
      path: artifact('baidu-search.png'),
      fullPage: true,
    })
  }

  const title = await page.title()
  let apiResult = null
  const apiUrl = typeof params.apiUrl === 'string' ? params.apiUrl.trim() : ''
  if (apiUrl) {
    const apiRequest = {
      url: apiUrl,
      method: params.apiBody === undefined ? 'GET' : 'POST',
      timeoutMs: timeout,
    }
    if (params.apiBody !== undefined) {
      apiRequest.json = params.apiBody
    }
    apiResult = await browserFetch(page, apiRequest)
  }
  log('keyword', keyword)
  log('title', title)

  return {
    ok: true,
    summary: \`已在百度搜索 \${keyword}\`,
    keyword,
    url: page.url(),
    title,
    apiResult,
  }
}`;
}

export function buildNotesTemplate(type: AutomationScriptType): string {
  if (type === "launch-api") {
    return "适合外部调度器或 HTTP 中台。脚本负责组装 selector 和 launch 参数，不直接接管页面。";
  }

  return "默认示例使用 useBrowser 启动并接管页面；需要调用站内接口时传 apiUrl/apiBody，会通过 browserFetch 在浏览器上下文发起请求。";
}

function buildDualInstanceRuntimeParamsText(
  codes = [...DUAL_INSTANCE_DEFAULT_CODES],
): string {
  return `{
  "browsers": [
    {
      "code": "${codes[0] || DUAL_INSTANCE_DEFAULT_CODES[0]}",
      "skipDefaultStartUrls": true,
      "startUrls": ["${DUAL_INSTANCE_DEFAULT_START_URLS[0]}"]
    },
    {
      "code": "${codes[1] || DUAL_INSTANCE_DEFAULT_CODES[1]}",
      "skipDefaultStartUrls": true,
      "startUrls": ["${DUAL_INSTANCE_DEFAULT_START_URLS[1]}"]
    }
  ],
  "timeoutMs": 45000
}`;
}

function buildDualInstanceRuntimeScriptText(): string {
  return `export async function run({ baseUrl, apiKey, params, log }) {
  const normalizeCode = (value, fallback) =>
    String(value || fallback || "").trim().toUpperCase()
  const normalizeStringArray = (value) =>
    Array.isArray(value)
      ? value
          .map((item) => String(item || "").trim())
          .filter(Boolean)
      : []
  const normalizeBrowserInput = (value, fallbackCode, fallbackStartUrls, defaultSkip) => {
    const raw = value && typeof value === "object" ? value : {}
    const code = normalizeCode(raw.code || raw.launchCode, fallbackCode)
    if (!code) {
      return null
    }
    const startUrls = normalizeStringArray(raw.startUrls)
    const fallbackUrls = normalizeStringArray(fallbackStartUrls)
    const launchArgs = normalizeStringArray(raw.launchArgs)

    return {
      code,
      skipDefaultStartUrls:
        raw.skipDefaultStartUrls !== undefined
          ? raw.skipDefaultStartUrls !== false
          : defaultSkip,
      startUrls: startUrls.length > 0 ? startUrls : fallbackUrls,
      launchArgs,
    }
  }

  const timeoutMs = Number.isFinite(Number(params.timeoutMs))
    ? Math.max(1000, Math.round(Number(params.timeoutMs)))
    : 45000
  const defaultSkipDefaultStartUrls = params.skipDefaultStartUrls !== false

  let browsers = Array.isArray(params.browsers)
    ? params.browsers
        .map((item, index) =>
          normalizeBrowserInput(
            item,
            ${JSON.stringify([...DUAL_INSTANCE_DEFAULT_CODES])}[index] || "",
            ${JSON.stringify([...DUAL_INSTANCE_DEFAULT_START_URLS])}[index] || [],
            defaultSkipDefaultStartUrls,
          ),
        )
        .filter(Boolean)
    : []

  if (browsers.length === 0) {
    browsers = [
      normalizeBrowserInput(
        { code: params.primaryCode, skipDefaultStartUrls: params.skipDefaultStartUrls },
        ${JSON.stringify(DUAL_INSTANCE_DEFAULT_CODES[0])},
        ${JSON.stringify([DUAL_INSTANCE_DEFAULT_START_URLS[0]])},
        defaultSkipDefaultStartUrls,
      ),
      normalizeBrowserInput(
        { code: params.secondaryCode, skipDefaultStartUrls: params.skipDefaultStartUrls },
        ${JSON.stringify(DUAL_INSTANCE_DEFAULT_CODES[1])},
        ${JSON.stringify([DUAL_INSTANCE_DEFAULT_START_URLS[1]])},
        defaultSkipDefaultStartUrls,
      ),
    ].filter(Boolean)
  }

  if (browsers.length === 0) {
    throw new Error("params.browsers 不能为空")
  }

  const headers = {
    "Content-Type": "application/json",
    ...(apiKey ? { "X-Ant-Api-Key": apiKey } : {}),
  }

  const post = async (path, payload) => {
    const response = await fetch(\`\${baseUrl}\${path}\`, {
      method: "POST",
      headers,
      body: JSON.stringify(payload),
    })
    const text = await response.text()
    let body = text
    try {
      body = text ? JSON.parse(text) : null
    } catch {
      body = text
    }
    if (!response.ok) {
      throw new Error(\`\${path} failed: \${response.status} \${text}\`)
    }
    return body
  }

  const sessions = []

  for (const browser of browsers) {
    const sessionResult = await post("/api/runtime/session", {
      selector: { code: browser.code, matchMode: "unique" },
      skipDefaultStartUrls: browser.skipDefaultStartUrls,
      ...(browser.startUrls.length > 0 ? { startUrls: browser.startUrls } : {}),
      ...(browser.launchArgs.length > 0 ? { launchArgs: browser.launchArgs } : {}),
      timeoutMs,
    })

    sessions.push(sessionResult)
  }

  const browserCodes = browsers.map((item) => item.code)
  log("browserCodes", browserCodes)

  return {
    ok: true,
    summary: \`\${browserCodes.length} 个浏览器已就绪：\${browserCodes.join(" / ")}\`,
    browserCodes,
    sessions,
  }
}`;
}

export function normalizeDualInstanceRuntimeParamsText(text: string): string {
  const fallback = buildDualInstanceRuntimeParamsText();

  try {
    const parsed = JSON.parse(text);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return fallback;
    }

    const raw = parsed as Record<string, unknown>;
    const topLevelSkipDefaultStartUrls = raw.skipDefaultStartUrls !== false;
    const rawBrowsers = Array.isArray(raw.browsers) ? raw.browsers : [];
    const browsers = rawBrowsers
      .map((item, index) => {
        if (!item || typeof item !== "object") {
          return null;
        }
        const entry = item as Record<string, unknown>;
        const code = normalizeAutomationScriptTargetSelector({
          code:
            typeof entry.code === "string"
              ? entry.code
              : typeof entry.launchCode === "string"
                ? entry.launchCode
                : "",
        }).code;
        if (!code) {
          return null;
        }

        const startUrls = Array.isArray(entry.startUrls)
          ? entry.startUrls
              .map((value) => String(value || "").trim())
              .filter(Boolean)
          : [];
        const launchArgs = Array.isArray(entry.launchArgs)
          ? entry.launchArgs
              .map((value) => String(value || "").trim())
              .filter(Boolean)
          : [];

        const fallbackStartUrls = DUAL_INSTANCE_DEFAULT_START_URLS[index]
          ? [DUAL_INSTANCE_DEFAULT_START_URLS[index]]
          : [];

        return {
          code: code || DUAL_INSTANCE_DEFAULT_CODES[index] || "",
          skipDefaultStartUrls:
            entry.skipDefaultStartUrls !== undefined
              ? entry.skipDefaultStartUrls !== false
              : topLevelSkipDefaultStartUrls,
          startUrls: startUrls.length > 0 ? startUrls : fallbackStartUrls,
          ...(launchArgs.length > 0 ? { launchArgs } : {}),
        };
      })
      .filter(
        (
          item,
        ): item is {
          code: string;
          skipDefaultStartUrls: boolean;
          startUrls: string[];
          launchArgs?: string[];
        } => item !== null,
      );

    const legacyCodes = [
      normalizeAutomationScriptTargetSelector({
        code: typeof raw.primaryCode === "string" ? raw.primaryCode : "",
      }).code,
      normalizeAutomationScriptTargetSelector({
        code: typeof raw.secondaryCode === "string" ? raw.secondaryCode : "",
      }).code,
    ].filter(Boolean);

    const normalizedBrowsers =
      browsers.length > 0
        ? browsers
        : legacyCodes.length > 0
          ? legacyCodes.map((code, index) => ({
              code,
              skipDefaultStartUrls: topLevelSkipDefaultStartUrls,
              startUrls: DUAL_INSTANCE_DEFAULT_START_URLS[index]
                ? [DUAL_INSTANCE_DEFAULT_START_URLS[index]]
                : [],
            }))
          : DUAL_INSTANCE_DEFAULT_CODES.map((code, index) => ({
              code,
              skipDefaultStartUrls: true,
              startUrls: DUAL_INSTANCE_DEFAULT_START_URLS[index]
                ? [DUAL_INSTANCE_DEFAULT_START_URLS[index]]
                : [],
            }));

    const timeoutMs =
      Number.isFinite(Number(raw.timeoutMs)) && Number(raw.timeoutMs) > 0
        ? Math.round(Number(raw.timeoutMs))
        : 45000;

    return JSON.stringify(
      {
        browsers: normalizedBrowsers,
        timeoutMs,
      },
      null,
      2,
    );
  } catch {
    return fallback;
  }
}

export function createNewsTxtScriptDraft(): AutomationScriptRecord {
  const createdAt = nowIso();

  return {
    packageFormat: AUTOMATION_SCRIPT_PACKAGE_FORMAT,
    manifestVersion: AUTOMATION_SCRIPT_MANIFEST_VERSION,
    id: "news-query-txt",
    name: "查询新闻并写 TXT",
    description: "通过 Bing 搜索新闻关键词，提取结果并写入本地 txt 文件。",
    type: "playwright-cdp",
    status: "ready",
    entryFile: "index.cjs",
    tags: ["Playwright", "新闻", "TXT"],
    selectorText: "",
    paramsText: `{
  "keyword": "OpenAI",
  "limit": 10,
  "timeRange": "week",
  "outputFileName": "openai-news.txt",
  "timeoutMs": 30000,
  "waitAfterLoadMs": 1500,
  "captureScreenshot": false
}`,
    scriptText: BACKEND_BUILTIN_SCRIPT_PLACEHOLDER,
    notes:
      "脚本会优先使用 Bing 搜索真实新闻结果，并自动追加时间过滤、排除问答/聚合站点、回退查询词和质量校验；只有达到新闻质量门槛时才会判定成功，并把结果写入本地 txt。执行成功后可在结果里的 outputPath 找到文件。",
    targetConfig: normalizeAutomationScriptTargetConfig(null),
    publicAPI: createAutomationScriptPublicAPIConfig(),
    source: {
      type: "builtin",
      uri: "repo://backend/internal/automation/demo-library/news-query-txt",
      ref: "HEAD",
      path: "news-query-txt",
      importedAt: "",
    },
    createdAt,
    updatedAt: createdAt,
  };
}

export function createDualInstanceRuntimeScriptDraft(): AutomationScriptRecord {
  const createdAt = nowIso();

  return {
    packageFormat: AUTOMATION_SCRIPT_PACKAGE_FORMAT,
    manifestVersion: AUTOMATION_SCRIPT_MANIFEST_VERSION,
    id: DUAL_INSTANCE_RUNTIME_SCRIPT_ID,
    name: "双实例启动与 Runtime 切换",
    description:
      "通过 Launch API 分别启动两个实例，切换 Runtime 会话后交给 OpenClaw 执行。",
    type: "launch-api",
    status: "ready",
    entryFile: "index.cjs",
    tags: ["Launch API", "OpenClaw", "双实例"],
    selectorText: "",
    paramsText: buildDualInstanceRuntimeParamsText(),
    scriptText: buildDualInstanceRuntimeScriptText(),
    notes:
      "先通过接口启动两个实例并切换 Runtime 会话；随后把实例信息交给 OpenClaw 执行自动化动作。",
    targetConfig: normalizeAutomationScriptTargetConfig(null),
    publicAPI: createAutomationScriptPublicAPIConfig(),
    source: {
      type: "builtin",
      uri: "repo://backend/internal/automation/demo-library/dual-instance-runtime-switch",
      ref: "HEAD",
      path: "dual-instance-runtime-switch",
      importedAt: "",
    },
    createdAt,
    updatedAt: createdAt,
  };
}

export function createWebImageGenerateDownloadScriptDraft(): AutomationScriptRecord {
  const createdAt = nowIso();

  return {
    packageFormat: AUTOMATION_SCRIPT_PACKAGE_FORMAT,
    manifestVersion: AUTOMATION_SCRIPT_MANIFEST_VERSION,
    id: "web-image-generate-download",
    name: "网页图片生成并下载",
    description:
      "打开指定网页，创建新会话，发送图片生成消息，等待图片生成后下载图片。当前是等待补充页面信息的脚手架。",
    type: "playwright-cdp",
    status: "draft",
    entryFile: "index.cjs",
    tags: ["Playwright", "图片生成", "下载", "脚手架"],
    selectorText: "",
    paramsText: `{
  "pageUrl": "https://chatgpt.com/",
  "prompt": "A cinematic chrome ant browser mascot, premium product lighting",
  "outputFileName": "generated-image.png",
  "selectors": {
    "newSessionButton": "",
    "promptInput": "#prompt-textarea[contenteditable=\"true\"], textarea[name=\"prompt-textarea\"]",
    "sendButton": "button[data-testid=\"send-button\"], button[aria-label*=\"发送\"], button.composer-submit-button-color",
    "generatedImage": "img[src*=\"/backend-api/estuary/content\"], img[alt*=\"已生成图片\"], img[src*=\"oaiusercontent\"], img[src*=\"oaidalleapiprodscus\"], img[alt*=\"生成\"], img[alt*=\"image\" i]",
    "downloadButton": ""
  },
  "timeoutMs": 300000,
  "waitAfterLoadMs": 1200,
  "settleMs": 2500,
  "captureScreenshot": false
}`,
    scriptText: BACKEND_BUILTIN_SCRIPT_PLACEHOLDER,
    notes:
      "脚本默认打开 ChatGPT，输入图片生成提示词并发送；等待 img[src*=\"/backend-api/estuary/content\"] 或 alt 包含“已生成图片”的结果出现后，使用页面登录态读取图片地址并保存到本地。",
    targetConfig: normalizeAutomationScriptTargetConfig(null),
    publicAPI: {
      ...createAutomationScriptPublicAPIConfig(),
      enabled: true,
      path: "image/chatgpt-generate-download",
      timeoutMs: 300000,
      requestBodyText: `{
  "params": {
    "prompt": "{{prompt}}"
  }
}`,
      responseBodyText: `{
  "ok": true,
  "outputPath": "\${artifactsDir}/generated-image.png",
  "downloadAddress": "\${artifactsDir}/generated-image.png"
}`,
      variables: [
        {
          name: "prompt",
          defaultValue:
            "A cinematic chrome ant browser mascot, premium product lighting",
          description: "发送到 ChatGPT 的图片生成提示词。",
          required: true,
        },
      ],
    },
    source: {
      type: "builtin",
      uri: "repo://backend/internal/automation/demo-library/web-image-generate-download",
      ref: "HEAD",
      path: "web-image-generate-download",
      importedAt: "",
    },
    createdAt,
    updatedAt: createdAt,
  };
}

export function buildDefaultAutomationScripts(): AutomationScriptRecord[] {
  return [
    createNewsTxtScriptDraft(),
    createDualInstanceRuntimeScriptDraft(),
    createWebImageGenerateDownloadScriptDraft(),
  ];
}
