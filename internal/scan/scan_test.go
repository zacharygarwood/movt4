package scan

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"strings"
	"testing"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
)

var (
	parasite = letterboxd.Film{Slug: "parasite-2019", Title: "Parasite (2019)"}
	whiplash = letterboxd.Film{Slug: "whiplash-2014", Title: "Whiplash (2014)"}
	top4     = []letterboxd.Film{parasite, whiplash, {Slug: "c"}, {Slug: "d"}}
	queries  = []string{"parasite", "whiplash", "c", "d"}
)

type fakeSource struct {
	search    map[int][][]string // search result pages by minShared
	searchErr map[int]error      // returned after a tier's pages
	rated     map[string][]letterboxd.Film
}

func (f fakeSource) Favorites(context.Context, string) ([]letterboxd.Film, error) {
	return top4, nil
}

func (f fakeSource) SearchFans(_ context.Context, _ []string, minShared int) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {
		for _, page := range f.search[minShared] {
			if !yield(page, nil) {
				return
			}
		}
		if err := f.searchErr[minShared]; err != nil {
			yield(nil, err)
		}
	}
}

// RatedFilms files each member's films under the lowest rating asked for.
func (f fakeSource) RatedFilms(_ context.Context, username string, from, _ letterboxd.Rating) (map[letterboxd.Rating][]letterboxd.Film, error) {
	films, ok := f.rated[username]
	if !ok {
		return nil, errors.New("not found")
	}
	return map[letterboxd.Rating][]letterboxd.Film{from: films}, nil
}

// FindFilm finds a film for any query except "nope".
func (f fakeSource) FindFilm(_ context.Context, query string) (letterboxd.Film, error) {
	if query == "nope" {
		return letterboxd.Film{}, errors.New("no matching film")
	}
	return letterboxd.Film{Slug: query, Title: query}, nil
}

func collect(t *testing.T, cfg Config, src Source) (Results, []string, Finished) {
	t.Helper()
	dial := func(context.Context, func(Event)) (Source, error) { return src, nil }
	var results Results
	var failed []string
	var done Finished
	for e := range Run(context.Background(), cfg, dial) {
		switch e := e.(type) {
		case UserScanned:
			results.Add(e.User)
		case UserFailed:
			failed = append(failed, e.Username)
		case Finished:
			done = e
		}
	}
	return results, failed, done
}

func TestRunAssignsTiersAndSkipsFailures(t *testing.T) {
	src := fakeSource{
		search: map[int][][]string{
			4: {{"ana"}},
			3: {{"ana", "ben"}, {"ben", "me"}},
			2: {{"ana", "ben", "cy", "gone"}},
		},
		rated: map[string][]letterboxd.Film{
			"ana": {parasite},
			"ben": {parasite, whiplash},
			"cy":  {whiplash},
		},
	}
	cfg := Config{Username: "me", Stars: []letterboxd.Rating{10}, MinShared: 2}

	results, failed, done := collect(t, cfg, src)
	if done.Err != nil {
		t.Fatal(done.Err)
	}
	shared := map[string]int{}
	for _, u := range results.Users {
		shared[u.Username] = u.Shared
	}
	if want := map[string]int{"ana": 4, "ben": 3, "cy": 2}; !reflect.DeepEqual(shared, want) {
		t.Errorf("shared = %v, want %v", shared, want)
	}
	if !reflect.DeepEqual(failed, []string{"gone"}) {
		t.Errorf("failed = %v, want [gone]", failed)
	}
}

func TestRunStopsAtMaxUsers(t *testing.T) {
	src := fakeSource{
		search: map[int][][]string{4: {{"a", "b"}, {"c"}}, 3: {{"d"}}},
		rated:  map[string][]letterboxd.Film{"a": nil, "b": nil, "c": nil, "d": nil},
	}
	cfg := Config{Films: queries, Stars: []letterboxd.Rating{10}, MinShared: 3, MaxUsers: 3}

	results, _, done := collect(t, cfg, src)
	if done.Err != nil {
		t.Fatal(done.Err)
	}
	if len(results.Users) != 3 {
		t.Errorf("scanned %d users, want 3", len(results.Users))
	}
}

