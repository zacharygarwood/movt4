package letterboxd

import (
	"context"
	"errors"
	"time"
)

// RetryBlocked wraps f so that a request Cloudflare blocks is tried again
// after each of waits in turn. Blocks lift on their own after a while, so
// waiting beats abandoning a scan. onWait is called before each pause.
func RetryBlocked(f Fetcher, waits []time.Duration, onWait func(time.Duration)) Fetcher {
	return retryBlocked{fetcher: f, waits: waits, onWait: onWait}
}

type retryBlocked struct {
	fetcher Fetcher
	waits   []time.Duration
	onWait  func(time.Duration)
}

func (r retryBlocked) Get(ctx context.Context, url string) (string, error) {
	for attempt := 0; ; attempt++ {
		body, err := r.fetcher.Get(ctx, url)
		if !errors.Is(err, ErrBlocked) || attempt == len(r.waits) {
			return body, err
		}
		r.onWait(r.waits[attempt])
		select {
		case <-time.After(r.waits[attempt]):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}
