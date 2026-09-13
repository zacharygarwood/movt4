package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
	"github.com/zacharygarwood/movt4/internal/scan"
)

func TestViewChartsScannedRatings(t *testing.T) {
	interstellar := letterboxd.Film{Slug: "interstellar", Title: "Interstellar (2014)"}
	parasite := letterboxd.Film{Slug: "parasite-2019", Title: "Parasite (2019)"}

	cfg := Config{Scan: scan.Config{Username: "zach", Stars: []letterboxd.Rating{10, 9}, MinShared: 2, MaxUsers: 100}}
	var model tea.Model = New(context.Background(), cfg)
	for _, msg := range []tea.Msg{
		tea.WindowSizeMsg{Width: 100, Height: 30},
		eventMsg{scan.StageStarted{Stage: scan.StageRatings}},
		eventMsg{scan.FavoritesLoaded{Films: []letterboxd.Film{parasite, {Title: "B"}, {Title: "C"}, {Title: "D"}}}},
		eventMsg{scan.MatchesFound{Shared: 3, Usernames: []string{"ana", "ben"}}},
		eventMsg{scan.UserScanned{User: scan.User{Username: "ana", Shared: 3, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {interstellar, parasite}}}}},
		eventMsg{scan.UserScanned{User: scan.User{Username: "ben", Shared: 3, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {interstellar}}}}},
		eventMsg{scan.Finished{}},
	} {
		model, _ = model.Update(msg)
	}

	screen := ansi.Strip(model.View().Content)
	t.Log("\n" + screen)

	for _, want := range []string{"✓ Fetch their ratings", "3/4 ▸ 2", "2/2", "★★★★★", "2 people"} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen is missing %q", want)
		}
	}
	if strings.Index(screen, "Interstellar") > strings.Index(screen, "Parasite (2019)  ") {
		t.Error("Interstellar (2 ratings) should be charted above Parasite (1 rating)")
	}
}

func TestViewShowsBlockedCountdown(t *testing.T) {
	cfg := Config{Scan: scan.Config{Stars: []letterboxd.Rating{10}, MinShared: 2}}
	var model tea.Model = New(context.Background(), cfg)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model, _ = model.Update(eventMsg{scan.StageStarted{Stage: scan.StageSearch}})
	model, _ = model.Update(eventMsg{scan.Blocked{RetryAt: time.Now().Add(time.Minute)}})

	if screen := ansi.Strip(model.View().Content); !strings.Contains(screen, "trying again in 1m0s") {
		t.Errorf("screen is missing the retry countdown:\n%s", screen)
	}

	model, _ = model.Update(eventMsg{scan.MatchesFound{Shared: 4}})
	if screen := ansi.Strip(model.View().Content); strings.Contains(screen, "trying again") {
		t.Error("the countdown should clear once requests get through")
	}
}
