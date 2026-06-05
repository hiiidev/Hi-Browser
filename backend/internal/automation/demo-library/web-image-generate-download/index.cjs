const fs = require('fs')
const path = require('path')

function normalizeText(value) {
  return String(value || '').trim()
}

function normalizeInt(value, fallback, min, max) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) {
    return fallback
  }
  const rounded = Math.round(parsed)
  if (rounded < min) {
    return min
  }
  if (rounded > max) {
    return max
  }
  return rounded
}

function sanitizeFileName(value, fallback) {
  const normalized = normalizeText(value || fallback)
    .replace(/[\\/:*?"<>|]+/g, '-')
    .replace(/\s+/g, ' ')
    .trim()
  return normalized || fallback
}

function ensurePngExtension(fileName) {
  if (/\.(png|jpg|jpeg|webp)$/i.test(fileName)) {
    return fileName
  }
  return fileName + '.png'
}

function resolveOutputPath(outputDir, outputFileName) {
  const safeName = ensurePngExtension(sanitizeFileName(outputFileName, 'generated-image.png'))
  return path.join(outputDir || process.cwd(), safeName)
}

function buildMissingSetup(selectors, pageUrl) {
  const missing = []
  if (!pageUrl) {
    missing.push('pageUrl')
  }
  for (const key of ['promptInput', 'generatedImage']) {
    if (!normalizeText(selectors[key])) {
      missing.push('selectors.' + key)
    }
  }
  return missing
}

async function detectLoginRequired(page) {
  const url = page.url()
  if (/auth\.openai\.com|\/auth\/login|email-verification/i.test(url)) {
    return { url, reason: 'auth_page' }
  }
  const bodyText = await page.locator('body').innerText({ timeout: 2000 }).catch(() => '')
  if (/requires you to be logged in|需要登录|登录以获取|登录以|log in|sign in|免费注册/i.test(bodyText)) {
    return { url, reason: 'login_required_text' }
  }
  return null
}

async function firstVisibleLocator(page, selector, timeoutMs, label) {
  const locator = page.locator(selector)
  const deadline = Date.now() + timeoutMs
  let lastCount = 0
  while (Date.now() < deadline) {
    lastCount = await locator.count().catch(() => 0)
    for (let index = 0; index < lastCount; index += 1) {
      const candidate = locator.nth(index)
      if (await candidate.isVisible().catch(() => false)) {
        return candidate
      }
    }
    await page.waitForTimeout(250)
  }
  throw new Error(`${label || 'selector'} 未找到可见元素：${selector}，匹配数量 ${lastCount}`)
}

async function clickWhenReady(page, selector, timeoutMs, label) {
  const locator = await firstVisibleLocator(page, selector, timeoutMs, label)
  await locator.click({ timeout: timeoutMs })
  return { step: label, selector }
}

async function fillPrompt(page, selector, prompt, timeoutMs) {
  const locator = await firstVisibleLocator(page, selector, timeoutMs, 'promptInput')
  const tagName = await locator.evaluate((element) => element.tagName.toLowerCase())
  const isContentEditable = await locator.evaluate((element) => element.isContentEditable)
  await locator.click({ timeout: timeoutMs })
  if (tagName === 'textarea' || tagName === 'input') {
    await locator.fill(prompt, { timeout: timeoutMs })
  } else if (isContentEditable) {
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+A' : 'Control+A')
    await page.keyboard.type(prompt, { delay: 8 })
  } else {
    await locator.fill(prompt, { timeout: timeoutMs })
  }
  return { step: 'input_prompt', selector, promptLength: prompt.length, inputMode: isContentEditable ? 'contenteditable' : tagName }
}

async function submitPrompt(page, selector, timeoutMs) {
  const normalizedSelector = normalizeText(selector)
  if (normalizedSelector) {
    const locator = await firstVisibleLocator(page, normalizedSelector, Math.min(timeoutMs, 15000), 'sendButton')
    await locator.click({ timeout: Math.min(timeoutMs, 15000) })
    return { step: 'submit_prompt', selector: normalizedSelector, submitMode: 'button' }
  }

  await page.keyboard.press('Enter')
  return { step: 'submit_prompt', selector: '', submitMode: 'keyboard-enter' }
}

async function waitForGeneratedImage(page, selector, timeoutMs) {
  const locator = page.locator(selector)
  const deadline = Date.now() + timeoutMs
  const loginRequiredPattern = /requires you to be logged in|需要登录|登录以获取|登录以|log in|sign in/i
  while (Date.now() < deadline) {
    if ((await locator.count().catch(() => 0)) > 0 && await locator.first().isVisible().catch(() => false)) {
      break
    }
    const bodyText = await page.locator('body').innerText({ timeout: 1000 }).catch(() => '')
    if (loginRequiredPattern.test(bodyText)) {
      return {
        step: 'wait_image',
        ok: false,
        status: 'needs_login',
        summary: 'ChatGPT 当前未登录，页面拒绝生成图片。',
      }
    }
    await page.waitForTimeout(1500)
  }
  const visibleLocator = await firstVisibleLocator(page, selector, Math.max(1000, deadline - Date.now()), 'generatedImage')
  const imageInfo = await visibleLocator.evaluate((element) => {
    const tagName = element.tagName.toLowerCase()
    const src = tagName === 'img' ? element.currentSrc || element.src || '' : ''
    const rect = element.getBoundingClientRect()
    return {
      tagName,
      src,
      width: Math.round(rect.width),
      height: Math.round(rect.height),
    }
  })
  return { step: 'wait_image', selector, imageInfo }
}

async function downloadByButton(page, selector, timeoutMs) {
  const downloadPromise = page.waitForEvent('download', { timeout: timeoutMs })
  await page.locator(selector).first().click({ timeout: timeoutMs })
  return downloadPromise
}

async function downloadByImageURL(page, imageSelector, timeoutMs, outputPath) {
  const locator = await firstVisibleLocator(page, imageSelector, timeoutMs, 'generatedImage')
  const imageUrl = await locator.evaluate((element) => {
    if (element.tagName.toLowerCase() !== 'img') {
      return ''
    }
    return element.currentSrc || element.src || ''
  })
  if (!imageUrl) {
    throw new Error('已找到生成结果，但没有可下载的图片 URL。请补充 selectors.downloadButton 或确认 generatedImage 指向 img。')
  }

  const result = await page.evaluate(async (targetUrl) => {
    const response = await fetch(targetUrl, { credentials: 'include' })
    if (!response.ok) {
      return {
        ok: false,
        status: response.status,
        statusText: response.statusText,
        contentType: response.headers.get('content-type') || '',
        bytes: [],
      }
    }
    const arrayBuffer = await response.arrayBuffer()
    return {
      ok: true,
      status: response.status,
      statusText: response.statusText,
      contentType: response.headers.get('content-type') || '',
      bytes: Array.from(new Uint8Array(arrayBuffer)),
    }
  }, imageUrl)
  if (!result.ok || result.bytes.length === 0) {
    throw new Error(`图片 URL 下载失败：HTTP ${result.status || 'unknown'} ${result.statusText || ''}`.trim())
  }
  const buffer = Buffer.from(result.bytes)
  fs.writeFileSync(outputPath, buffer)
  return { source: 'image-url', url: imageUrl, contentType: result.contentType, bytes: buffer.length }
}

async function captureScreenshotIfNeeded(page, enabled, outputDir, label) {
  if (!enabled) {
    return ''
  }
  const screenshotPath = path.join(outputDir || process.cwd(), `web-image-generate-download-${label}.png`)
  await page.screenshot({ path: screenshotPath, fullPage: true }).catch(() => {})
  return screenshotPath
}

exports.run = async function run({ useBrowser, params = {}, artifact, artifactsDir, log }) {
  const pageUrl = normalizeText(params.pageUrl || params.url) || 'https://chatgpt.com/'
  const prompt = normalizeText(params.prompt) || 'A cinematic chrome ant browser mascot, premium product lighting'
  const selectors = params.selectors && typeof params.selectors === 'object' ? params.selectors : {}
  const timeoutMs = normalizeInt(params.timeoutMs, 300000, 5000, 900000)
  const waitAfterLoadMs = normalizeInt(params.waitAfterLoadMs, 1200, 0, 30000)
  const settleMs = normalizeInt(params.settleMs, 2500, 0, 60000)
  const captureScreenshot = Boolean(params.captureScreenshot)
  const outputFileName = ensurePngExtension(sanitizeFileName(params.outputFileName, 'generated-image.png'))
  const outputPath = typeof artifact === 'function'
    ? artifact(outputFileName)
    : resolveOutputPath(artifactsDir, outputFileName)
  const steps = []

  const missing = buildMissingSetup(selectors, pageUrl)
  if (missing.length > 0) {
    return {
      ok: false,
      status: 'needs_page_info',
      summary: '网页图片生成脚手架已创建，等待补充页面 URL 和选择器。',
      missing,
      expectedFlow: [
        'open_page',
        'create_new_session',
        'send_image_prompt',
        'wait_image_generated',
        'download_image',
      ],
    }
  }

  log && log('web-image-generate-download:start', {
    pageUrl,
    promptLength: prompt.length,
    hasDownloadButton: Boolean(normalizeText(selectors.downloadButton)),
  })

  if (typeof useBrowser !== 'function') {
    throw new Error('automation runtime does not provide useBrowser')
  }

  const { page } = await useBrowser({
    url: pageUrl,
    waitUntil: 'domcontentloaded',
    timeoutMs,
    reuseCurrentPage: true,
    bringToFront: true,
  })
  if (waitAfterLoadMs > 0) {
    await page.waitForTimeout(waitAfterLoadMs)
  }
  steps.push({ step: 'open_page', url: pageUrl })

  const loginRequired = await detectLoginRequired(page)
  if (loginRequired) {
    const screenshotPath = await captureScreenshotIfNeeded(page, true, path.dirname(outputPath), 'needs-login')
    return {
      ok: false,
      status: 'needs_login',
      summary: '目标实例未登录 ChatGPT，无法生成图片。',
      screenshotPath,
      steps: [...steps, { step: 'check_login', ok: false, ...loginRequired }],
    }
  }

  if (normalizeText(selectors.newSessionButton)) {
    steps.push(await clickWhenReady(page, selectors.newSessionButton, timeoutMs, 'create_new_session'))
  } else {
    steps.push({ step: 'create_new_session', skipped: true, reason: 'selectors.newSessionButton is empty' })
  }
  steps.push(await fillPrompt(page, selectors.promptInput, prompt, timeoutMs))
  steps.push(await submitPrompt(page, selectors.sendButton, timeoutMs))
  const generatedImageStep = await waitForGeneratedImage(page, selectors.generatedImage, timeoutMs)
  steps.push(generatedImageStep)
  if (generatedImageStep && generatedImageStep.ok === false) {
    const screenshotPath = await captureScreenshotIfNeeded(page, true, path.dirname(outputPath), 'needs-login')
    return {
      ok: false,
      status: generatedImageStep.status || 'failed',
      summary: generatedImageStep.summary || '图片未生成。',
      screenshotPath,
      steps,
    }
  }

  if (settleMs > 0) {
    await page.waitForTimeout(settleMs)
  }

  let downloadInfo
  if (normalizeText(selectors.downloadButton)) {
    const download = await downloadByButton(page, selectors.downloadButton, timeoutMs)
    await download.saveAs(outputPath)
    downloadInfo = {
      source: 'download-button',
      suggestedFilename: download.suggestedFilename(),
    }
  } else {
    downloadInfo = await downloadByImageURL(page, selectors.generatedImage, timeoutMs, outputPath)
  }
  steps.push({ step: 'download_image', outputPath, ...downloadInfo })

  const screenshotPath = await captureScreenshotIfNeeded(page, captureScreenshot, path.dirname(outputPath), 'done')

  return {
    ok: true,
    status: 'completed',
    summary: '图片已生成并下载。',
    outputPath,
    downloadAddress: outputPath,
    screenshotPath,
    steps,
  }
}
