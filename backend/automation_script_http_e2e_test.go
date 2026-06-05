package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/config"
)

type automationHTTPProfileCreateResponse struct {
	OK         bool   `json:"ok"`
	Created    bool   `json:"created"`
	Launched   bool   `json:"launched"`
	ProfileID  string `json:"profileId"`
	LaunchCode string `json:"launchCode"`
}

type automationHTTPScriptsResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		Count int `json:"count"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	} `json:"data"`
}

type automationHTTPRunResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		Run struct {
			Status     string `json:"status"`
			Summary    string `json:"summary"`
			Error      string `json:"error"`
			ResultText string `json:"resultText"`
		} `json:"run"`
	} `json:"data"`
}

type automationHTTPHookEnvelopeResponse struct {
	OK      bool                   `json:"ok"`
	Status  string                 `json:"status"`
	Summary string                 `json:"summary"`
	Result  map[string]interface{} `json:"result"`
}

type automationHTTPLaunchLogsResponse struct {
	OK    bool              `json:"ok"`
	Items []json.RawMessage `json:"items"`
}

func TestAutomationScriptRunHTTPReturnsSavedMailProbeScript(t *testing.T) {
	nodePath := lookupAutomationHTTPProbeNode(t)
	chromePath := lookupAutomationHTTPProbeChrome(t)
	repoRoot := automationHTTPRepoRoot(t)

	tempRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Logging.FileEnabled = false
	cfg.LaunchServer.Port = automationHTTPFreePort(t)
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.Automation.SystemNodePath = nodePath
	cfg.Automation.HeadlessDefault = true
	if err := cfg.Save(filepath.Join(tempRoot, "config.yaml")); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	if err := prepareAutomationHTTPRuntime(tempRoot, repoRoot, cfg.Automation.RuntimeVersion); err != nil {
		t.Fatalf("prepare runtime failed: %v", err)
	}

	fixtureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, automationHTTPMailFixtureHTML)
	}))
	defer fixtureServer.Close()

	app := NewApp(tempRoot)
	Start(app, nil)
	defer Stop(app, nil)

	if err := app.BrowserCoreSave(BrowserCoreInput{
		CoreId:    "system-chrome",
		CoreName:  "System Chrome",
		CorePath:  chromePath,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("save core failed: %v", err)
	}
	if err := app.BrowserCoreSetDefault("system-chrome"); err != nil {
		t.Fatalf("set default core failed: %v", err)
	}

	baseURL, ok := app.GetLaunchServerInfo()["baseUrl"].(string)
	if !ok || strings.TrimSpace(baseURL) == "" {
		t.Fatalf("launch server baseUrl missing: %+v", app.GetLaunchServerInfo())
	}

	var createResp automationHTTPProfileCreateResponse
	if err := automationHTTPRequestJSON(http.MethodPost, baseURL+"/api/profiles", map[string]any{
		"profile": map[string]any{
			"profileName": "mail-probe",
			"launchArgs": []string{
				"--headless=new",
				"--disable-gpu",
				"--no-first-run",
				"--no-default-browser-check",
				"--window-size=1440,1024",
			},
		},
		"launchCode": "MAIL01",
	}, &createResp); err != nil {
		t.Fatalf("create profile via http failed: %v", err)
	}
	if !createResp.OK || !createResp.Created || createResp.LaunchCode != "MAIL01" {
		t.Fatalf("unexpected create response: %+v", createResp)
	}

	savedScript, err := app.AutomationScriptSave(automation.ScriptRecord{
		ID:         "mail-probe-script",
		Name:       "测试邮件探针",
		Type:       "playwright-cdp",
		Status:     "ready",
		EntryFile:  "index.cjs",
		ScriptText: automationHTTPMailProbeScriptText,
	})
	if err != nil {
		t.Fatalf("save mail probe script failed: %v", err)
	}
	if savedScript == nil {
		t.Fatalf("expected mail probe script to be saved")
	}
	savedScript.PublicAPI = automation.ScriptPublicAPIConfig{
		Enabled:      true,
		Method:       "POST",
		Path:         "mail/probe-message",
		RequestMode:  "params-only",
		ResponseMode: "envelope",
		TimeoutMs:    120000,
	}
	savedScript.SelectorText = fmt.Sprintf("{\n  \"code\": %q\n}", createResp.LaunchCode)
	if _, err := app.AutomationScriptSave(*savedScript); err != nil {
		t.Fatalf("save mail probe public api config failed: %v", err)
	}

	var scriptsResp automationHTTPScriptsResponse
	if err := automationHTTPRequestJSON(http.MethodGet, baseURL+"/api/automation/scripts", nil, &scriptsResp); err != nil {
		t.Fatalf("list scripts via http failed: %v", err)
	}
	if !scriptsResp.OK || scriptsResp.Data.Count == 0 {
		t.Fatalf("unexpected scripts response: %+v", scriptsResp)
	}
	if !automationHTTPHasScript(scriptsResp.Data.Items, "mail-probe-script") {
		t.Fatalf("saved mail probe script missing: %+v", scriptsResp)
	}

	var runResp automationHTTPRunResponse
	runErr := automationHTTPRequestJSON(http.MethodPost, baseURL+"/api/automation/scripts/run", map[string]any{
		"scriptId": "mail-probe-script",
		"selector": map[string]any{
			"code": createResp.LaunchCode,
		},
		"params": map[string]any{
			"inboxUrl":  fixtureServer.URL,
			"timeoutMs": 45000,
		},
		"timeoutMs": 120000,
	}, &runResp)
	if runErr != nil {
		t.Fatalf("run script via http failed: %v", runErr)
	}

	parsed := make(map[string]any)
	if text := strings.TrimSpace(runResp.Data.Run.ResultText); text != "" {
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			t.Fatalf("parse run result failed: %v; result=%s", err, text)
		}
		if nested, ok := parsed["result"].(map[string]any); ok && len(nested) > 0 {
			parsed = nested
		}
	}
	if runResp.Data.Run.Status != "success" {
		var logsResp automationHTTPLaunchLogsResponse
		_ = automationHTTPRequestJSON(http.MethodGet, baseURL+"/api/launch/logs?limit=10", nil, &logsResp)
		t.Fatalf("unexpected run response: status=%s summary=%s error=%s logs=%s",
			runResp.Data.Run.Status,
			runResp.Data.Run.Summary,
			runResp.Data.Run.Error,
			automationHTTPMarshal(t, logsResp),
		)
	}

	if got := automationHTTPStringValue(parsed, "mailboxName"); got != "ChatGPT" {
		t.Fatalf("unexpected mailboxName: %q parsed=%s", got, automationHTTPMarshal(t, parsed))
	}
	if got := automationHTTPStringValue(parsed, "senderEmail"); got != "noreply@tm.openai.com" {
		t.Fatalf("unexpected senderEmail: %q parsed=%s", got, automationHTTPMarshal(t, parsed))
	}
	if got := automationHTTPStringValue(parsed, "recipientEmail"); got != "target@example.com" {
		t.Fatalf("unexpected recipientEmail: %q parsed=%s", got, automationHTTPMarshal(t, parsed))
	}
	if got := automationHTTPStringValue(parsed, "verificationCode"); got != "429792" {
		t.Fatalf("unexpected verificationCode: %q parsed=%s", got, automationHTTPMarshal(t, parsed))
	}
	if got := parsed["permissionApplied"]; got != true {
		t.Fatalf("expected permissionApplied=true, got %#v parsed=%s", got, automationHTTPMarshal(t, parsed))
	}
	if got := automationHTTPStringValue(parsed, "permissionOrigin"); got != fixtureServer.URL {
		t.Fatalf("unexpected permissionOrigin: %q parsed=%s", got, automationHTTPMarshal(t, parsed))
	}
	signature := automationHTTPStringValue(parsed, "signature")
	if !strings.Contains(signature, "Best regards") || !strings.Contains(signature, "ChatGPT") {
		t.Fatalf("unexpected signature: %q parsed=%s", signature, automationHTTPMarshal(t, parsed))
	}

	var hookResp automationHTTPHookEnvelopeResponse
	hookErr := automationHTTPRequestJSON(http.MethodPost, baseURL+"/api/automation/hooks/mail/probe-message", map[string]any{
		"params": map[string]any{
			"inboxUrl": fixtureServer.URL,
		},
		"timeoutMs": 45000,
	}, &hookResp)
	if hookErr != nil {
		t.Fatalf("run public hook via http failed: %v", hookErr)
	}
	if !hookResp.OK || hookResp.Status != "success" {
		t.Fatalf("unexpected hook response: %+v", hookResp)
	}
	if got := automationHTTPStringValue(hookResp.Result, "verificationCode"); got != "429792" {
		t.Fatalf("unexpected hook verificationCode: %q resp=%s", got, automationHTTPMarshal(t, hookResp))
	}
	if got := automationHTTPStringValue(hookResp.Result, "senderEmail"); got != "noreply@tm.openai.com" {
		t.Fatalf("unexpected hook senderEmail: %q resp=%s", got, automationHTTPMarshal(t, hookResp))
	}

	t.Logf("automation http result: %s", automationHTTPMarshal(t, map[string]any{
		"profileId":        createResp.ProfileID,
		"launchCode":       createResp.LaunchCode,
		"runStatus":        runResp.Data.Run.Status,
		"runSummary":       runResp.Data.Run.Summary,
		"hookStatus":       hookResp.Status,
		"hookSummary":      hookResp.Summary,
		"mailboxName":      automationHTTPStringValue(parsed, "mailboxName"),
		"senderEmail":      automationHTTPStringValue(parsed, "senderEmail"),
		"recipientEmail":   automationHTTPStringValue(parsed, "recipientEmail"),
		"verificationCode": automationHTTPStringValue(parsed, "verificationCode"),
		"signature":        signature,
		"subject":          automationHTTPStringValue(parsed, "subject"),
	}))
}

func lookupAutomationHTTPProbeNode(t *testing.T) string {
	t.Helper()

	const preferred = `D:\code\plugin\nodejs\node.exe`
	if _, err := os.Stat(preferred); err == nil {
		return preferred
	}
	return lookupAutomationTestNode(t)
}

func lookupAutomationHTTPProbeChrome(t *testing.T) string {
	t.Helper()

	const preferred = `C:\Program Files\Google\Chrome\Application\chrome.exe`
	if _, err := os.Stat(preferred); err == nil {
		return preferred
	}
	t.Skip("system chrome is not installed")
	return ""
}

func automationHTTPRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("resolve repo root failed")
	}
	return filepath.Dir(filepath.Dir(file))
}

func automationHTTPFreePort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port failed: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func prepareAutomationHTTPRuntime(appRoot string, repoRoot string, runtimeVersion string) error {
	repoRuntimeDir := filepath.Join(repoRoot, "data", "runtime", "automation", strings.TrimSpace(runtimeVersion))
	tempRuntimeDir := filepath.Join(appRoot, "data", "runtime", "automation", strings.TrimSpace(runtimeVersion))
	if _, err := os.Stat(repoRuntimeDir); err != nil {
		return fmt.Errorf("repo runtime not found: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(tempRuntimeDir, "node_modules"), 0o755); err != nil {
		return err
	}
	if err := automationHTTPCopyFile(
		filepath.Join(repoRuntimeDir, "runner.cjs"),
		filepath.Join(tempRuntimeDir, "runner.cjs"),
	); err != nil {
		return err
	}
	return automationHTTPCopyDir(
		filepath.Join(repoRuntimeDir, "node_modules", "playwright-core"),
		filepath.Join(tempRuntimeDir, "node_modules", "playwright-core"),
	)
}

func automationHTTPCopyFile(src string, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func automationHTTPCopyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relativePath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode())
	})
}

func automationHTTPRequestJSON(method string, url string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s returned %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode %s %s failed: %w; body=%s", method, url, err, string(raw))
	}
	return nil
}

func automationHTTPHasScript(items []struct {
	ID string `json:"id"`
}, scriptID string) bool {
	for _, item := range items {
		if strings.TrimSpace(item.ID) == scriptID {
			return true
		}
	}
	return false
}

func automationHTTPStringValue(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func automationHTTPMarshal(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal debug payload failed: %v", err)
	}
	return string(data)
}

const automationHTTPMailFixtureHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>Mail Fixture</title>
  <script>
    window.__notificationProbe = {
      supported: typeof Notification !== 'undefined',
      requested: false,
      result: '',
      error: '',
    };
    document.addEventListener('DOMContentLoaded', () => {
      if (typeof Notification === 'undefined' || typeof Notification.requestPermission !== 'function') {
        document.documentElement.setAttribute('data-notification-probe', 'unsupported');
        return;
      }
      window.__notificationProbe.requested = true;
      Notification.requestPermission()
        .then((result) => {
          window.__notificationProbe.result = String(result || '');
          document.documentElement.setAttribute('data-notification-probe', window.__notificationProbe.result || 'empty');
        })
        .catch((error) => {
          window.__notificationProbe.error = String(error && error.message ? error.message : error);
          document.documentElement.setAttribute('data-notification-probe', 'error');
        });
    });
  </script>
  <style>
    body { margin: 0; font-family: Arial, sans-serif; background: #f6f7fb; }
    main { display: flex; gap: 20px; padding: 24px; min-height: 100vh; box-sizing: border-box; }
    .sidebar { width: 32%; min-width: 320px; background: #fff; border: 1px solid #d9dce6; border-radius: 12px; padding: 20px; box-sizing: border-box; }
    .viewer { width: 60%; min-height: 420px; background: #fff; border: 1px solid #d9dce6; border-radius: 12px; padding: 24px; box-sizing: border-box; }
    input { width: 100%; height: 42px; padding: 0 12px; font-size: 16px; box-sizing: border-box; }
    [role="row"] { margin-top: 16px; min-height: 56px; border: 1px solid #c8cfdd; border-radius: 10px; padding: 16px; cursor: pointer; background: #fafbff; }
    p { margin: 0 0 12px; line-height: 1.55; }
    h1 { margin: 0 0 16px; font-size: 28px; }
  </style>
</head>
<body>
  <main>
    <section class="sidebar">
      <div role="dialog" tabindex="-1" data-focus-root="1" class="overlay no-outline" data-testid="overlay-button" id="advanced-search-overlay-14">
        <input
          type="search"
          readonly
          title="关键词"
          placeholder="搜索邮件"
          value=""
          aria-label="Search messages"
          data-testid="search-keyword"
          class="input-element w-full cursor-text"
        />
      </div>
      <div role="row">target@example.com ChatGPT verification code 429792</div>
    </section>
    <article role="article" class="viewer">
      <h1>Your ChatGPT verification code</h1>
      <p>From: ChatGPT &lt;noreply@tm.openai.com&gt;</p>
      <p>To: target@example.com</p>
      <p>Hello,</p>
      <p>Your verification code is 429792.</p>
      <p>Please use this code to continue signing in.</p>
      <p>Best regards</p>
      <p>ChatGPT</p>
    </article>
  </main>
</body>
</html>`

