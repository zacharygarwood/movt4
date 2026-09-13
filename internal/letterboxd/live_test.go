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
