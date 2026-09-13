package tui

import (
	"context"
	"image"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
	"github.com/zacharygarwood/movt4/internal/scan"
)

var (
	interstellar = letterboxd.Film{Slug: "interstellar", Title: "Interstellar (2014)"}
	parasite     = letterboxd.Film{Slug: "parasite-2019", Title: "Parasite (2019)"}
)

func update(model tea.Model, msgs ...tea.Msg) tea.Model {
	for _, msg := range msgs {
		model, _ = model.Update(msg)
	}
	return model
}

func screen(model tea.Model) string {
	return ansi.Strip(model.View().Content)
}

func TestViewChartsScannedRatings(t *testing.T) {
	cfg := Config{Scan: scan.Config{Username: "zach", Stars: []letterboxd.Rating{10, 9}, MinShared: 2, MaxUsers: 100}}
	model := update(New(context.Background(), cfg),
		tea.WindowSizeMsg{Width: 100, Height: 30},
		eventMsg{scan.StageStarted{Stage: scan.StageRatings}},
		eventMsg{scan.FavoritesLoaded{Films: []letterboxd.Film{parasite, {Title: "B"}, {Title: "C"}, {Title: "D"}}}},
		eventMsg{scan.MatchesFound{Shared: 3, Usernames: []string{"ana", "ben"}}},
		eventMsg{scan.UserScanned{User: scan.User{Username: "ana", Shared: 3, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {interstellar, parasite}}}}},
		eventMsg{scan.UserScanned{User: scan.User{Username: "ben", Shared: 3, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {interstellar}}}}},
		eventMsg{scan.Finished{}},
	)

	got := screen(model)
	t.Log("\n" + got)
	for _, want := range []string{"✓ Fetch their ratings", "3/4 ▸ 2", "2/2", "★★★★★", "2 people"} {
		if !strings.Contains(got, want) {
			t.Errorf("screen is missing %q", want)
		}
	}
	if strings.Index(got, "Interstellar") > strings.Index(got, "Parasite (2019)  ") {
		t.Error("Interstellar (2 ratings) should be charted above Parasite (1 rating)")
	}
}

func TestViewShowsBlockedCountdown(t *testing.T) {
	cfg := Config{Scan: scan.Config{Stars: []letterboxd.Rating{10}, MinShared: 2}}
	model := update(New(context.Background(), cfg),
		tea.WindowSizeMsg{Width: 100, Height: 30},
		eventMsg{scan.StageStarted{Stage: scan.StageSearch}},
		eventMsg{scan.Blocked{RetryAt: time.Now().Add(time.Minute)}},
	)
	if got := screen(model); !strings.Contains(got, "resuming in 1m0s") {
		t.Errorf("screen is missing the resume countdown:\n%s", got)
	}

	model = update(model, eventMsg{scan.MatchesFound{Shared: 4}})
	if strings.Contains(screen(model), "resuming") {
		t.Error("the countdown should clear once requests get through")
	}
}

func TestHideFavorites(t *testing.T) {
	cfg := Config{Scan: scan.Config{Stars: []letterboxd.Rating{10}, MinShared: 2}}
	model := update(New(context.Background(), cfg),
		eventMsg{scan.FavoritesLoaded{Films: []letterboxd.Film{parasite}}},
		eventMsg{scan.UserScanned{User: scan.User{Username: "ana", Shared: 2, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {interstellar, parasite}}}}},
	)
	if n := len(model.(Model).tally()); n != 2 {
		t.Fatalf("chart shows %d films, want both by default", n)
	}

	model = update(model, tea.KeyPressMsg{Code: 'f', Text: "f"})
	if tally := model.(Model).tally(); len(tally) != 1 || tally[0].Film != interstellar {
		t.Errorf("with the Top 4 hidden, chart shows %+v, want only Interstellar", tally)
	}
}

