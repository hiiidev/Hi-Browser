package main

import "testing"

func TestFitStartupWindowBoundsUsesThreeQuartersOfDesktop(t *testing.T) {
	bounds := fitStartupWindowBounds(
		startupWindowBounds{Width: 1750, Height: 1000, MinWidth: 1200, MinHeight: 700},
		desktopWorkArea{Width: 1366, Height: 768},
		true,
	)

	if bounds.Width != 1024 || bounds.Height != 576 {
		t.Fatalf("expected 1024x576, got %dx%d", bounds.Width, bounds.Height)
	}
}

func TestFitStartupWindowBoundsKeepsSmallerConfiguredMinimum(t *testing.T) {
	bounds := fitStartupWindowBounds(
		startupWindowBounds{Width: 1750, Height: 1000, MinWidth: 800, MinHeight: 420},
		desktopWorkArea{Width: 1366, Height: 768},
		true,
	)

	if bounds.MinWidth != 800 || bounds.MinHeight != 420 {
		t.Fatalf("expected configured min size 800x420, got %dx%d", bounds.MinWidth, bounds.MinHeight)
	}
}

func TestFitStartupWindowBoundsRelaxesOversizedMinimum(t *testing.T) {
	bounds := fitStartupWindowBounds(
		startupWindowBounds{Width: 1750, Height: 1000, MinWidth: 1200, MinHeight: 700},
		desktopWorkArea{Width: 1366, Height: 768},
		true,
	)

	if bounds.MinWidth != 768 {
		t.Fatalf("expected min width to relax to 768, got %d", bounds.MinWidth)
	}
	if bounds.MinHeight != 432 {
		t.Fatalf("expected min height to relax to 432, got %d", bounds.MinHeight)
	}
}

func TestFitStartupWindowBoundsKeepsConfigWhenDesktopUnavailable(t *testing.T) {
	bounds := fitStartupWindowBounds(
		startupWindowBounds{Width: 1750, Height: 1000, MinWidth: 1200, MinHeight: 700},
		desktopWorkArea{},
		false,
	)

	if bounds.Width != 1750 || bounds.Height != 1000 {
		t.Fatalf("expected configured size 1750x1000, got %dx%d", bounds.Width, bounds.Height)
	}
	if bounds.MinWidth != 1200 || bounds.MinHeight != 700 {
		t.Fatalf("expected configured min size 1200x700, got %dx%d", bounds.MinWidth, bounds.MinHeight)
	}
}
