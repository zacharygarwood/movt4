package letterboxd

import (
	"context"
	"testing"
	"time"
)

type okFetcher struct{}

func (okFetcher) Get(context.Context, string) (string, error) { return "ok", nil }

func TestThrottlerSlowsDownAndRecovers(t *testing.T) {
	throttle := Throttle(okFetcher{}, time.Microsecond)
	for range 5 {
		throttle.SlowDown()
	}
	if throttle.every != maxSlowdown*time.Microsecond {
		t.Fatalf("after slowing down, every = %s, want %s", throttle.every, maxSlowdown*time.Microsecond)
	}

	for range recoverAfter {
		throttle.Get(context.Background(), "url")
	}
	if want := maxSlowdown / 2 * time.Microsecond; throttle.every != want {
		t.Errorf("after %d requests, every = %s, want %s", recoverAfter, throttle.every, want)
	}
}
