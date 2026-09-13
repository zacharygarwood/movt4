package tui

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"golang.org/x/image/draw"
)

// minPosterHeight is the smallest poster, in rows, worth showing.
const minPosterHeight = 12

// quadrants holds the block character for each combination of a cell's four
// quarters drawn in the foreground color: bit 0 is top left, 1 top right,
// 2 bottom left and 3 bottom right.
var quadrants = []rune(" ▘▝▀▖▌▞▛▗▚▐▜▄▙▟█")

// posterSize returns the largest poster that fits in maxWidth×maxHeight
// cells. Terminal cells are about twice as tall as they are wide, so a
// poster's 2:3 aspect ratio works out to 4 columns for every 3 rows.
func posterSize(maxWidth, maxHeight int) (width, height int) {
	height = max(0, min(maxHeight, maxWidth*3/4))
	return height * 4 / 3, height
}

// renderPoster draws img with quadrant blocks in truecolor. Each cell shows
// 2×2 pixels in the two colors that fit them best, which gives twice the
// detail of half blocks without needing an image protocol.
func renderPoster(img image.Image, width, height int) string {
	pixels := image.NewRGBA(image.Rect(0, 0, width*2, height*2))
	draw.CatmullRom.Scale(pixels, pixels.Bounds(), img, img.Bounds(), draw.Src, nil)

	var b strings.Builder
	for y := 0; y < height; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x := 0; x < width; x++ {
			var cell [4]color.RGBA
			for i := range cell {
				cell[i] = pixels.RGBAAt(2*x+i%2, 2*y+i/2)
			}
			mask, fg, bg := splitColors(cell)
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm%c", fg.R, fg.G, fg.B, bg.R, bg.G, bg.B, quadrants[mask])
		}
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

// splitColors divides a cell's four pixels into the two groups whose average
// colors represent them with the least error. It returns which pixels take
// the foreground color, as a quadrants index, and the two colors.
func splitColors(cell [4]color.RGBA) (mask int, fg, bg color.RGBA) {
	bestErr := -1.0
	for m := 1; m < len(quadrants); m++ {
		var sum [2][3]float64 // per group: background, foreground
		var count [2]float64
		for i, c := range cell {
			g := m >> i & 1
			sum[g][0] += float64(c.R)
			sum[g][1] += float64(c.G)
			sum[g][2] += float64(c.B)
			count[g]++
		}
		var mean [2][3]float64
		for g := range mean {
			for k := range mean[g] {
				if count[g] > 0 {
					mean[g][k] = sum[g][k] / count[g]
				}
			}
		}
		err := 0.0
		for i, c := range cell {
			g := m >> i & 1
			for k, v := range [3]uint8{c.R, c.G, c.B} {
				d := float64(v) - mean[g][k]
				err += d * d
			}
		}
		if bestErr < 0 || err < bestErr {
			bestErr, mask = err, m
			fg = color.RGBA{uint8(mean[1][0]), uint8(mean[1][1]), uint8(mean[1][2]), 255}
			bg = color.RGBA{uint8(mean[0][0]), uint8(mean[0][1]), uint8(mean[0][2]), 255}
			if count[0] == 0 {
				bg = fg
			}
		}
	}
	return mask, fg, bg
}
