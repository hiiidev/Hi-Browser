package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

func TestRunScriptTaskExecutesCustomRunner(t *testing.T) {
	nodeExecPath := lookupNodeExecutable(t)

	cfg := config.DefaultConfig()
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.Automation.SystemNodePath = nodeExecPath
	cfg.Automation.NodeVersion = "test-node"
	cfg.Automation.PlaywrightCoreVersion = "1.59.0"
	cfg.Automation.RuntimeVersion = "test-runtime"

	manager := NewManager(t.TempDir(), cfg, nil, Options{})

	state := manager.CurrentState()
	if err := writeRunnerScript(state.RunnerPath); err != nil {
		t.Fatalf("write runner script failed: %v", err)
	}
	if err := writeMockPlaywrightModule(state.RuntimeDir, cfg.Automation.PlaywrightCoreVersion); err != nil {
		t.Fatalf("write mock playwright module failed: %v", err)
	}

	receivedBody := map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/launch" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"profileId": "profile-script",
			"debugPort": 9333,
			"cdpUrl":    "http://127.0.0.1:9333",
		})
	}))
	defer server.Close()

	scriptDir := filepath.Join(state.RuntimeDir, "tmp", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "script.cjs")
	scriptSource := `const fs = require('fs');

module.exports.run = async ({ launch, connect, selector, params, log, artifact }) => {
  const session = await launch({
    selector,
    startUrls: params.startUrls,
    skipDefaultStartUrls: true,
  })

  const { browser } = await connect(session)
  const context = browser.contexts()[0]
  const page = context.pages()[0] || await context.newPage()
  await page.goto(params.url, { waitUntil: 'domcontentloaded', timeout: params.timeoutMs || 30000 })

  const filePath = artifact('script-output.txt')
  fs.writeFileSync(filePath, 'artifact-ready')
  log('profile', session.profileId)

  return {
    ok: true,
    summary: '脚本执行成功',
    profileId: session.profileId,
    url: page.url(),
    artifactPath: filePath,
  }
}`
	if err := os.WriteFile(scriptPath, []byte(scriptSource), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	artifactDir := filepath.Join(t.TempDir(), "artifacts")
	result, err := manager.RunScriptTask(context.Background(), ScriptTaskRequest{
		TaskKey:       "script:test",
		ScriptPath:    scriptPath,
		Selector:      map[string]any{"code": "BUYER_001"},
		Params:        map[string]any{"url": "https://example.com/script", "startUrls": []string{"https://example.com/script"}},
		LaunchBaseURL: server.URL,
		ArtifactDir:   artifactDir,
	})
	if err != nil {
		t.Fatalf("RunScriptTask returned error: %v", err)
	}

	if !result.OK {
		t.Fatalf("expected script task to succeed, got %+v", result)
	}
	if result.Summary != "脚本执行成功" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.ResultText, `"profileId":"profile-script"`) {
		t.Fatalf("expected result text to contain profileId, got %s", result.ResultText)
	}
	if !strings.Contains(result.ResultText, `"artifactPath":"`) {
		t.Fatalf("expected result text to contain artifact path, got %s", result.ResultText)
	}

	if selector, ok := receivedBody["selector"].(map[string]any); !ok || selector["code"] != "BUYER_001" {
		t.Fatalf("unexpected selector payload: %+v", receivedBody)
	}

	artifactData, err := os.ReadFile(filepath.Join(artifactDir, "script-output.txt"))
	if err != nil {
		t.Fatalf("read script artifact failed: %v", err)
	}
	if string(artifactData) != "artifact-ready" {
		t.Fatalf("unexpected script artifact payload: %s", string(artifactData))
	}
}