const automationHTTPMailProbeScriptTextRaw = `module.exports.run = async ({ launch, connect, openPage, selector, params = {} }) => {
  const normalizeText = (value) => String(value == null ? '' : value).trim()
  const timeoutMs = Number.isFinite(Number(params.timeoutMs))
    ? Math.max(5000, Math.round(Number(params.timeoutMs)))
    : 45000
  const inboxUrl = normalizeText(params.inboxUrl)

  if (!inboxUrl) {
    throw new Error('inboxUrl is required')
  }

  const session = await launch({
    selector,
    skipDefaultStartUrls: true,
    startUrls: [inboxUrl],
  })
  const connection = await connect(session, { timeoutMs })
  const browser = connection.browser
  if (!browser) {
    throw new Error('browser connection is unavailable')
  }

  const context =
    connection.context ||
    browser.contexts()[0] ||
    (typeof browser.newContext === 'function' ? await browser.newContext() : null)
  if (!context) {
    throw new Error('browser context is unavailable')
  }

  const opened = await openPage(connection, {
    url: inboxUrl,
    timeoutMs,
    permissions: ['notifications'],
  })
  const page = opened.page
  await page.waitForLoadState('networkidle', {
    timeout: Math.min(timeoutMs, 2500),
  }).catch(() => {})

  const result = await page.evaluate(() => {
    const normalizeText = (value) => String(value == null ? '' : value).replace(/\s+/g, ' ').trim()
    const article = document.querySelector('article')
    const lines = Array.from(document.querySelectorAll('article p'))
      .map((node) => normalizeText(node.textContent))
      .filter(Boolean)
    const subject = normalizeText(document.querySelector('article h1')?.textContent)
    const fromLine = lines.find((line) => line.startsWith('From:')) || ''
    const toLine = lines.find((line) => line.startsWith('To:')) || ''
    const articleText = normalizeText(article?.textContent)
    const mailboxMatch = fromLine.match(/^From:\s*([^<]+?)\s*</)
    const senderEmailMatch = fromLine.match(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i)
    const recipientEmailMatch = toLine.match(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i)
    const verificationCodeMatch = articleText.match(/\b\d{6}\b/)
    const signature = lines.slice(-2).join('\n')

    return {
      notificationPermission: typeof Notification !== 'undefined' ? Notification.permission : '',
      notificationProbe: document.documentElement.getAttribute('data-notification-probe') || '',
      mailboxName: mailboxMatch ? normalizeText(mailboxMatch[1]) : '',
      senderEmail: senderEmailMatch ? senderEmailMatch[0] : '',
      recipientEmail: recipientEmailMatch ? recipientEmailMatch[0] : '',
      subject,
      verificationCode: verificationCodeMatch ? verificationCodeMatch[0] : '',
      signature,
    }
  })

  return {
    ok: true,
    permissionApplied: opened.permissionResult && opened.permissionResult.applied === true,
    permissionOrigin: opened.permissionResult && opened.permissionResult.origin ? opened.permissionResult.origin : '',
    summary: '已提取测试邮件内容',
    ...result,
  }
}`

var automationHTTPMailProbeScriptSummaryLine = regexp.MustCompile(`summary:[^\n]+`)

var automationHTTPMailProbeScriptText = automationHTTPMailProbeScriptSummaryLine.ReplaceAllString(
	automationHTTPMailProbeScriptTextRaw,
	"summary: 'mail probe extracted message',",
)
