// Package iconbadge renders a compact profile badge over an application icon.
package iconbadge

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var iconsetSizes = map[string]int{
	"icon_16x16.png":      16,
	"icon_16x16@2x.png":   32,
	"icon_32x32.png":      32,
	"icon_32x32@2x.png":   64,
	"icon_128x128.png":    128,
	"icon_128x128@2x.png": 256,
	"icon_256x256.png":    256,
	"icon_256x256@2x.png": 512,
	"icon_512x512.png":    512,
	"icon_512x512@2x.png": 1024,
}

// WriteIconset writes all PNG sizes required by macOS iconutil.
func WriteIconset(sourcePath, destinationDir, badge, badgeColor string) error {
	source, err := readImage(sourcePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return fmt.Errorf("create iconset directory: %w", err)
	}
	for name, size := range iconsetSizes {
		badged, err := Render(source, size, badge, badgeColor)
		if err != nil {
			return err
		}
		if err := writePNG(filepath.Join(destinationDir, name), badged); err != nil {
			return err
		}
	}
	return nil
}

// Render scales source to a square and draws the badge in the bottom-right corner.
func Render(source image.Image, size int, badge, badgeColor string) (*image.NRGBA, error) {
	if source == nil || size <= 0 {
		return nil, fmt.Errorf("invalid icon source or size")
	}
	parsedColor, err := parseHexColor(badgeColor)
	if err != nil {
		return nil, err
	}
	badge = strings.TrimSpace(badge)
	canvas := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), source, source.Bounds(), draw.Over, nil)

	centerX := float64(size) * 0.78
	centerY := float64(size) * 0.78
	radius := float64(size) * 0.205
	border := float64(size) * 0.025
	drawCircle(canvas, centerX, centerY, radius+border, color.NRGBA{R: 255, G: 255, B: 255, A: 245})
	drawCircle(canvas, centerX, centerY, radius, parsedColor)

	if badge != "" && size >= 32 {
		if err := drawBadgeText(canvas, badge, centerX, centerY, radius); err != nil {
			return nil, err
		}
	}
	return canvas, nil
}

func readImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open source icon: %w", err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode source icon: %w", err)
	}
	return img, nil
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create icon PNG: %w", err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode icon PNG: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close icon PNG: %w", err)
	}
	return nil
}

func parseHexColor(value string) (color.NRGBA, error) {
	var red, green, blue uint8
	if _, err := fmt.Sscanf(strings.ToUpper(strings.TrimSpace(value)), "#%02X%02X%02X", &red, &green, &blue); err != nil {
		return color.NRGBA{}, fmt.Errorf("invalid badge color %q", value)
	}
	return color.NRGBA{R: red, G: green, B: blue, A: 255}, nil
}

func drawCircle(img *image.NRGBA, centerX, centerY, radius float64, fill color.NRGBA) {
	minX := max(0, int(centerX-radius-1))
	maxX := min(img.Bounds().Dx(), int(centerX+radius+1))
	minY := max(0, int(centerY-radius-1))
	maxY := min(img.Bounds().Dy(), int(centerY+radius+1))
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			dx := float64(x) + 0.5 - centerX
			dy := float64(y) + 0.5 - centerY
			if dx*dx+dy*dy <= radius*radius {
				img.SetNRGBA(x, y, fill)
			}
		}
	}
}

func drawBadgeText(img *image.NRGBA, text string, centerX, centerY, radius float64) error {
	fontData := gobold.TTF
	for _, candidate := range systemFontCandidates() {
		data, err := os.ReadFile(candidate)
		if err == nil {
			fontData = data
			break
		}
	}
	collection, err := opentype.ParseCollection(fontData)
	if err != nil {
		return fmt.Errorf("parse badge font: %w", err)
	}
	parsedFont, err := collection.Font(0)
	if err != nil {
		return fmt.Errorf("load badge font: %w", err)
	}
	runeCount := len([]rune(text))
	fontSize := radius * 1.12
	if runeCount == 2 {
		fontSize = radius * 0.88
	} else if runeCount >= 3 {
		fontSize = radius * 0.68
	}
	face, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{Size: fontSize, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return fmt.Errorf("create badge font face: %w", err)
	}
	defer face.Close()

	width := font.MeasureString(face, text)
	metrics := face.Metrics()
	baseline := fixed.I(int(centerY)) + (metrics.Ascent-metrics.Descent)/2
	drawer := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(int(centerX)) - width/2, Y: baseline},
	}
	drawer.DrawString(text)
	return nil
}

func systemFontCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		}
	case "windows":
		return []string{
			filepath.Join(os.Getenv("WINDIR"), "Fonts", "msyh.ttc"),
			filepath.Join(os.Getenv("WINDIR"), "Fonts", "arialbd.ttf"),
		}
	default:
		return []string{
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
		}
	}
}