func TestRunWithoutMatches(t *testing.T) {
	cfg := Config{Films: queries, Stars: []letterboxd.Rating{10}, MinShared: 2}
	if _, _, done := collect(t, cfg, fakeSource{}); done.Err == nil {
		t.Error("want an error when nobody matches")
	}
}

func TestRunKeepsMatchesWhenSearchIsBlocked(t *testing.T) {
	src := fakeSource{
		search:    map[int][][]string{4: {{"ana"}}, 3: {{"ben"}}},
		searchErr: map[int]error{4: letterboxd.ErrBlocked},
		rated:     map[string][]letterboxd.Film{"ana": nil, "ben": nil},
	}
	cfg := Config{Films: queries, Stars: []letterboxd.Rating{10}, MinShared: 3}

	results, _, done := collect(t, cfg, src)
	if done.Err != nil {
		t.Fatal(done.Err)
	}
	if len(results.Users) != 1 || results.Users[0].Username != "ana" {
		t.Errorf("scanned %+v, want only ana: the search should stop at the blocked tier", results.Users)
	}
}

// blockedSource blocks every member's ratings except those in allowed.
type blockedSource struct {
	fakeSource
	allowed map[string]bool
}

func (s blockedSource) RatedFilms(_ context.Context, username string, _, _ letterboxd.Rating) (map[letterboxd.Rating][]letterboxd.Film, error) {
	if s.allowed[username] {
		return nil, nil
	}
	return nil, letterboxd.ErrBlocked
}

func TestRunFinishesEarlyWhenBlockedRepeatedly(t *testing.T) {
	src := blockedSource{
		fakeSource: fakeSource{search: map[int][][]string{4: {{"a", "b", "c", "d", "e", "f"}}}},
		allowed:    map[string]bool{"b": true},
	}
	cfg := Config{Films: queries, Stars: []letterboxd.Rating{10}, MinShared: 4}

	results, failed, done := collect(t, cfg, src)
	// b resets the count, so c, d and e are three blocks in a row and f is never tried.
	if done.Err != nil || len(results.Users) != 1 || !reflect.DeepEqual(failed, []string{"a", "c", "d", "e"}) || done.Unscanned != 1 {
		t.Errorf("got %d users, failed %v, %+v", len(results.Users), failed, done)
	}
}

// rangedSource checks that ratings are requested as one range from 4 to 5 stars.
type rangedSource struct{ fakeSource }

func (rangedSource) RatedFilms(_ context.Context, _ string, from, to letterboxd.Rating) (map[letterboxd.Rating][]letterboxd.Film, error) {
	if from != 8 || to != 10 {
		return nil, fmt.Errorf("asked for ratings %d-%d, want 8-10", from, to)
	}
	return map[letterboxd.Rating][]letterboxd.Film{8: {parasite}, 9: {whiplash}, 10: {parasite}}, nil
}

func TestRunFetchesRatingsAsOneRange(t *testing.T) {
	src := rangedSource{fakeSource{search: map[int][][]string{4: {{"ana"}}}}}
	cfg := Config{Films: queries, Stars: []letterboxd.Rating{10, 8}, MinShared: 4}

	results, failed, done := collect(t, cfg, src)
	if done.Err != nil || len(failed) != 0 || len(results.Users) != 1 {
		t.Fatalf("got %d users, failed %v, %+v", len(results.Users), failed, done)
	}
	// ★★★★½ falls inside the range but wasn't asked for.
	want := map[letterboxd.Rating][]letterboxd.Film{10: {parasite}, 8: {parasite}}
	if got := results.Users[0].Ratings; !reflect.DeepEqual(got, want) {
		t.Errorf("ratings = %v, want %v", got, want)
	}
}

func TestRunNamesFilmsItCantFind(t *testing.T) {
	cfg := Config{Films: []string{"parasite", "nope"}, Stars: []letterboxd.Rating{10}, MinShared: 2}
	if _, _, done := collect(t, cfg, fakeSource{}); done.Err == nil || !strings.Contains(done.Err.Error(), `"nope"`) {
		t.Errorf("err = %v, want it to name the film it couldn't find", done.Err)
	}
}
