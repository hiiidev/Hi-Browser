package backend

import "testing"

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
