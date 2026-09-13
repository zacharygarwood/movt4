package letterboxd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Fetcher retrieves the body of a page.
type Fetcher interface {
	Get(ctx context.Context, url string) (string, error)
}

const baseURL = "https://letterboxd.com"

// userAgent is sent by every fetcher; Letterboxd challenges obvious bots.
const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36"

// StatusError reports a response other than 200 OK.
type StatusError struct {
	URL  string
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("GET %s: HTTP %d", e.URL, e.Code)
}

// HTTPFetcher fetches pages that Cloudflare does not guard.
type HTTPFetcher struct {
	Client *http.Client
}

func (f HTTPFetcher) Get(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &StatusError{URL: url, Code: resp.StatusCode}
	}
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

// Throttle wraps f so that requests start at least every apart, keeping the
// load on Letterboxd polite. The returned Fetcher is safe for concurrent use.
func Throttle(f Fetcher, every time.Duration) Fetcher {
	return &throttled{fetcher: f, every: every}
}

type throttled struct {
	fetcher Fetcher
	every   time.Duration

	mu   sync.Mutex
	next time.Time
}

func (t *throttled) Get(ctx context.Context, url string) (string, error) {
	t.mu.Lock()
	wait := time.Until(t.next)
	if wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			t.mu.Unlock()
			return "", ctx.Err()
		}
	}
	t.next = time.Now().Add(t.every)
	t.mu.Unlock()
	return t.fetcher.Get(ctx, url)
}