func TestRunScriptTaskOpenPageCreatesFreshPageAndGrantsPermissions(t *testing.T) {
	nodeExecPath := lookupNodeExecutable(t)

	cfg := config.DefaultConfig()
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.Automation.SystemNodePath = nodeExecPath
	cfg.Automation.NodeVersion = "test-node"
	cfg.Automation.PlaywrightCoreVersion = "1.59.0"
	cfg.Automation.RuntimeVersion = "test-runtime"

	manager := NewManager(t.TempDir(), cfg, nil, Options{})

	state := manager.CurrentState()
	if err := writeRunnerScript(state.RunnerPath); err != nil {
		t.Fatalf("write runner script failed: %v", err)
	}
	if err := writeMockPlaywrightModule(state.RuntimeDir, cfg.Automation.PlaywrightCoreVersion); err != nil {
		t.Fatalf("write mock playwright module failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"profileId": "profile-script",
			"debugPort": 9333,
			"cdpUrl":    "http://127.0.0.1:9333",
		})
	}))
	defer server.Close()

	scriptDir := filepath.Join(state.RuntimeDir, "tmp", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "script-open-page.cjs")
	scriptSource := `module.exports.run = async ({ launch, connect, openPage, selector, params }) => {
  const session = await launch({
    selector,
    startUrls: [params.url],
    skipDefaultStartUrls: true,
  })

  const connection = await connect(session)
  const opened = await openPage(connection, {
    url: params.url,
    timeoutMs: params.timeoutMs || 30000,
    permissions: ['notifications'],
  })

  return {
    ok: true,
    summary: 'openPage helper ok',
    url: opened.page.url(),
    permissionApplied: opened.permissionResult.applied,
    permissionOrigin: opened.permissionResult.origin,
    permissionStrategy: opened.permissionResult.strategy || '',
    reusedPage: opened.reusedPage,
  }
}`
	if err := os.WriteFile(scriptPath, []byte(scriptSource), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	result, err := manager.RunScriptTask(context.Background(), ScriptTaskRequest{
		TaskKey:       "script:open-page",
		ScriptPath:    scriptPath,
		Selector:      map[string]any{"code": "BUYER_001"},
		Params:        map[string]any{"url": "https://example.com/inbox", "timeoutMs": 30000},
		LaunchBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("RunScriptTask returned error: %v", err)
	}

	if !result.OK {
		t.Fatalf("expected script task to succeed, got %+v", result)
	}

	parsed := map[string]any{}
	if err := json.Unmarshal([]byte(result.ResultText), &parsed); err != nil {
		t.Fatalf("parse result text failed: %v result=%s", err, result.ResultText)
	}
	if nested, ok := parsed["result"].(map[string]any); ok && len(nested) > 0 {
		parsed = nested
	}
	if parsed["permissionApplied"] != true {
		t.Fatalf("expected permissionApplied to be true, got %+v", parsed)
	}
	if parsed["permissionOrigin"] != "https://example.com" {
		t.Fatalf("unexpected permissionOrigin: %+v", parsed)
	}
	if parsed["reusedPage"] != false {
		t.Fatalf("expected reusedPage to be false, got %+v", parsed)
	}
	if parsed["url"] != "https://example.com/inbox" {
		t.Fatalf("unexpected url: %+v", parsed)
	}
}

func TestRunScriptTaskCallPageAPIUsesBrowserContext(t *testing.T) {
	nodeExecPath := lookupNodeExecutable(t)

	cfg := config.DefaultConfig()
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.Automation.SystemNodePath = nodeExecPath
	cfg.Automation.NodeVersion = "test-node"
	cfg.Automation.PlaywrightCoreVersion = "1.59.0"
	cfg.Automation.RuntimeVersion = "test-runtime"

	manager := NewManager(t.TempDir(), cfg, nil, Options{})

	state := manager.CurrentState()
	if err := writeRunnerScript(state.RunnerPath); err != nil {
		t.Fatalf("write runner script failed: %v", err)
	}
	if err := writeMockPlaywrightModule(state.RuntimeDir, cfg.Automation.PlaywrightCoreVersion); err != nil {
		t.Fatalf("write mock playwright module failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"profileId": "profile-page-api",
			"debugPort": 9333,
			"cdpUrl":    "http://127.0.0.1:9333",
		})
	}))
	defer server.Close()

	scriptDir := filepath.Join(state.RuntimeDir, "tmp", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "script-page-api.cjs")
	scriptSource := `module.exports.run = async ({ useBrowser, callPageAPI, browserFetch, selector, params }) => {
  const runtime = await useBrowser({
    selector,
    startUrls: [params.url],
    skipDefaultStartUrls: true,
    url: params.url,
    reuseCurrentPage: true,
    timeoutMs: 30000,
  })

  const created = await callPageAPI(runtime, {
    url: '/api/order/create',
    method: 'POST',
    query: {
      source: 'automation',
      tag: ['a', 'b'],
    },
    headers: {
      'X-Test': 'page-api',
    },
    json: {
      skuId: params.skuId,
      count: 2,
    },
  })
  const ping = await browserFetch(runtime.page, '/api/ping', { method: 'GET' })

  return {
    ok: true,
    summary: 'page api helper ok',
    status: created.status,
    requestUrl: created.json.url,
    method: created.json.method,
    credentials: created.json.credentials,
    contentType: created.json.headers['Content-Type'],
    testHeader: created.json.headers['X-Test'],
    requestBody: created.json.body,
    pingMethod: ping.json.method,
  }
}`
	if err := os.WriteFile(scriptPath, []byte(scriptSource), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	result, err := manager.RunScriptTask(context.Background(), ScriptTaskRequest{
		TaskKey:       "script:page-api",
		ScriptPath:    scriptPath,
		Selector:      map[string]any{"code": "BUYER_001"},
		Params:        map[string]any{"url": "https://example.com/app", "skuId": "sku-123"},
		LaunchBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("RunScriptTask returned error: %v", err)
	}

	if !result.OK {
		t.Fatalf("expected script task to succeed, got %+v", result)
	}

	parsed := map[string]any{}
	if err := json.Unmarshal([]byte(result.ResultText), &parsed); err != nil {
		t.Fatalf("parse result text failed: %v result=%s", err, result.ResultText)
	}
	if nested, ok := parsed["result"].(map[string]any); ok && len(nested) > 0 {
		parsed = nested
	}
	if parsed["status"] != float64(201) {
		t.Fatalf("unexpected status: %+v", parsed)
	}
	if parsed["method"] != "POST" || parsed["pingMethod"] != "GET" {
		t.Fatalf("unexpected methods: %+v", parsed)
	}
	if parsed["credentials"] != "include" {
		t.Fatalf("expected credentials=include, got %+v", parsed)
	}
	if parsed["contentType"] != "application/json" || parsed["testHeader"] != "page-api" {
		t.Fatalf("unexpected headers: %+v", parsed)
	}
	if !strings.Contains(fmt.Sprint(parsed["requestUrl"]), "/api/order/create?source=automation&tag=a&tag=b") {
		t.Fatalf("unexpected requestUrl: %+v", parsed)
	}
	if !strings.Contains(fmt.Sprint(parsed["requestBody"]), `"skuId":"sku-123"`) {
		t.Fatalf("unexpected requestBody: %+v", parsed)
	}
}

func TestRunScriptTaskLaunchFiltersNonLaunchParams(t *testing.T) {
	nodeExecPath := lookupNodeExecutable(t)

	cfg := config.DefaultConfig()
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.Automation.SystemNodePath = nodeExecPath
	cfg.Automation.NodeVersion = "test-node"
	cfg.Automation.PlaywrightCoreVersion = "1.59.0"
	cfg.Automation.RuntimeVersion = "test-runtime"

	manager := NewManager(t.TempDir(), cfg, nil, Options{})

	state := manager.CurrentState()
	if err := writeRunnerScript(state.RunnerPath); err != nil {
		t.Fatalf("write runner script failed: %v", err)
	}
	if err := writeMockPlaywrightModule(state.RuntimeDir, cfg.Automation.PlaywrightCoreVersion); err != nil {
		t.Fatalf("write mock playwright module failed: %v", err)
	}

	type launchRequestPayload struct {
		Code                 string         `json:"code"`
		Key                  string         `json:"key"`
		ProfileID            string         `json:"profileId"`
		ProfileName          string         `json:"profileName"`
		Keyword              string         `json:"keyword"`
		Keywords             []string       `json:"keywords"`
		Tag                  string         `json:"tag"`
		Tags                 []string       `json:"tags"`
		GroupID              string         `json:"groupId"`
		MatchMode            string         `json:"matchMode"`
		ProxyID              string         `json:"proxyId"`
		ProxyConfig          string         `json:"proxyConfig"`
		Selector             map[string]any `json:"selector"`
		LaunchArgs           []string       `json:"launchArgs"`
		StartURLs            []string       `json:"startUrls"`
		SkipDefaultStartURLs bool           `json:"skipDefaultStartUrls"`
	}

	receivedBody := launchRequestPayload{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/launch" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&receivedBody); err != nil {
			t.Fatalf("decode launch request body failed: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"profileId": "profile-script",
			"debugPort": 9333,
			"cdpUrl":    "http://127.0.0.1:9333",
		})
	}))
	defer server.Close()

	scriptDir := filepath.Join(state.RuntimeDir, "tmp", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "script-launch-filter.cjs")
	scriptSource := `module.exports.run = async ({ launch, selector, params }) => {
  const session = await launch({
    selector,
    startUrls: params.startUrls,
    skipDefaultStartUrls: true,
  })

  return {
    ok: true,
    summary: '脚本执行成功',
    profileId: session.profileId,
  }
}`
	if err := os.WriteFile(scriptPath, []byte(scriptSource), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	result, err := manager.RunScriptTask(context.Background(), ScriptTaskRequest{
		TaskKey:       "script:launch-filter",
		ScriptPath:    scriptPath,
		Selector:      map[string]any{"code": "DEMO_READY"},
		Params:        map[string]any{"url": "https://www.baidu.com", "keyword": "OpenAI", "captureScreenshot": true, "waitAfterSearchMs": 1500, "startUrls": []string{"https://www.baidu.com"}},
		LaunchBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("RunScriptTask returned error: %v", err)
	}

	if !result.OK {
		t.Fatalf("expected script task to succeed, got %+v", result)
	}
	if receivedBody.Selector["code"] != "DEMO_READY" {
		t.Fatalf("unexpected selector payload: %+v", receivedBody)
	}
	if len(receivedBody.StartURLs) != 1 || receivedBody.StartURLs[0] != "https://www.baidu.com" {
		t.Fatalf("unexpected startUrls payload: %+v", receivedBody.StartURLs)
	}
	if !receivedBody.SkipDefaultStartURLs {
		t.Fatalf("expected skipDefaultStartUrls to be true")
	}
	if receivedBody.Keyword != "" {
		t.Fatalf("expected non-launch params to be filtered, got keyword=%q", receivedBody.Keyword)
	}
	if receivedBody.ProxyID != "" || receivedBody.ProxyConfig != "" {
		t.Fatalf("expected proxy launch params to be empty, got %+v", receivedBody)
	}
}

func TestRunScriptTaskLaunchPassesTemporaryProxyParams(t *testing.T) {
	nodeExecPath := lookupNodeExecutable(t)

	cfg := config.DefaultConfig()
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.Automation.SystemNodePath = nodeExecPath
	cfg.Automation.NodeVersion = "test-node"
	cfg.Automation.PlaywrightCoreVersion = "1.59.0"
	cfg.Automation.RuntimeVersion = "test-runtime"

	manager := NewManager(t.TempDir(), cfg, nil, Options{})

	state := manager.CurrentState()
	if err := writeRunnerScript(state.RunnerPath); err != nil {
		t.Fatalf("write runner script failed: %v", err)
	}
	if err := writeMockPlaywrightModule(state.RuntimeDir, cfg.Automation.PlaywrightCoreVersion); err != nil {
		t.Fatalf("write mock playwright module failed: %v", err)
	}

	type launchRequestPayload struct {
		ProxyID              string `json:"proxyId"`
		ProxyConfig          string `json:"proxyConfig"`
		SkipDefaultStartURLs bool   `json:"skipDefaultStartUrls"`
	}

	receivedBody := launchRequestPayload{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/launch" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&receivedBody); err != nil {
			t.Fatalf("decode launch request body failed: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"profileId": "profile-script",
			"debugPort": 9333,
			"cdpUrl":    "http://127.0.0.1:9333",
		})
	}))
	defer server.Close()

	scriptDir := filepath.Join(state.RuntimeDir, "tmp", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "script-launch-proxy.cjs")
	scriptSource := `module.exports.run = async ({ launch }) => {
  await launch({
    proxyId: 'proxy-picked',
    proxyConfig: 'socks5://127.0.0.1:1080',
    skipDefaultStartUrls: true,
  })

  return {
    ok: true,
    summary: '脚本执行成功',
  }
}`
	if err := os.WriteFile(scriptPath, []byte(scriptSource), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	result, err := manager.RunScriptTask(context.Background(), ScriptTaskRequest{
		TaskKey:       "script:launch-proxy",
		ScriptPath:    scriptPath,
		LaunchBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("RunScriptTask returned error: %v", err)
	}

	if !result.OK {
		t.Fatalf("expected script task to succeed, got %+v", result)
	}
	if receivedBody.ProxyID != "proxy-picked" {
		t.Fatalf("expected proxyId to be forwarded, got %+v", receivedBody)
	}
	if receivedBody.ProxyConfig != "socks5://127.0.0.1:1080" {
		t.Fatalf("expected proxyConfig to be forwarded, got %+v", receivedBody)
	}
	if !receivedBody.SkipDefaultStartURLs {
		t.Fatalf("expected skipDefaultStartUrls to stay true, got %+v", receivedBody)
	}
}

func TestRunScriptTaskFallsBackToLaunchBaseURLWhenSessionEndpointIsInvalid(t *testing.T) {
	nodeExecPath := lookupNodeExecutable(t)

	cfg := config.DefaultConfig()
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.Automation.SystemNodePath = nodeExecPath
	cfg.Automation.NodeVersion = "test-node"
	cfg.Automation.PlaywrightCoreVersion = "1.59.0"
	cfg.Automation.RuntimeVersion = "test-runtime"

	manager := NewManager(t.TempDir(), cfg, nil, Options{})

	state := manager.CurrentState()
	if err := writeRunnerScript(state.RunnerPath); err != nil {
		t.Fatalf("write runner script failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/launch" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":         true,
			"profileId":  "profile-script",
			"debugPort":  0,
			"debugReady": false,
			"cdpUrl":     "http://127.0.0.1:0",
		})
	}))
	defer server.Close()

	if err := writeMockPlaywrightModuleWithExpectedEndpoint(state.RuntimeDir, cfg.Automation.PlaywrightCoreVersion, server.URL); err != nil {
		t.Fatalf("write mock playwright module failed: %v", err)
	}

	scriptDir := filepath.Join(state.RuntimeDir, "tmp", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "script-fallback.cjs")
	scriptSource := `module.exports.run = async ({ launch, connect, selector }) => {
  const session = await launch({ selector })
  const connection = await connect(session)

  return {
    ok: true,
    summary: '脚本已通过 Launch 地址回退连接',
    connectedEndpoint: connection.session.cdpUrl,
    profileId: session.profileId,
  }
}`
	if err := os.WriteFile(scriptPath, []byte(scriptSource), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	result, err := manager.RunScriptTask(context.Background(), ScriptTaskRequest{
		TaskKey:       "script:fallback",
		ScriptPath:    scriptPath,
		Selector:      map[string]any{"code": "DEMO_READY"},
		LaunchBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("RunScriptTask returned error: %v", err)
	}

	if !result.OK {
		t.Fatalf("expected script task to succeed, got %+v", result)
	}
	if result.Summary != "脚本已通过 Launch 地址回退连接" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if !strings.Contains(result.ResultText, `"connectedEndpoint":"`+server.URL+`"`) {
		t.Fatalf("expected result text to contain fallback endpoint, got %s", result.ResultText)
	}
}

func TestRunScriptTaskClosesBrowserConnections(t *testing.T) {
	nodeExecPath := lookupNodeExecutable(t)

	cfg := config.DefaultConfig()
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.Automation.SystemNodePath = nodeExecPath
	cfg.Automation.NodeVersion = "test-node"
	cfg.Automation.PlaywrightCoreVersion = "1.59.0"
	cfg.Automation.RuntimeVersion = "test-runtime"

	manager := NewManager(t.TempDir(), cfg, nil, Options{})

	state := manager.CurrentState()
	if err := writeRunnerScript(state.RunnerPath); err != nil {
		t.Fatalf("write runner script failed: %v", err)
	}
	if err := writeMockPlaywrightModuleWithPersistentConnection(state.RuntimeDir, cfg.Automation.PlaywrightCoreVersion, ""); err != nil {
		t.Fatalf("write mock playwright module failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"profileId": "profile-script-close",
			"debugPort": 9333,
			"cdpUrl":    "http://127.0.0.1:9333",
		})
	}))
	defer server.Close()

	scriptDir := filepath.Join(state.RuntimeDir, "tmp", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "script-close.cjs")
	scriptSource := `module.exports.run = async ({ launch, connect, selector }) => {
  const session = await launch({ selector })
  const connection = await connect(session)

  return {
    ok: true,
    summary: '脚本执行成功',
    connectedEndpoint: connection.session.cdpUrl,
  }
}`
	if err := os.WriteFile(scriptPath, []byte(scriptSource), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := manager.RunScriptTask(ctx, ScriptTaskRequest{
		TaskKey:       "script:close",
		ScriptPath:    scriptPath,
		Selector:      map[string]any{"code": "DEMO_READY"},
		LaunchBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("RunScriptTask returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected script task to succeed, got %+v", result)
	}
}

func TestRunScriptTaskConnectHonorsPerCallTimeout(t *testing.T) {
	nodeExecPath := lookupNodeExecutable(t)

	cfg := config.DefaultConfig()
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.Automation.SystemNodePath = nodeExecPath
	cfg.Automation.NodeVersion = "test-node"
	cfg.Automation.PlaywrightCoreVersion = "1.59.0"
	cfg.Automation.RuntimeVersion = "test-runtime"

	manager := NewManager(t.TempDir(), cfg, nil, Options{})

	state := manager.CurrentState()
	if err := writeRunnerScript(state.RunnerPath); err != nil {
		t.Fatalf("write runner script failed: %v", err)
	}
	if err := writeMockPlaywrightModuleWithExpectedConnectTimeout(state.RuntimeDir, cfg.Automation.PlaywrightCoreVersion, 47000); err != nil {
		t.Fatalf("write mock playwright module failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"profileId": "profile-timeout",
			"debugPort": 9333,
			"cdpUrl":    "http://127.0.0.1:9333",
		})
	}))
	defer server.Close()

	scriptDir := filepath.Join(state.RuntimeDir, "tmp", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "script-connect-timeout.cjs")
	scriptSource := `module.exports.run = async ({ launch, connect, selector }) => {
  const session = await launch({ selector })
  const connection = await connect(session, { timeoutMs: 47000 })

  return {
    ok: true,
    summary: '脚本执行成功',
    connectedEndpoint: connection.session.cdpUrl,
  }
}`
	if err := os.WriteFile(scriptPath, []byte(scriptSource), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	result, err := manager.RunScriptTask(context.Background(), ScriptTaskRequest{
		TaskKey:       "script:connect-timeout",
		ScriptPath:    scriptPath,
		Selector:      map[string]any{"code": "DEMO_READY"},
		LaunchBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("RunScriptTask returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected script task to succeed, got %+v", result)
	}
}

func TestRunScriptTaskTerminatesHungScriptOnTimeout(t *testing.T) {
	nodeExecPath := lookupNodeExecutable(t)

	cfg := config.DefaultConfig()
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.Automation.SystemNodePath = nodeExecPath
	cfg.Automation.NodeVersion = "test-node"
	cfg.Automation.PlaywrightCoreVersion = "1.59.0"
	cfg.Automation.RuntimeVersion = "test-runtime"

	manager := NewManager(t.TempDir(), cfg, nil, Options{})

	state := manager.CurrentState()
	if err := writeRunnerScript(state.RunnerPath); err != nil {
		t.Fatalf("write runner script failed: %v", err)
	}
	if err := writeMockPlaywrightModule(state.RuntimeDir, cfg.Automation.PlaywrightCoreVersion); err != nil {
		t.Fatalf("write mock playwright module failed: %v", err)
	}

	scriptDir := filepath.Join(state.RuntimeDir, "tmp", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "script-timeout.cjs")
	scriptSource := `module.exports.run = async () => {
  await new Promise(() => setInterval(() => {}, 1000))
}`
	if err := os.WriteFile(scriptPath, []byte(scriptSource), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	startedAt := time.Now()
	_, err := manager.RunScriptTask(context.Background(), ScriptTaskRequest{
		TaskKey:       "script:timeout",
		ScriptPath:    scriptPath,
		LaunchBaseURL: "http://127.0.0.1",
		Timeout:       150 * time.Millisecond,
	})
	elapsed := time.Since(startedAt)
	if err == nil {
		t.Fatalf("expected RunScriptTask to fail on timeout")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("expected timeout to terminate quickly, took %s", elapsed)
	}

	manager.mu.Lock()
	activeTaskCount := len(manager.activeTasks)
	profileTaskCount := len(manager.profileTask)
	manager.mu.Unlock()
	if activeTaskCount != 0 || profileTaskCount != 0 {
		t.Fatalf("expected timed out task to be unregistered, active=%d profile=%d", activeTaskCount, profileTaskCount)
	}
}

func lookupNodeExecutable(t *testing.T) string {
	t.Helper()

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node is not available: %v", err)
	}

	cmd := exec.Command(nodePath, "-p", "process.execPath")
	output, err := cmd.Output()
	if err != nil {
		return nodePath
	}

	resolved := strings.TrimSpace(string(output))
	if resolved == "" {
		return nodePath
	}
	return resolved
}

func writeMockPlaywrightModule(runtimeDir, version string) error {
	return writeMockPlaywrightModuleWithExpectedEndpoint(runtimeDir, version, "")
}

func writeMockPlaywrightModuleWithExpectedEndpoint(runtimeDir, version, expectedEndpoint string) error {
	return writeMockPlaywrightModuleWithOptions(runtimeDir, version, expectedEndpoint, false)
}

func writeMockPlaywrightModuleWithPersistentConnection(runtimeDir, version, expectedEndpoint string) error {
	return writeMockPlaywrightModuleWithOptions(runtimeDir, version, expectedEndpoint, true)
}

func writeMockPlaywrightModuleWithExpectedConnectTimeout(runtimeDir, version string, expectedConnectTimeout int) error {
	moduleDir := filepath.Join(runtimeDir, "node_modules", "playwright-core")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		return err
	}

	packageJSON := fmt.Sprintf("{\"name\":\"playwright-core\",\"version\":\"%s\",\"main\":\"index.js\"}", version)
	if err := os.WriteFile(filepath.Join(moduleDir, "package.json"), []byte(packageJSON), 0o644); err != nil {
		return err
	}

	indexJS := fmt.Sprintf(`const expectedConnectTimeout = %d;

const context = {
  async grantPermissions() {},
  async newPage() {
    return {
      async goto() {},
      async bringToFront() {},
      async waitForLoadState() {},
      async waitForTimeout() {},
      async close() {},
      isClosed() {
        return false;
      },
      async title() {
        return 'Mock Page Title';
      },
      url() {
        return 'about:blank';
      },
    };
  },
  pages() {
    return [];
  },
};

exports.chromium = {
  async connectOverCDP(endpoint, options = {}) {
    if (options.timeout !== expectedConnectTimeout) {
      throw new Error('unexpected connect timeout: ' + String(options.timeout));
    }
    return {
      contexts() {
        return [context];
      },
      async close() {},
    };
  },
};
`, expectedConnectTimeout)
	return os.WriteFile(filepath.Join(moduleDir, "index.js"), []byte(indexJS), 0o644)
}

