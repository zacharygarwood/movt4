package tui

import (
	"fmt"
	"image"
	"strings"

	"golang.org/x/image/draw"
)

// Poster size in terminal cells. Half blocks make each cell two pixels tall,
// so 24×18 cells hold a 24×36 pixel image: a poster's 2:3 aspect ratio.
const (
	posterWidth  = 24
	posterHeight = 18
)

// renderPoster draws img with truecolor half blocks. Each "▀" shows the
// upper pixel in its foreground color and the lower pixel in its background,
// which works in any truecolor terminal without an image protocol.
func renderPoster(img image.Image, width, height int) string {
	pixels := image.NewRGBA(image.Rect(0, 0, width, height*2))
	draw.CatmullRom.Scale(pixels, pixels.Bounds(), img, img.Bounds(), draw.Src, nil)

	var b strings.Builder
	for y := 0; y < height*2; y += 2 {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x := 0; x < width; x++ {
			top, bottom := pixels.RGBAAt(x, y), pixels.RGBAAt(x, y+1)
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", top.R, top.G, top.B, bottom.R, bottom.G, bottom.B)
		}
		b.WriteString("\x1b[0m")
	}
	return b.String()
}
