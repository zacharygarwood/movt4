// Command movt4 finds Letterboxd members who share your Top 4 favorite films
// and charts the films they rated.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
	"github.com/zacharygarwood/movt4/internal/scan"
	"github.com/zacharygarwood/movt4/internal/tui"
)

const usage = `Usage:
  movt4 [flags] <letterboxd-username>
  movt4 [flags] --films <film>,<film>,<film>,<film>

Finds members who share your Top 4 favorites and charts the films they rated.
Films can be slugs (parasite-2019) or Letterboxd film URLs.

Flags:
`

// blockedWaits are the pauses before retrying a request Cloudflare blocked.
// Blocks seen so far have lifted within a few minutes.
var blockedWaits = []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "movt4:", err)
		os.Exit(1)
	}
}

func run() error {
	films := flag.String("films", "", "comma-separated films to match instead of a member's Top 4")
	stars := flag.String("stars", "5,4.5,4", "comma-separated star ratings to fetch")
	minShared := flag.Int("min-shared", 2, "fewest shared favorites that count as a match (2-4)")
	maxUsers := flag.Int("max-users", 100, "most members to scan, 0 for no limit")
	delay := flag.Duration("delay", time.Second, "pause between Letterboxd requests")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg, err := scanConfig(flag.Args(), *films, *stars, *minShared, *maxUsers)
	if err != nil {
		flag.Usage()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	open := letterboxd.Throttle(letterboxd.HTTPFetcher{}, *delay)
	var browser atomic.Pointer[letterboxd.BrowserFetcher]
	dial := func(ctx context.Context, report func(scan.Event)) (scan.Source, error) {
		b, err := letterboxd.NewBrowserFetcher(ctx)
		if err != nil {
			return nil, err
		}
		browser.Store(b)
		guarded := letterboxd.RetryBlocked(letterboxd.Throttle(b, *delay), blockedWaits, func(wait time.Duration) {
			report(scan.Blocked{RetryAt: time.Now().Add(wait)})
		})
		return letterboxd.NewClient(open, guarded), nil
	}

	// Posters are on pages Cloudflare doesn't guard, so they skip the browser.
	// The UI only requests the selected film's poster, once.
	posters := letterboxd.NewClient(letterboxd.HTTPFetcher{}, nil)

	ui := tui.New(ctx, tui.Config{Scan: cfg, Dial: dial, Poster: posters.Poster})
	_, err = tea.NewProgram(ui).Run()

	// Stop the scan, then wait for Chromium to exit.
	cancel()
	if b := browser.Load(); b != nil {
		b.Close()
	}
	return err
}

// scanConfig validates the command line and turns it into a scan.Config.
func scanConfig(args []string, films, stars string, minShared, maxUsers int) (scan.Config, error) {
	cfg := scan.Config{MinShared: minShared, MaxUsers: maxUsers}

	switch {
	case len(args) == 1 && films == "":
		cfg.Username = strings.ToLower(strings.Trim(args[0], "/@ "))
	case len(args) == 0 && films != "":
		for _, film := range strings.Split(films, ",") {
			slug := filmSlug(film)
			cfg.Films = append(cfg.Films, letterboxd.Film{Slug: slug, Title: slug})
		}
		if len(cfg.Films) > 4 {
			return cfg, errors.New("--films takes at most 4 films")
		}
	default:
		return cfg, errors.New("give either a Letterboxd username or --films")
	}

	if minShared < 2 || minShared > 4 {
		return cfg, errors.New("--min-shared must be 2, 3 or 4")
	}
	if maxUsers < 0 {
		return cfg, errors.New("--max-users can't be negative")
	}
	for _, s := range strings.Split(stars, ",") {
		rating, err := letterboxd.ParseRating(s)
		if err != nil {
			return cfg, err
		}
		cfg.Stars = append(cfg.Stars, rating)
	}
	return cfg, nil
}

// filmSlug accepts a slug or any Letterboxd film URL and returns the slug.
func filmSlug(film string) string {
	film = strings.TrimSpace(film)
	if _, after, found := strings.Cut(film, "/film/"); found {
		film = after
	}
	slug, _, _ := strings.Cut(film, "/")
	return slug
}
