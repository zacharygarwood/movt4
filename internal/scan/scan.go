// Package scan finds Letterboxd members who share a Top 4 and collects the
// films they rated, reporting progress as a stream of events.
package scan

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
)

// maxBlockedInARow is how many members in a row Letterboxd may block before
// the scan stops fetching ratings and finishes with what it has.
const maxBlockedInARow = 3

// Source is the part of the Letterboxd client a scan needs.
type Source interface {
	Favorites(ctx context.Context, username string) ([]letterboxd.Film, error)
	SearchFans(ctx context.Context, slugs []string, minShared int) iter.Seq2[[]string, error]
	RatedFilms(ctx context.Context, username string, from, to letterboxd.Rating) (map[letterboxd.Rating][]letterboxd.Film, error)
}

// Dialer connects to a Source. Starting the browser is slow enough to be
// reported as its own stage, so the scan does it rather than the caller.
// report lets the Source send events of its own, such as Blocked.
type Dialer func(ctx context.Context, report func(Event)) (Source, error)

// Config describes a scan.
type Config struct {
	Username  string            // read the Top 4 from this member's profile...
	Films     []letterboxd.Film // ...or match against these films instead
	Stars     []letterboxd.Rating
	MinShared int // the fewest shared favorites that count as a match
	MaxUsers  int // stop matching after this many members; 0 means no limit
}

// Run scans in the background. The returned channel delivers progress and
// is closed after the Finished event, or early if ctx is cancelled.
func Run(ctx context.Context, cfg Config, dial Dialer) <-chan Event {
	events := make(chan Event)
	s := &scanner{ctx: ctx, cfg: cfg, events: events}
	go func() {
		defer close(events)
		err := s.run(dial)
		s.emit(Finished{Err: err, Unscanned: s.unscanned})
	}()
	return events
}

type scanner struct {
	ctx       context.Context
	cfg       Config
	events    chan<- Event
	unscanned int
}

type match struct {
	username string
	shared   int
}

func (s *scanner) emit(e Event) {
	select {
	case s.events <- e:
	case <-s.ctx.Done():
	}
}

func (s *scanner) run(dial Dialer) error {
	s.emit(StageStarted{StageConnect})
	src, err := dial(s.ctx, s.emit)
	if err != nil {
		return err
	}

	s.emit(StageStarted{StageFavorites})
	films, err := s.favorites(src)
	if err != nil {
		return err
	}
	s.emit(FavoritesLoaded{films})

	s.emit(StageStarted{StageSearch})
	matches, err := s.findMatches(src, films)
	if err != nil {
		return err
	}

	s.emit(StageStarted{StageRatings})
	blockedInARow := 0
	for i, m := range matches {
		user, err := s.fetchRatings(src, m)
		if s.ctx.Err() != nil {
			return s.ctx.Err()
		}
		if err != nil {
			s.emit(UserFailed{Username: m.username, Err: err})
			if errors.Is(err, letterboxd.ErrBlocked) {
				if blockedInARow++; blockedInARow == maxBlockedInARow {
					s.unscanned = len(matches) - i - 1
					return nil
				}
			}
			continue
		}
		blockedInARow = 0
		s.emit(UserScanned{user})
	}
	return nil
}

func (s *scanner) favorites(src Source) ([]letterboxd.Film, error) {
	films := s.cfg.Films
	if s.cfg.Username != "" {
		var err error
		if films, err = src.Favorites(s.ctx, s.cfg.Username); err != nil {
			return nil, fmt.Errorf("reading %s's profile: %w", s.cfg.Username, err)
		}
	}
	if len(films) < s.cfg.MinShared {
		return nil, fmt.Errorf("need at least %d favorite films to match on, found %d", s.cfg.MinShared, len(films))
	}
	return films, nil
}

// findMatches searches from the strictest tier down. Each search returns
// everyone sharing at least that many films, so a username not seen in a
// stricter tier shares exactly that many.
//
// If a search fails partway, the matches found so far are kept but the
// search stops: without the rest of a stricter tier, members in looser tiers
// could be counted as sharing fewer films than they do.
func (s *scanner) findMatches(src Source, films []letterboxd.Film) ([]match, error) {
	slugs := make([]string, len(films))
	for i, f := range films {
		slugs[i] = f.Slug
	}
	seen := map[string]bool{strings.ToLower(s.cfg.Username): true}
	var matches []match
	full := func() bool { return s.cfg.MaxUsers > 0 && len(matches) >= s.cfg.MaxUsers }

	var searchErr error
	for shared := len(slugs); shared >= s.cfg.MinShared && !full() && searchErr == nil; shared-- {
		for page, err := range src.SearchFans(s.ctx, slugs, shared) {
			if err != nil {
				searchErr = err
				break
			}
			var found []string
			for _, username := range page {
				if !seen[username] && !full() {
					seen[username] = true
					matches = append(matches, match{username, shared})
					found = append(found, username)
				}
			}
			s.emit(MatchesFound{Shared: shared, Usernames: found})
			if full() {
				break
			}
		}
	}
	if s.ctx.Err() != nil {
		return nil, s.ctx.Err()
	}
	if len(matches) == 0 {
		if searchErr != nil {
			return nil, fmt.Errorf("searching members: %w", searchErr)
		}
		return nil, errors.New("no members share enough of these favorites")
	}
	return matches, nil
}

// fetchRatings asks for one range spanning every configured rating, which
// takes far fewer requests than asking for each, then keeps only the
// configured ratings from it.
func (s *scanner) fetchRatings(src Source, m match) (User, error) {
	rated, err := src.RatedFilms(s.ctx, m.username, slices.Min(s.cfg.Stars), slices.Max(s.cfg.Stars))
	if err != nil {
		return User{}, err
	}
	user := User{Username: m.username, Shared: m.shared, Ratings: map[letterboxd.Rating][]letterboxd.Film{}}
	for _, r := range s.cfg.Stars {
		user.Ratings[r] = rated[r]
	}
	return user, nil
}