func TestViewFitsPosterBesideThePage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 90))
	tests := []struct {
		size       tea.WindowSizeMsg
		wantPoster bool
	}{
		{tea.WindowSizeMsg{Width: 160, Height: 50}, true},
		{tea.WindowSizeMsg{Width: 100, Height: 40}, true},
		{tea.WindowSizeMsg{Width: 80, Height: 24}, false},
	}
	for _, tt := range tests {
		cfg := Config{
			Scan:   scan.Config{Username: "zach", Stars: []letterboxd.Rating{10}, MinShared: 2, MaxUsers: 100},
			Poster: func(context.Context, string) (image.Image, error) { return img, nil },
		}
		model := update(New(context.Background(), cfg),
			tt.size,
			eventMsg{scan.StageStarted{Stage: scan.StageRatings}},
			eventMsg{scan.MatchesFound{Shared: 3, Usernames: []string{"ana"}}},
			eventMsg{scan.UserScanned{User: scan.User{Username: "ana", Shared: 3, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {interstellar}}}}},
			posterMsg{slug: interstellar.Slug, img: img},
		)

		content := model.View().Content
		lines := strings.Split(content, "\n")
		if len(lines) > tt.size.Height {
			t.Errorf("%d×%d: %d lines, taller than the window", tt.size.Width, tt.size.Height, len(lines))
		}
		for i, line := range lines {
			if w := ansi.StringWidth(line); w > tt.size.Width {
				t.Errorf("%d×%d: line %d is %d wide, wider than the window", tt.size.Width, tt.size.Height, i, w)
			}
		}
		if !strings.Contains(ansi.Strip(content), "q quit") {
			t.Errorf("%d×%d: the key help is cut off", tt.size.Width, tt.size.Height)
		}
		// With a poster, the title appears twice: in the chart and as the caption.
		if hasPoster := strings.Count(ansi.Strip(content), interstellar.Title) == 2; hasPoster != tt.wantPoster {
			t.Errorf("%d×%d: poster shown = %v, want %v", tt.size.Width, tt.size.Height, hasPoster, tt.wantPoster)
		}
	}
}

func TestBarOf(t *testing.T) {
	tests := []struct{ value, largest, width, cells int }{
		{10, 10, 20, 20}, // the largest fills the width
		{5, 10, 20, 10},
		{1, 100, 20, 1}, // never vanishes
	}
	for _, tt := range tests {
		if got := utf8.RuneCountInString(barOf(tt.value, tt.largest, tt.width)); got != tt.cells {
			t.Errorf("barOf(%d, %d, %d) is %d cells, want %d", tt.value, tt.largest, tt.width, got, tt.cells)
		}
	}
}

func TestPosterViewFillsTheWindow(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 90))
	cfg := Config{
		Scan:   scan.Config{Stars: []letterboxd.Rating{10}, MinShared: 2},
		Poster: func(context.Context, string) (image.Image, error) { return img, nil },
	}
	model := update(New(context.Background(), cfg),
		tea.WindowSizeMsg{Width: 100, Height: 40},
		eventMsg{scan.UserScanned{User: scan.User{Username: "ana", Shared: 2, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {interstellar}}}}},
		posterMsg{slug: interstellar.Slug, img: img},
		tea.KeyPressMsg{Code: tea.KeyEnter},
	)

	got := screen(model)
	if strings.Contains(got, "Fetch their ratings") || !strings.Contains(got, interstellar.Title) {
		t.Errorf("enter should show only the poster and its title:\n%s", got)
	}
	if lines := strings.Count(model.View().Content, "\n") + 1; lines > 40 {
		t.Errorf("poster view is %d lines, taller than the window", lines)
	}
	// 40 rows, less the page padding and a row each for the title and help.
	if h := model.(Model).art.height; h != 36 {
		t.Errorf("poster is %d rows, want 36", h)
	}

	model = update(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(screen(model), "Fetch their ratings") {
		t.Error("esc should close the poster")
	}
}
