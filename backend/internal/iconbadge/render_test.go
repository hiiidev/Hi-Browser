package iconbadge

import (
	"image"
	"image/color"
	"testing"
)

func TestRenderAddsBadgeColor(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 230, G: 230, B: 230, A: 255})
		}
	}
	got, err := Render(source, 128, "01", "#2563EB")
	if err != nil {
		t.Fatal(err)
	}
	pixel := got.NRGBAAt(100, 100)
	if pixel.B <= pixel.R {
		t.Errorf("badge pixel = %#v, want blue-dominant color", pixel)
	}
}

func TestRenderRejectsInvalidColor(t *testing.T) {
	_, err := Render(image.NewNRGBA(image.Rect(0, 0, 32, 32)), 32, "1", "blue")
	if err == nil {
		t.Error("Render() error = nil, want invalid color error")
	}
}
