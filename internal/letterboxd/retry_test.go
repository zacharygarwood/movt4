package letterboxd

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

// fakeFetcher returns errs in order, repeating the last one.
type fakeFetcher struct {
	errs  []error
	calls int
}

func (f *fakeFetcher) Get(context.Context, string) (string, error) {
	err := f.errs[min(f.calls, len(f.errs)-1)]
	f.calls++
	return "", err
}

func TestRetryBlocked(t *testing.T) {
	waits := []time.Duration{time.Millisecond, time.Millisecond}
	tests := []struct {
		name       string
		errs       []error
		wantErr    error
		wantCalls  int
		wantPauses int
	}{
		{"succeeds after a block", []error{ErrBlocked, nil}, nil, 2, 1},
		{"gives up after every wait", []error{ErrBlocked}, ErrBlocked, 3, 2},
		{"other errors aren't retried", []error{io.ErrUnexpectedEOF}, io.ErrUnexpectedEOF, 1, 0},
	}
	for _, tt := range tests {
		f := &fakeFetcher{errs: tt.errs}
		pauses := 0
		_, err := RetryBlocked(f, waits, func(time.Duration) { pauses++ }).Get(context.Background(), "url")
		if !errors.Is(err, tt.wantErr) || f.calls != tt.wantCalls || pauses != tt.wantPauses {
			t.Errorf("%s: err %v after %d calls and %d pauses, want %v after %d and %d",
				tt.name, err, f.calls, pauses, tt.wantErr, tt.wantCalls, tt.wantPauses)
		}
	}
}
