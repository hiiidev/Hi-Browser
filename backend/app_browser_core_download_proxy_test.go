package backend

import (
	"net/http"
	"testing"
)

func TestBrowserCoreGitHubProxyURL(t *testing.T) {
	t.Parallel()

	got, err := browserCoreGitHubProxyURL("https://github.com/adryfish/fingerprint-chromium/releases/download/v1/chrome.zip")
	if err != nil {
		t.Fatalf("browserCoreGitHubProxyURL() error = %v", err)
	}
	want := "https://gh-proxy.com/https://github.com/adryfish/fingerprint-chromium/releases/download/v1/chrome.zip"
	if got != want {
		t.Fatalf("browserCoreGitHubProxyURL() = %q, want %q", got, want)
	}
}

func TestPrepareBrowserCoreDownloadFallback(t *testing.T) {
	t.Parallel()

	app := &App{}
	githubURL := "https://github.com/adryfish/fingerprint-chromium/releases/download/v1/chrome.zip"
	for _, proxyConfig := range []string{"", "__system__", "__direct__", "direct://"} {
		gotURL, client, err := app.prepareBrowserCoreDownloadFallback(githubURL, proxyConfig)
		if err != nil {
			t.Fatalf("prepareBrowserCoreDownloadFallback(%q) error = %v", proxyConfig, err)
		}
		if gotURL != browserCoreGitHubProxyPrefix+githubURL {
			t.Errorf("prepareBrowserCoreDownloadFallback(%q) URL = %q", proxyConfig, gotURL)
		}
		transport, ok := client.Transport.(*http.Transport)
		if !ok || transport.Proxy != nil {
			t.Errorf("prepareBrowserCoreDownloadFallback(%q) client is not direct", proxyConfig)
		}
	}
}

func TestPrepareBrowserCoreDownloadFallbackDoesNotOverrideCustomMode(t *testing.T) {
	t.Parallel()

	app := &App{}
	githubURL := "https://github.com/adryfish/fingerprint-chromium/releases/download/v1/chrome.zip"
	for _, proxyConfig := range []string{"__gh_proxy__", "http://127.0.0.1:7890", "socks5://127.0.0.1:1080"} {
		gotURL, client, err := app.prepareBrowserCoreDownloadFallback(githubURL, proxyConfig)
		if err != nil || gotURL != "" || client != nil {
			t.Errorf("prepareBrowserCoreDownloadFallback(%q) = %q, %v, %v", proxyConfig, gotURL, client, err)
		}
	}

	gotURL, client, err := app.prepareBrowserCoreDownloadFallback("https://example.com/chrome.zip", "__system__")
	if err != nil || gotURL != "" || client != nil {
		t.Errorf("non-GitHub fallback = %q, %v, %v", gotURL, client, err)
	}
}

func TestPrepareBrowserCoreGitHubProxyUsesDirectClient(t *testing.T) {
	t.Parallel()

	app := &App{}
	targetURL, client, err := app.prepareBrowserCoreDownload(
		"https://github.com/adryfish/fingerprint-chromium/releases/download/v1/chrome.zip",
		"__gh_proxy__",
	)
	if err != nil {
		t.Fatal(err)
	}
	if targetURL != "https://gh-proxy.com/https://github.com/adryfish/fingerprint-chromium/releases/download/v1/chrome.zip" {
		t.Fatalf("targetURL = %q", targetURL)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatal("GitHub proxy client must connect directly to the acceleration service")
	}
}

func TestBrowserCoreGitHubProxyURLRejectsNonGitHubURL(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"http://github.com/adryfish/fingerprint-chromium/releases/download/v1/chrome.zip",
		"https://example.com/chrome.zip",
		"file:///tmp/chrome.zip",
	} {
		if _, err := browserCoreGitHubProxyURL(value); err == nil {
			t.Errorf("browserCoreGitHubProxyURL(%q) error = nil, want error", value)
		}
	}
}
