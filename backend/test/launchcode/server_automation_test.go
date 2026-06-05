package launchcode_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"ant-chrome/backend/internal/automation"
)

type mockAutomationStarter struct {
	*mockStarterWithParams
	scripts          []automation.ScriptRecord
	runs             []automation.ScriptRunRecord
	lastGetID        string
	lastRunRequest   automation.ScriptRunRequest
	lastRunListLimit int
	runResult        *automation.ScriptRunRecord
	getErr           error
	listErr          error
	runErr           error
	runListErr       error
}

func newMockAutomationStarter() *mockAutomationStarter {
	return &mockAutomationStarter{
		mockStarterWithParams: newMockStarterWithParams(),
	}
}

func (m *mockAutomationStarter) AutomationScriptList() ([]automation.ScriptRecord, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return append([]automation.ScriptRecord(nil), m.scripts...), nil
}

func (m *mockAutomationStarter) AutomationScriptGet(scriptID string) (*automation.ScriptRecord, error) {
	m.lastGetID = scriptID
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, item := range m.scripts {
		if item.ID == scriptID {
			record := item
			return &record, nil
		}
	}
	return nil, os.ErrNotExist
}

func (m *mockAutomationStarter) AutomationScriptRunWithOptions(input automation.ScriptRunRequest) (*automation.ScriptRunRecord, error) {
	m.lastRunRequest = input
	if m.runErr != nil {
		return nil, m.runErr
	}
	if m.runResult == nil {
		return &automation.ScriptRunRecord{
			ID:       "run-default",
			ScriptID: input.ScriptID,
			Status:   "success",
		}, nil
	}

	record := *m.runResult
	return &record, nil
}

