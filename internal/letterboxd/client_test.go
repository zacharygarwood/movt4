package letterboxd

import (
	"context"
	"strings"
	"testing"
)

// recordingFetcher answers every request with body and remembers the URL.
type recordingFetcher struct {
	body string
	url  string
}

func (f *recordingFetcher) Get(_ context.Context, url string) (string, error) {
	f.url = url
	return f.body, nil
}

const budapestResults = `{"data":[
	{"name":"The Grand Budapest Hotel","releaseYear":2014,"slug":"the-grand-budapest-hotel"},
	{"name":"The Making of The Grand Budapest Hotel","releaseYear":2014,"slug":"the-making-of-the-grand-budapest-hotel"}]}`

func TestFindFilm(t *testing.T) {
	tests := []struct{ query, wantSlug, wantSearch string }{
		// A loose name takes the best match.
		{"grand budpest", "the-grand-budapest-hotel", "q=grand+budpest"},
		// A URL is searched by its slug, and the exact slug wins over the best match.
		{" https://letterboxd.com/film/the-making-of-the-grand-budapest-hotel/ ", "the-making-of-the-grand-budapest-hotel", "q=the+making+of+the+grand+budapest+hotel"},
	}
	for _, tt := range tests {
		fetcher := &recordingFetcher{body: budapestResults}
		film, err := NewClient(nil, fetcher).FindFilm(context.Background(), tt.query)
		if err != nil || film.Slug != tt.wantSlug || !strings.Contains(fetcher.url, tt.wantSearch) {
			t.Errorf("FindFilm(%q) = %+v, %v after requesting %s", tt.query, film, err, fetcher.url)
		}
	}

	if _, err := NewClient(nil, &recordingFetcher{body: `{"data":[]}`}).FindFilm(context.Background(), "zzzz"); err == nil {
		t.Error("want an error when nothing matches")
	}
}
