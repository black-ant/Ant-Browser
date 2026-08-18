//go:build windows
// +build windows

package backend

import "testing"

func TestProfileWindowMarkerIconDrawsBadgeInTopRight(t *testing.T) {
	pixels := make([]uint32, iconWidth*iconHeight)
	drawProfileWindowMarkerWindowGlyph(pixels, `A`)
	drawProfileWindowMarkerBadge(pixels, `A`)

	badgeColor := profileWindowMarkerColor(`A`)
	badgePixels := 0
	for pixelY := 0; pixelY <= 12; pixelY++ {
		for pixelX := 18; pixelX <= 30; pixelX++ {
			if pixels[pixelY*iconWidth+pixelX] == badgeColor {
				badgePixels++
			}
		}
	}
	if badgePixels == 0 {
		t.Fatal(`instance marker badge was not drawn in the top-right area`)
	}

	for pixelY := 16; pixelY < iconHeight; pixelY++ {
		for pixelX := 19; pixelX < iconWidth; pixelX++ {
			if pixels[pixelY*iconWidth+pixelX] == badgeColor {
				t.Fatalf(`badge color leaked into the lower-right area at (%d, %d)`, pixelX, pixelY)
			}
		}
	}
}
