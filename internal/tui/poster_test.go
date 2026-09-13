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