func writeMockPlaywrightModuleWithOptions(runtimeDir, version, expectedEndpoint string, persistentConnection bool) error {
	moduleDir := filepath.Join(runtimeDir, "node_modules", "playwright-core")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		return err
	}

	packageJSON := fmt.Sprintf("{\"name\":\"playwright-core\",\"version\":\"%s\",\"main\":\"index.js\"}", version)
	if err := os.WriteFile(filepath.Join(moduleDir, "package.json"), []byte(packageJSON), 0o644); err != nil {
		return err
	}

	expectedEndpointJSON, err := json.Marshal(expectedEndpoint)
	if err != nil {
		return err
	}
	persistentConnectionJSON, err := json.Marshal(persistentConnection)
	if err != nil {
		return err
	}

	indexJS := fmt.Sprintf(`const fs = require('fs');

const expectedEndpoint = %s;
const persistentConnection = %s;

function createPage() {
  let currentURL = 'about:blank';
  return {
    async goto(url) {
      currentURL = url;
    },
    async bringToFront() {},
    async waitForLoadState() {},
    async waitForTimeout() {},
    async screenshot(options) {
      fs.writeFileSync(options.path, 'mock-screenshot');
    },
    async evaluate(fn, arg) {
      const previousFetch = global.fetch;
      global.fetch = async (url, init = {}) => {
        return {
          ok: String(init.method || 'GET').toUpperCase() !== 'DELETE',
          status: String(init.method || 'GET').toUpperCase() === 'POST' ? 201 : 200,
          statusText: String(init.method || 'GET').toUpperCase() === 'DELETE' ? 'Forbidden' : 'OK',
          url: String(url),
          headers: {
            forEach(callback) {
              callback('application/json', 'content-type');
            },
          },
          async text() {
            return JSON.stringify({
              ok: true,
              url: String(url),
              method: String(init.method || 'GET').toUpperCase(),
              credentials: init.credentials || '',
              headers: init.headers || {},
              body: init.body || '',
            });
          },
        };
      };
      try {
        return await fn(arg);
      } finally {
        global.fetch = previousFetch;
      }
    },
    async title() {
      return 'Mock Page Title';
    },
    url() {
      return currentURL;
    },
    isClosed() {
      return false;
    },
    async close() {},
  };
}

const context = {
  async grantPermissions() {},
  async newPage() {
    return createPage();
  },
  pages() {
    return [];
  },
};

exports.chromium = {
  async connectOverCDP(endpoint) {
    if (String(endpoint).includes(':0')) {
      throw new Error('invalid cdp endpoint');
    }
    if (expectedEndpoint && endpoint !== expectedEndpoint) {
      throw new Error('unexpected cdp endpoint: ' + endpoint);
    }
    const hold = persistentConnection ? setInterval(() => {}, 1000) : null;
    return {
      contexts() {
        return [context];
      },
      async close() {
        if (hold) {
          clearInterval(hold);
        }
      },
    };
  },
};
`, string(expectedEndpointJSON), string(persistentConnectionJSON))
	return os.WriteFile(filepath.Join(moduleDir, "index.js"), []byte(indexJS), 0o644)
}
