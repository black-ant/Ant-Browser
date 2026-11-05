package windowsizing

import "testing"

func TestFitStartupWindowBoundsKeepsConfiguredSizeWithinWorkArea(t *testing.T) {
	configBounds := StartupWindowBounds{
		Width:     1600,
		Height:    900,
		MinWidth:  1200,
		MinHeight: 700,
	}

	got := fitStartupWindowBounds(configBounds, DesktopWorkArea{Width: 1920, Height: 1080}, true)

	if got.Width != configBounds.Width || got.Height != configBounds.Height {
		t.Fatalf("window size = %dx%d, want configured size %dx%d", got.Width, got.Height, configBounds.Width, configBounds.Height)
	}
	if got.MinWidth != configBounds.MinWidth || got.MinHeight != configBounds.MinHeight {
		t.Fatalf("minimum size = %dx%d, want configured minimum %dx%d", got.MinWidth, got.MinHeight, configBounds.MinWidth, configBounds.MinHeight)
	}
}

func TestFitStartupWindowBoundsClampsOnlyOversizedDimensions(t *testing.T) {
	configBounds := StartupWindowBounds{
		Width:     2200,
		Height:    800,
		MinWidth:  1200,
		MinHeight: 700,
	}

	got := fitStartupWindowBounds(configBounds, DesktopWorkArea{Width: 1920, Height: 700}, true)

	if got.Width != 1920 {
		t.Fatalf("width = %d, want work area width 1920", got.Width)
	}
	if got.Height != 700 {
		t.Fatalf("height = %d, want work area height 700", got.Height)
	}
	if got.MinWidth != 1200 || got.MinHeight != 525 {
		t.Fatalf("minimum size = %dx%d, want 1200x525", got.MinWidth, got.MinHeight)
	}
}

func TestFitStartupWindowBoundsKeepsConfiguredSizeWithoutWorkArea(t *testing.T) {
	configBounds := StartupWindowBounds{
		Width:     1600,
		Height:    900,
		MinWidth:  1200,
		MinHeight: 700,
	}

	got := fitStartupWindowBounds(configBounds, DesktopWorkArea{}, false)

	if got != configBounds {
		t.Fatalf("bounds = %+v, want %+v", got, configBounds)
	}
}