func (m *mockAutomationStarter) AutomationScriptRunList(limit int) ([]automation.ScriptRunRecord, error) {
	m.lastRunListLimit = limit
	if m.runListErr != nil {
		return nil, m.runListErr
	}

	items := append([]automation.ScriptRunRecord(nil), m.runs...)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func TestAutomationScriptsEndpointReturnsMetadata(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.scripts = []automation.ScriptRecord{
		{
			ID:           "news-query-txt",
			Name:         "查询新闻并写 TXT",
			Description:  "测试脚本",
			Type:         "playwright-cdp",
			Status:       "ready",
			EntryFile:    "index.cjs",
			Tags:         []string{"Playwright", "新闻"},
			SelectorText: `{"code":"BUYER_001"}`,
			ParamsText:   `{"keyword":"OpenAI","limit":10}`,
			ScriptText:   `module.exports.run = async () => ({ ok: true })`,
			Notes:        "note",
			CreatedAt:    "2026-04-08T10:00:00Z",
			UpdatedAt:    "2026-04-08T11:00:00Z",
		},
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/automation/scripts", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "scriptText") {
		t.Fatalf("公共脚本列表不应返回脚本文本: %s", w.Body.String())
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Count int `json:"count"`
			Items []struct {
				ID       string                 `json:"id"`
				Type     string                 `json:"type"`
				Status   string                 `json:"status"`
				Selector map[string]interface{} `json:"selector"`
				Params   map[string]interface{} `json:"params"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.OK || resp.Data.Count != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("响应结构错误: %+v", resp)
	}
	item := resp.Data.Items[0]
	if item.ID != "news-query-txt" || item.Type != "playwright-cdp" || item.Status != "ready" {
		t.Fatalf("脚本元数据错误: %+v", item)
	}
	if item.Selector["code"] != "BUYER_001" {
		t.Fatalf("selector 解析错误: %+v", item.Selector)
	}
	if item.Params["keyword"] != "OpenAI" {
		t.Fatalf("params 解析错误: %+v", item.Params)
	}
}

func TestAutomationScriptDetailEndpointReturnsSingleScript(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.scripts = []automation.ScriptRecord{
		{
			PackageFormat:   "ant-automation-script",
			ManifestVersion: 1,
			ID:              "news-query-txt",
			Name:            "查询新闻并写 TXT",
			Description:     "测试脚本",
			Type:            "playwright-cdp",
			Status:          "ready",
			EntryFile:       "index.cjs",
			Tags:            []string{"Playwright", "新闻"},
			SelectorText:    `{"code":"BUYER_001"}`,
			ParamsText:      `{"keyword":"OpenAI","limit":10}`,
			ScriptText:      `module.exports.run = async () => ({ ok: true })`,
			Notes:           "note",
			Source: automation.ScriptSource{
				Type: "git",
				URI:  "https://example.com/repo.git",
				Ref:  "main",
			},
			CreatedAt: "2026-04-08T10:00:00Z",
			UpdatedAt: "2026-04-08T11:00:00Z",
		},
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/automation/scripts/news-query-txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if starter.lastGetID != "news-query-txt" {
		t.Fatalf("scriptId 路径解析错误: %s", starter.lastGetID)
	}
	if strings.Contains(w.Body.String(), "scriptText") {
		t.Fatalf("公共脚本详情不应返回脚本文本: %s", w.Body.String())
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Item struct {
				ID              string                  `json:"id"`
				PackageFormat   string                  `json:"packageFormat"`
				ManifestVersion int                     `json:"manifestVersion"`
				Source          automation.ScriptSource `json:"source"`
				Selector        map[string]interface{}  `json:"selector"`
			} `json:"item"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	item := resp.Data.Item
	if !resp.OK || item.ID != "news-query-txt" {
		t.Fatalf("详情响应错误: %+v", resp)
	}
	if item.PackageFormat != "ant-automation-script" || item.ManifestVersion != 1 {
		t.Fatalf("详情元数据错误: %+v", item)
	}
	if item.Source.Type != "git" || item.Source.URI != "https://example.com/repo.git" {
		t.Fatalf("source 返回错误: %+v", item.Source)
	}
	if item.Selector["code"] != "BUYER_001" {
		t.Fatalf("selector 解析错误: %+v", item.Selector)
	}
}

func TestAutomationScriptDetailEndpointReturnsNotFound(t *testing.T) {
	handler := buildTestHandlerWithManager(newInMemoryService(), newMockAutomationStarter(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/automation/scripts/missing-script", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("期望 404，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "script not found") {
		t.Fatalf("错误信息不正确: %s", w.Body.String())
	}
}

func TestAutomationScriptRunEndpointConvertsObjectPayload(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.runResult = &automation.ScriptRunRecord{
		ID:         "run-1",
		ScriptID:   "news-query-txt",
		Status:     "success",
		ResultText: `{"ok":true,"summary":"done","result":{"subject":"Hello","contentText":"Mail body"}}`,
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/automation/scripts/run", bytes.NewBufferString(`{
		"scriptId":"news-query-txt",
		"selector":{"code":"BUYER_001"},
		"params":{"keyword":"OpenAI"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if starter.lastRunRequest.ScriptID != "news-query-txt" {
		t.Fatalf("scriptId 传递错误: %+v", starter.lastRunRequest)
	}
	if starter.lastRunRequest.UseScriptSelector || starter.lastRunRequest.UseScriptParams {
		t.Fatalf("对象参数应关闭脚本默认 selector/params: %+v", starter.lastRunRequest)
	}
	if starter.lastRunRequest.SelectorText != `{"code":"BUYER_001"}` {
		t.Fatalf("selectorText 转换错误: %s", starter.lastRunRequest.SelectorText)
	}
	if starter.lastRunRequest.ParamsText != `{"keyword":"OpenAI"}` {
		t.Fatalf("paramsText 转换错误: %s", starter.lastRunRequest.ParamsText)
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Result map[string]interface{} `json:"result"`
			Run    struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"run"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.OK || resp.Data.Run.ID != "run-1" || resp.Data.Run.Status != "success" {
		t.Fatalf("run 响应错误: %+v", resp)
	}
	if resp.Data.Result["subject"] != "Hello" || resp.Data.Result["contentText"] != "Mail body" {
		t.Fatalf("expected parsed result payload, got %+v", resp.Data.Result)
	}
}

func TestAutomationScriptRunEndpointUsesScriptDefaultsWhenFieldsOmitted(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/automation/scripts/run", bytes.NewBufferString(`{"scriptId":"news-query-txt"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !starter.lastRunRequest.UseScriptSelector || !starter.lastRunRequest.UseScriptParams {
		t.Fatalf("缺省时应回退到脚本默认 selector/params: %+v", starter.lastRunRequest)
	}
	if starter.lastRunRequest.SelectorText != "" || starter.lastRunRequest.ParamsText != "" {
		t.Fatalf("缺省时不应透传 selectorText/paramsText: %+v", starter.lastRunRequest)
	}
}

func TestAutomationPublicHookStandardModeReturnsEnvelope(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.scripts = []automation.ScriptRecord{
		{
			ID:   "proton-mail-first-message",
			Name: "Proton 邮件搜索并读取最新邮件",
			PublicAPI: automation.ScriptPublicAPIConfig{
				Enabled:      true,
				Method:       "POST",
				Path:         "mail/proton-first-message",
				RequestMode:  "standard",
				ResponseMode: "envelope",
				TimeoutMs:    120000,
			},
		},
	}
	starter.runResult = &automation.ScriptRunRecord{
		ID:         "run-hook-1",
		ScriptID:   "proton-mail-first-message",
		ScriptName: "Proton 邮件搜索并读取最新邮件",
		Status:     "success",
		Summary:    "已返回最新命中邮件内容",
		ResultText: `{"ok":true,"result":{"verificationCode":"429792","recipientEmail":"target@example.com"}}`,
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/automation/hooks/mail/proton-first-message", bytes.NewBufferString(`{
		"code":"BUYER_001",
		"params":{"recipientQuery":"target@example.com"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if starter.lastRunRequest.ScriptID != "proton-mail-first-message" {
		t.Fatalf("scriptId 透传错误: %+v", starter.lastRunRequest)
	}
	if starter.lastRunRequest.UseScriptSelector || starter.lastRunRequest.UseScriptParams {
		t.Fatalf("公共 Hook 应使用请求里的 code/param: %+v", starter.lastRunRequest)
	}
	if starter.lastRunRequest.SelectorText != `{"code":"BUYER_001"}` {
		t.Fatalf("selectorText 转换错误: %s", starter.lastRunRequest.SelectorText)
	}
	if starter.lastRunRequest.ParamsText != `{"recipientQuery":"target@example.com"}` {
		t.Fatalf("paramsText 转换错误: %s", starter.lastRunRequest.ParamsText)
	}

	var resp struct {
		OK     bool                   `json:"ok"`
		Status string                 `json:"status"`
		Data   map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.OK || resp.Status != "success" {
		t.Fatalf("hook 响应错误: %+v", resp)
	}
	if resp.Data["verificationCode"] != "429792" {
		t.Fatalf("expected data payload, got %+v", resp.Data)
	}
}

func TestAutomationPublicHookParamsOnlyModeReturnsResultOnly(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.scripts = []automation.ScriptRecord{
		{
			ID:   "proton-mail-first-message",
			Name: "Proton 邮件搜索并读取最新邮件",
			PublicAPI: automation.ScriptPublicAPIConfig{
				Enabled:      true,
				Method:       "POST",
				Path:         "mail/proton-result-only",
				RequestMode:  "params-only",
				ResponseMode: "result-only",
				TimeoutMs:    45000,
			},
		},
	}
	starter.runResult = &automation.ScriptRunRecord{
		ID:         "run-hook-2",
		ScriptID:   "proton-mail-first-message",
		ScriptName: "Proton 邮件搜索并读取最新邮件",
		Status:     "success",
		Summary:    "已返回最新命中邮件内容",
		ResultText: `{"ok":true,"result":{"verificationCode":"429792","mailboxName":"ChatGPT"}}`,
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/automation/hooks/mail/proton-result-only?timeoutMs=60000", bytes.NewBufferString(`{
		"code":"BUYER_001",
		"params":{"recipientQuery":"target@example.com"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if starter.lastRunRequest.UseScriptSelector || starter.lastRunRequest.UseScriptParams {
		t.Fatalf("公共 Hook 应透传 code/param: %+v", starter.lastRunRequest)
	}
	if starter.lastRunRequest.ParamsText != `{"recipientQuery":"target@example.com"}` {
		t.Fatalf("paramsText 转换错误: %s", starter.lastRunRequest.ParamsText)
	}
	if starter.lastRunRequest.TimeoutMs != 60000 {
		t.Fatalf("timeoutMs 透传错误: %+v", starter.lastRunRequest)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok || data["verificationCode"] != "429792" || data["mailboxName"] != "ChatGPT" {
		t.Fatalf("expected data payload, got %+v", resp)
	}
}

func TestAutomationPublicHookResultOnlyCompactsDownloadFields(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.scripts = []automation.ScriptRecord{
		{
			ID:   "grok-image-generate-download",
			Name: "Grok 生成图片并下载",
			PublicAPI: automation.ScriptPublicAPIConfig{
				Enabled:      true,
				Method:       "POST",
				Path:         "image/grok-generate-download",
				RequestMode:  "params-only",
				ResponseMode: "result-only",
				TimeoutMs:    300000,
			},
		},
	}
	starter.runResult = &automation.ScriptRunRecord{
		ID:         "run-grok-image",
		ScriptID:   "grok-image-generate-download",
		ScriptName: "Grok 生成图片并下载",
		Status:     "success",
		Summary:    "Grok 图片已生成并下载",
		ResultText: `{"ok":true,"downloadAddress":"D:/tmp/grok.png","downloadPath":"D:/tmp/grok.png","sourceImageUrl":"https://example.com/image.png","steps":[{"step":"open"}],"startedAt":"2026-06-03T00:00:00Z"}`,
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/automation/hooks/image/grok-generate-download", bytes.NewBufferString(`{"code":"BUYER_001","params":{"prompt":"ant"}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok || data["downloadAddress"] != "D:/tmp/grok.png" || data["downloadPath"] != "D:/tmp/grok.png" || data["sourceImageUrl"] != "https://example.com/image.png" {
		t.Fatalf("下载字段缺失: %+v", resp)
	}
	if _, exists := data["steps"]; exists {
		t.Fatalf("不应返回冗余 steps: %+v", resp)
	}
	if _, exists := data["runId"]; exists {
		t.Fatalf("不应返回 runId: %+v", resp)
	}
}

func TestAutomationPublicHookAppliesRequestBodyVariables(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.scripts = []automation.ScriptRecord{
		{
			ID:         "image-generate",
			Name:       "图片生成",
			ParamsText: `{"prompt":"默认提示词","selectors":{"promptInput":"#prompt-textarea","generatedImage":"img.generated"},"timeoutMs":300000}`,
			PublicAPI: automation.ScriptPublicAPIConfig{
				Enabled:         true,
				Method:          "POST",
				Path:            "image/generate",
				RequestMode:     "standard",
				ResponseMode:    "envelope",
				TimeoutMs:       300000,
				RequestBodyText: `{"code":"{{code}}","params":{"prompt":"{{prompt}}","outputFileName":"{{outputFileName}}"}}`,
				Variables: []automation.ScriptPublicAPIVariable{
					{Name: "prompt", DefaultValue: "默认提示词", Required: true},
					{Name: "outputFileName", DefaultValue: "generated-image.png"},
				},
			},
		},
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/automation/hooks/image/generate", bytes.NewBufferString(`{
		"code":"BUYER_001",
		"params":{"prompt":"海边的机器人","outputFileName":"robot.png"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if starter.lastRunRequest.UseScriptParams {
		t.Fatalf("变量模板应生成 params: %+v", starter.lastRunRequest)
	}
	if starter.lastRunRequest.SelectorText != `{"code":"BUYER_001"}` {
		t.Fatalf("变量模板应生成 selector: %+v", starter.lastRunRequest)
	}

	var params map[string]interface{}
	if err := json.Unmarshal([]byte(starter.lastRunRequest.ParamsText), &params); err != nil {
		t.Fatalf("解析 paramsText 失败: %v", err)
	}
	if params["prompt"] != "海边的机器人" || params["outputFileName"] != "robot.png" {
		t.Fatalf("变量未映射到 params: %+v", params)
	}
	selectors, ok := params["selectors"].(map[string]interface{})
	if !ok || selectors["promptInput"] != "#prompt-textarea" || selectors["generatedImage"] != "img.generated" {
		t.Fatalf("默认 selectors 不应被变量模板覆盖丢失: %+v", params)
	}
	if params["timeoutMs"] != float64(300000) {
		t.Fatalf("默认 timeoutMs 不应被变量模板覆盖丢失: %+v", params)
	}
	if starter.lastRunRequest.TimeoutMs != 300000 {
		t.Fatalf("timeoutMs 错误: %+v", starter.lastRunRequest)
	}
}

func TestAutomationPublicHookReturnsNotFoundWhenDisabled(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.scripts = []automation.ScriptRecord{
		{
			ID:   "disabled-hook",
			Name: "Disabled Hook",
			PublicAPI: automation.ScriptPublicAPIConfig{
				Enabled: false,
				Path:    "mail/disabled-hook",
			},
		},
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/automation/hooks/mail/disabled-hook", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("期望 404，实际 %d，body=%s", w.Code, w.Body.String())
	}
}

func TestAutomationPublicHookRejectsLegacyParamAndTopLevelVariables(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.scripts = []automation.ScriptRecord{
		{
			ID:   "strict-hook",
			Name: "Strict Hook",
			PublicAPI: automation.ScriptPublicAPIConfig{
				Enabled:   true,
				Method:    "POST",
				Path:      "mail/strict-hook",
				TimeoutMs: 120000,
			},
		},
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	cases := []struct {
		name string
		body string
	}{
		{name: "legacy-param", body: `{"code":"BUYER_001","param":{"recipientQuery":"target@example.com"}}`},
		{name: "top-level-variable", body: `{"code":"BUYER_001","recipientQuery":"target@example.com"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/automation/hooks/mail/strict-hook", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("期望 400，实际 %d，body=%s", w.Code, w.Body.String())
			}

			var resp struct {
				OK    bool `json:"ok"`
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("解析响应失败: %v", err)
			}
			if resp.OK || resp.Error.Code != "invalid_request" {
				t.Fatalf("错误 envelope 不正确: %+v", resp)
			}
		})
	}
}

func TestAutomationPublicHookRejectsInvalidTimeout(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.scripts = []automation.ScriptRecord{
		{
			ID:   "strict-hook-timeout",
			Name: "Strict Hook Timeout",
			PublicAPI: automation.ScriptPublicAPIConfig{
				Enabled:   true,
				Method:    "POST",
				Path:      "mail/strict-hook-timeout",
				TimeoutMs: 120000,
			},
		},
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/automation/hooks/mail/strict-hook-timeout", bytes.NewBufferString(`{"code":"BUYER_001","params":{},"timeoutMs":999}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "timeoutMs must be between 1000 and 1800000") {
		t.Fatalf("错误信息不正确: %s", w.Body.String())
	}
}

func TestAutomationScriptRunsEndpointPassesLimit(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	starter.runs = []automation.ScriptRunRecord{
		{ID: "run-1", ScriptID: "script-a", Status: "success"},
		{ID: "run-2", ScriptID: "script-b", Status: "failed"},
	}

	handler := buildTestHandlerWithManager(svc, starter, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/automation/scripts/runs?limit=1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if starter.lastRunListLimit != 1 {
		t.Fatalf("limit 透传错误: %d", starter.lastRunListLimit)
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Count int                          `json:"count"`
			Items []automation.ScriptRunRecord `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.OK || resp.Data.Count != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("runs 响应错误: %+v", resp)
	}
}

func TestAutomationScriptAPIUnavailableReturnsServiceUnavailable(t *testing.T) {
	handler := buildTestHandlerWithManager(newInMemoryService(), newMockStarterWithParams(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/automation/scripts", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("期望 503，实际 %d，body=%s", w.Code, w.Body.String())
	}
}

func TestAutomationScriptRunEndpointRejectsInvalidBody(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockAutomationStarter()
	handler := buildTestHandlerWithManager(svc, starter, nil)

	t.Run("invalid-json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/automation/scripts/run", bytes.NewBufferString("{bad json}"))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("期望 400，实际 %d，body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("selector-must-be-object", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/automation/scripts/run", bytes.NewBufferString(`{
			"scriptId":"news-query-txt",
			"selector":"BUYER_001"
		}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("期望 400，实际 %d，body=%s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "selector must be a JSON object") {
			t.Fatalf("错误信息不正确: %s", w.Body.String())
		}
	})

	t.Run("timeout-must-be-in-range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/automation/scripts/run", bytes.NewBufferString(`{
			"scriptId":"news-query-txt",
			"timeoutMs":1800001
		}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("期望 400，实际 %d，body=%s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "timeoutMs must be between 1000 and 1800000") {
			t.Fatalf("错误信息不正确: %s", w.Body.String())
		}
	})
}
