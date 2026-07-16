//go:build darwin

package backend

import (
	"ant-chrome/backend/internal/iconbadge"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreparePlatformProfileBrowserIcon(t *testing.T) {
	root := t.TempDir()
	sourceBundle := filepath.Join(root, "Source.app")
	macOSDir := filepath.Join(sourceBundle, "Contents", "MacOS")
	resourcesDir := filepath.Join(sourceBundle, "Contents", "Resources")
	if err := os.MkdirAll(macOSDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(resourcesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	executablePath := filepath.Join(macOSDir, "Chromium")
	copyTestFile(t, "/bin/sleep", executablePath, 0o755)
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.example.source</string>
<key>CFBundleDisplayName</key><string>Source</string>
<key>CFBundleExecutable</key><string>Chromium</string>
<key>CFBundleIconFile</key><string>app.icns</string>
<key>CFBundleIconName</key><string>AppIcon</string>
<key>CFBundlePackageType</key><string>APPL</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(sourceBundle, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	sourcePNG := filepath.Join(root, "source.png")
	writeTestIconPNG(t, sourcePNG)
	iconsetDir := filepath.Join(root, "source.iconset")
	if err := iconbadge.WriteIconset(sourcePNG, iconsetDir, "S", "#64748B"); err != nil {
		t.Fatal(err)
	}
	if err := runProfileIconCommand("iconutil", "-c", "icns", iconsetDir, "-o", filepath.Join(resourcesDir, "app.icns")); err != nil {
		t.Fatal(err)
	}

	profile := &BrowserProfile{ProfileId: "profile-1", ProfileName: "工作 01", IconBadge: "中1", IconBadgeColor: "#2563EB"}
	derivedExecutable, err := preparePlatformProfileBrowserIcon(filepath.Join(root, "state"), profile, executablePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(derivedExecutable, "profile-browser-icons") {
		t.Errorf("derived executable = %q, want cached profile app", derivedExecutable)
	}
	if info, err := os.Stat(derivedExecutable); err != nil || info.IsDir() {
		t.Fatalf("derived executable stat = %v, %v", info, err)
	}
	derivedBundle, err := macOSAppBundleForExecutable(derivedExecutable)
	if err != nil {
		t.Fatal(err)
	}
	plistPath := filepath.Join(derivedBundle, "Contents", "Info.plist")
	plistOutput, err := exec.Command("plutil", "-p", plistPath).CombinedOutput()
	if err != nil {
		t.Fatalf("read derived plist: %v: %s", err, plistOutput)
	}
	plistText := string(plistOutput)
	if strings.Contains(plistText, "CFBundleIconName") {
		t.Fatalf("derived plist still contains CFBundleIconName: %s", plistText)
	}
	if !strings.Contains(plistText, `"CFBundleIconFile" => "app.icns"`) {
		t.Fatalf("derived plist does not point to app.icns: %s", plistText)
	}
	if err := runProfileIconCommand("codesign", "--verify", "--deep", "--strict", derivedBundle); err != nil {
		t.Fatalf("derived app signature invalid: %v", err)
	}
	assertNSWorkspaceIconContainsBadgeColor(t, derivedBundle, profile.IconBadgeColor)

	sourcePlistOutput, err := exec.Command("plutil", "-p", filepath.Join(sourceBundle, "Contents", "Info.plist")).CombinedOutput()
	if err != nil || !strings.Contains(string(sourcePlistOutput), `"CFBundleIconName" => "AppIcon"`) {
		t.Fatalf("source app plist changed: %v: %s", err, sourcePlistOutput)
	}
	if _, err := preparePlatformProfileBrowserIcon(filepath.Join(root, "state"), profile, executablePath); err != nil {
		t.Fatalf("cached prepare failed: %v", err)
	}
}

func assertNSWorkspaceIconContainsBadgeColor(t *testing.T, appBundle, hexColor string) {
	t.Helper()
	scriptPath := filepath.Join(t.TempDir(), "workspace_icon.swift")
	outputPath := filepath.Join(t.TempDir(), "workspace_icon.png")
	script := `import AppKit
let icon = NSWorkspace.shared.icon(forFile: CommandLine.arguments[1])
icon.size = NSSize(width: 512, height: 512)
guard let tiff = icon.tiffRepresentation,
      let bitmap = NSBitmapImageRep(data: tiff),
      let png = bitmap.representation(using: .png, properties: [:]) else { exit(2) }
try png.write(to: URL(fileURLWithPath: CommandLine.arguments[2]))
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("swift", scriptPath, appBundle, outputPath).CombinedOutput()
	if err != nil {
		t.Fatalf("NSWorkspace.icon(forFile:) failed: %v: %s", err, output)
	}
	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	want := parseTestHexColor(t, hexColor)
	found := false
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y && !found; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a > 0xD000 && absColor(int(r>>8), int(want.R)) < 28 && absColor(int(g>>8), int(want.G)) < 28 && absColor(int(b>>8), int(want.B)) < 28 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("NSWorkspace icon does not contain badge color %s", hexColor)
	}
}

func parseTestHexColor(t *testing.T, value string) color.NRGBA {
	t.Helper()
	var red, green, blue uint8
	if _, err := fmt.Sscanf(value, "#%02X%02X%02X", &red, &green, &blue); err != nil {
		t.Fatalf("parse color %q: %v", value, err)
	}
	return color.NRGBA{R: red, G: green, B: blue, A: 0xFF}
}

func absColor(left, right int) int {
	if left < right {
		return right - left
	}
	return left - right
}

func TestPreparePlatformProfileBrowserIconWithInstalledCore(t *testing.T) {
	executablePath := os.Getenv("HI_BROWSER_TEST_CHROMIUM")
	if executablePath == "" {
		t.Skip("HI_BROWSER_TEST_CHROMIUM is not set")
	}
	stateRoot := os.Getenv("HI_BROWSER_TEST_STATE_ROOT")
	if stateRoot == "" {
		stateRoot = t.TempDir()
	}
	profiles := []*BrowserProfile{
		{ProfileId: "real-core-smoke-01", ProfileName: "角标冒烟测试 01", IconBadge: "01", IconBadgeColor: "#2563EB"},
		{ProfileId: "real-core-smoke-02", ProfileName: "角标冒烟测试 02", IconBadge: "02", IconBadgeColor: "#DC2626"},
	}
	for _, profile := range profiles {
		derivedExecutable, err := preparePlatformProfileBrowserIcon(stateRoot, profile, executablePath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(derivedExecutable); err != nil {
			t.Fatalf("derived executable %q: %v", derivedExecutable, err)
		}
		bundlePath, err := macOSAppBundleForExecutable(derivedExecutable)
		if err != nil {
			t.Fatal(err)
		}
		assertNSWorkspaceIconContainsBadgeColor(t, bundlePath, profile.IconBadgeColor)
		cachedExecutable, err := preparePlatformProfileBrowserIcon(stateRoot, profile, executablePath)
		if err != nil || cachedExecutable != derivedExecutable {
			t.Fatalf("cache reuse = %q, %v; want %q", cachedExecutable, err, derivedExecutable)
		}
		t.Logf("derived executable for %s: %s", profile.IconBadge, derivedExecutable)
	}
}

func copyTestFile(t *testing.T, sourcePath, destinationPath string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destinationPath, data, mode); err != nil {
		t.Fatal(err)
	}
}

func writeTestIconPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 235, G: 240, B: 245, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}
