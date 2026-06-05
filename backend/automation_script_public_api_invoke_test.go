package backend

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
)

func TestAutomationScriptInvokePublicAPIReturnsJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body failed: %v", err)
		}
		if strings.TrimSpace(string(body)) != `{"hello":"world"}` {
			t.Fatalf("unexpected request body: %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"verificationCode":"429792"}`))
	}))
	defer server.Close()

	app := NewApp(t.TempDir())
	result, err := app.AutomationScriptInvokePublicAPI(AutomationScriptPublicAPIInvokeInput{
		URL:      server.URL + "/api/automation/hooks/test",
		Method:   http.MethodPost,
		BodyText: `{"hello":"world"}`,
	})
	if err != nil {
		t.Fatalf("AutomationScriptInvokePublicAPI returned error: %v", err)
	}
	if result == nil {
		t.Fatal("AutomationScriptInvokePublicAPI returned nil result")
	}
	if !result.OK {
		t.Fatalf("expected ok result, got %+v", result)
	}
	if result.Status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", result.Status)
	}

	bodyJSON, ok := result.BodyJSON.(map[string]interface{})
	if !ok {
		t.Fatalf("expected bodyJson object, got %#v", result.BodyJSON)
	}
	if bodyJSON["verificationCode"] != "429792" {
		t.Fatalf("expected verificationCode 429792, got %#v", bodyJSON["verificationCode"])
	}
}

func TestAutomationScriptInvokePublicAPIAutoUsesLaunchServerAuth(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Test-Key"); got != "secret-123" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"ok":false}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"mailboxName":"ChatGPT"}`))
	}))
	defer server.Close()

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server url failed: %v", err)
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		t.Fatalf("parse test server port failed: %v", err)
	}

	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.config.LaunchServer.Auth.Enabled = true
	app.config.LaunchServer.Auth.APIKey = "secret-123"
	app.config.LaunchServer.Auth.Header = "X-Test-Key"
	app.launchServer = launchcode.NewLaunchServer(nil, nil, nil, port)
	app.launchServer.SetAPIAuthConfig(launchcode.APIAuthConfig{
		Enabled: true,
		APIKey:  "secret-123",
		Header:  "X-Test-Key",
	})

	result, err := app.AutomationScriptInvokePublicAPI(AutomationScriptPublicAPIInvokeInput{
		URL:      server.URL + "/api/automation/hooks/test",
		Method:   http.MethodPost,
		BodyText: `{}`,
	})
	if err != nil {
		t.Fatalf("AutomationScriptInvokePublicAPI returned error: %v", err)
	}
	if result == nil {
		t.Fatal("AutomationScriptInvokePublicAPI returned nil result")
	}
	if !result.OK {
		t.Fatalf("expected ok result, got %+v", result)
	}
	if result.Status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", result.Status)
	}
}
