package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

func TestBuildBrowserLaunchTargetsDefersConfiguredTargetsWhenLightStartEnabled(t *testing.T) {
	t.Parallel()

	launchTargets, deferredTargets := buildBrowserLaunchTargets(
		[]string{"https://one.example/", "https://two.example/"},
		nil,
		false,
		false,
		true,
	)
	if !reflect.DeepEqual(launchTargets, []string{"about:blank"}) {
		t.Fatalf("expected blank-page launch target, got %v", launchTargets)
	}
	if !reflect.DeepEqual(deferredTargets, []string{"https://one.example/", "https://two.example/"}) {
		t.Fatalf("expected deferred targets to be preserved, got %v", deferredTargets)
	}
}

func TestBuildBrowserLaunchTargetsPreservesSessionRestoreWhenNoConfiguredTargets(t *testing.T) {
	t.Parallel()

	launchTargets, deferredTargets := buildBrowserLaunchTargets(nil, nil, false, true, true)
	if len(launchTargets) != 0 {
		t.Fatalf("expected no launch targets when restore-last-session is enabled, got %v", launchTargets)
	}
	if len(deferredTargets) != 0 {
		t.Fatalf("expected no deferred targets, got %v", deferredTargets)
	}
}

func TestOpenBrowserStartTargetsNavigatesFirstPageAndCreatesRemainingTargets(t *testing.T) {
	t.Parallel()

	server := newRecordedCDPServer(t)
	defer server.Close()

	if err := openBrowserStartTargets(server.Port(), []string{"https://one.example/", "https://two.example/"}); err != nil {
		t.Fatalf("openBrowserStartTargets returned error: %v", err)
	}

	want := []recordedCDPCommand{
		{Scope: "page", Method: "Page.navigate", URL: "https://one.example/"},
		{Scope: "browser", Method: "Target.createTarget", URL: "https://two.example/"},
	}
	if got := server.Commands(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected CDP command sequence:\n got=%v\nwant=%v", got, want)
	}
}

type recordedCDPCommand struct {
	Scope  string
	Method string
	URL    string
}

type recordedCDPServer struct {
	server   *httptest.Server
	upgrader websocket.Upgrader

	mu       sync.Mutex
	commands []recordedCDPCommand
}

func newRecordedCDPServer(t *testing.T) *recordedCDPServer {
	t.Helper()

	recorder := &recordedCDPServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		wsURL := recorder.wsURL("/devtools/page/page-1")
		payload := []map[string]any{
			{
				"type":                 "page",
				"webSocketDebuggerUrl": wsURL,
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	})
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		payload := map[string]any{
			"Browser":              "Chrome/142.0",
			"webSocketDebuggerUrl": recorder.wsURL("/devtools/browser/browser-1"),
		}
		_ = json.NewEncoder(w).Encode(payload)
	})
	mux.HandleFunc("/devtools/page/page-1", func(w http.ResponseWriter, r *http.Request) {
		recorder.handleWebsocket(w, r, "page")
	})
	mux.HandleFunc("/devtools/browser/browser-1", func(w http.ResponseWriter, r *http.Request) {
		recorder.handleWebsocket(w, r, "browser")
	})

	recorder.server = httptest.NewServer(mux)
	return recorder
}

func (s *recordedCDPServer) Close() {
	if s == nil || s.server == nil {
		return
	}
	s.server.Close()
}

func (s *recordedCDPServer) Port() int {
	parsed, err := url.Parse(s.server.URL)
	if err != nil {
		return 0
	}
	port, _ := strconv.Atoi(parsed.Port())
	return port
}

func (s *recordedCDPServer) Commands() []recordedCDPCommand {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recordedCDPCommand{}, s.commands...)
}

func (s *recordedCDPServer) wsURL(path string) string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http") + path
}

func (s *recordedCDPServer) handleWebsocket(w http.ResponseWriter, r *http.Request, scope string) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var msg cdpMessage
	if err := conn.ReadJSON(&msg); err != nil {
		return
	}

	command := recordedCDPCommand{
		Scope:  scope,
		Method: msg.Method,
	}
	if urlValue, _ := msg.Params["url"].(string); urlValue != "" {
		command.URL = urlValue
	}

	s.mu.Lock()
	s.commands = append(s.commands, command)
	s.mu.Unlock()

	_ = conn.WriteJSON(cdpResponse{
		Id:     msg.Id,
		Result: map[string]any{"targetId": "target-1"},
	})
}
