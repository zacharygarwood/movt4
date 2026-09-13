package tui

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderPoster(t *testing.T) {
	// Top half red, bottom half blue.
	img := image.NewRGBA(image.Rect(0, 0, 60, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 60; x++ {
			c := color.RGBA{R: 255, A: 255}
			if y >= 45 {
				c = color.RGBA{B: 255, A: 255}
			}
			img.Set(x, y, c)
		}
	}

	art := renderPoster(img, 4, 6)
	lines := strings.Split(art, "\n")
	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != 4 {
			t.Errorf("line %d is %d cells wide, want 4", i, w)
		}
	}
	if !strings.Contains(lines[0], "38;2;255;0;0") || !strings.Contains(lines[5], "48;2;0;0;255") {
		t.Error("want red at the top and blue at the bottom")
	}
}

func TestSplitColors(t *testing.T) {
	black, white := color.RGBA{A: 255}, color.RGBA{R: 255, G: 255, B: 255, A: 255}
	mask, fg, bg := splitColors([4]color.RGBA{black, white, white, black})
	if quadrants[mask] != '▞' || fg != white || bg != black {
		t.Errorf("got %q in %v on %v, want ▞ in white on black", quadrants[mask], fg, bg)
	}
}

func TestPosterSize(t *testing.T) {
	tests := []struct{ maxWidth, maxHeight, width, height int }{
		{40, 30, 40, 30},  // exactly fits
		{100, 30, 40, 30}, // limited by height
		{20, 30, 20, 15},  // limited by width
	}
	for _, tt := range tests {
		if w, h := posterSize(tt.maxWidth, tt.maxHeight); w != tt.width || h != tt.height {
			t.Errorf("posterSize(%d, %d) = %d×%d, want %d×%d", tt.maxWidth, tt.maxHeight, w, h, tt.width, tt.height)
		}
	}
}
