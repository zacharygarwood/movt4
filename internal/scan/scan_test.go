package scan

import (
	"context"
	"errors"
	"iter"
	"reflect"
	"testing"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
)

var (
	parasite = letterboxd.Film{Slug: "parasite-2019", Title: "Parasite (2019)"}
	whiplash = letterboxd.Film{Slug: "whiplash-2014", Title: "Whiplash (2014)"}
	top4     = []letterboxd.Film{parasite, whiplash, {Slug: "c"}, {Slug: "d"}}
)

type fakeSource struct {
	search map[int][][]string // search result pages by minShared
	rated  map[string][]letterboxd.Film
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
	}
}

func (f fakeSource) RatedFilms(_ context.Context, username string, _ letterboxd.Rating) ([]letterboxd.Film, error) {
	films, ok := f.rated[username]
	if !ok {
		return nil, errors.New("not found")
	}
	return films, nil
}

func collect(t *testing.T, cfg Config, src Source) (Results, []string, error) {
	t.Helper()
	dial := func(context.Context, func(Event)) (Source, error) { return src, nil }
	var results Results
	var failed []string
	var finished error
	for e := range Run(context.Background(), cfg, dial) {
		switch e := e.(type) {
		case UserScanned:
			results.Add(e.User)
		case UserFailed:
			failed = append(failed, e.Username)
		case Finished:
			finished = e.Err
		}
	}
	return results, failed, finished
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

	results, failed, err := collect(t, cfg, src)
	if err != nil {
		t.Fatal(err)
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
	cfg := Config{Films: top4, Stars: []letterboxd.Rating{10}, MinShared: 3, MaxUsers: 3}

	results, _, err := collect(t, cfg, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Users) != 3 {
		t.Errorf("scanned %d users, want 3", len(results.Users))
	}
}

func TestRunWithoutMatches(t *testing.T) {
	cfg := Config{Films: top4, Stars: []letterboxd.Rating{10}, MinShared: 2}
	if _, _, err := collect(t, cfg, fakeSource{}); err == nil {
		t.Error("want an error when nobody matches")
	}
}

type blockedSource struct{ fakeSource }

func (blockedSource) RatedFilms(context.Context, string, letterboxd.Rating) ([]letterboxd.Film, error) {
	return nil, letterboxd.ErrBlocked
}

func TestRunStopsWhenBlocked(t *testing.T) {
	src := blockedSource{fakeSource{search: map[int][][]string{4: {{"a", "b"}}}}}
	cfg := Config{Films: top4, Stars: []letterboxd.Rating{10}, MinShared: 4}

	_, failed, err := collect(t, cfg, src)
	if !errors.Is(err, letterboxd.ErrBlocked) || len(failed) != 0 {
		t.Errorf("got err %v and failed %v, want ErrBlocked and no skipped users", err, failed)
	}
}
