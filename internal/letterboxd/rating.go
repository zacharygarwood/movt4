package letterboxd

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Rating is a Letterboxd star rating counted in half-stars, so 10 is ★★★★★
// and 9 is ★★★★½. Storing half-stars keeps ratings usable as map keys.
type Rating int

// ParseRating parses a star value in half steps from "0.5" to "5".
func ParseRating(s string) (Rating, error) {
	stars, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	halves := stars * 2
	if err != nil || halves != math.Trunc(halves) || halves < 1 || halves > 10 {
		return 0, fmt.Errorf("invalid rating %q: use 0.5 to 5 in half steps", s)
	}
	return Rating(halves), nil
}

// Stars formats the rating as a number of stars, e.g. "4.5".
func (r Rating) Stars() string {
	return strconv.FormatFloat(float64(r)/2, 'f', -1, 64)
}

// String renders the rating with star glyphs, e.g. "★★★★½".
func (r Rating) String() string {
	return strings.Repeat("★", int(r)/2) + strings.Repeat("½", int(r)%2)
}
