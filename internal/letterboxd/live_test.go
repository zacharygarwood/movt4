//go:build live

// Live tests hit letterboxd.com to catch markup or Cloudflare changes.
// Run them with: go test -tags live ./internal/letterboxd/

package letterboxd

import (
	"context"
	"testing"
	"time"
)

func TestLiveBrowserFetcher(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	browser, err := NewBrowserFetcher(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	for _, url := range []string{
		baseURL + "/dave/films/rated/5/",
		baseURL + "/dave/films/rated/5/page/2/",
		baseURL + "/s/search/members/(fan:parasite-2019%20fan:whiplash-2014)/",
	} {
		start := time.Now()
		body, err := browser.Get(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d bytes in %s", url, len(body), time.Since(start))
	}
}

func TestLiveClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	browser, err := NewBrowserFetcher(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	client := NewClient(HTTPFetcher{}, browser)

	favorites, err := client.Favorites(ctx, "dave")
	if err != nil || len(favorites) != 4 {
		t.Fatalf("Favorites = %v, %v; want 4 films", favorites, err)
	}

	pages := 0
	seen := map[string]bool{}
	slugs := []string{"parasite-2019", "the-grand-budapest-hotel", "whiplash-2014", "the-lighthouse-2019"}
	for usernames, err := range client.SearchFans(ctx, slugs, 3) {
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range usernames {
			seen[u] = true
		}
		if pages++; pages == 2 {
			break
		}
	}
	if len(seen) < 30 {
		t.Errorf("two search pages found %d members, want at least 30", len(seen))
	}

	films, err := client.RatedFilms(ctx, "dave", 10)
	if err != nil || len(films) <= 72 {
		t.Errorf("RatedFilms = %d films, %v; want more than one page", len(films), err)
	}

	poster, err := client.Poster(ctx, "parasite-2019")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("favorites %v, %d members, %d five-star films, poster %v", favorites, len(seen), len(films), poster.Bounds())
}
