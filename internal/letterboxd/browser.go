package letterboxd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// challengeTitle is the page title Cloudflare shows while its challenge runs.
const challengeTitle = "Just a moment..."

// ErrBlocked means Cloudflare still refused requests after its challenge,
// which happens for a while after many requests in a short time.
var ErrBlocked = errors.New("Letterboxd's Cloudflare protection is blocking this network for now; wait a few minutes and try again")

// BrowserFetcher fetches Cloudflare-guarded pages through a headless Chromium
// tab. The first guarded request navigates the tab so the challenge can run
// and set its clearance cookie; every request is then made with fetch() from
// inside the page, which reuses that cookie without rendering anything.
type BrowserFetcher struct {
	tab    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex // chromedp tabs run one action at a time
}

// NewBrowserFetcher starts Chromium and opens a tab on letterboxd.com. Call
// Close to shut the browser down.
func NewBrowserFetcher(ctx context.Context) (*BrowserFetcher, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(userAgent),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	tab, cancelTab := chromedp.NewContext(allocCtx)
	b := &BrowserFetcher{tab: tab, cancel: func() { cancelTab(); cancelAlloc() }}

	// The first Run launches the browser and ties its lifetime to the context
	// it is given, so it must use the tab itself rather than a timeout.
	// fetch() then needs a same-origin page; robots.txt is the lightest one.
	err := chromedp.Run(tab)
	if err == nil {
		err = b.run(30*time.Second, chromedp.Navigate(baseURL+"/robots.txt"))
	}
	if err != nil {
		b.Close()
		return nil, fmt.Errorf("starting browser: %w", err)
	}
	return b, nil
}

// Close shuts down the browser.
func (b *BrowserFetcher) Close() { b.cancel() }

func (b *BrowserFetcher) Get(ctx context.Context, url string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	status, body, err := b.fetchInPage(url)
	if err == nil && status == 403 {
		if err := b.solveChallenge(url); err != nil {
			return "", err
		}
		if status, body, err = b.fetchInPage(url); err == nil && status == 403 {
			return "", ErrBlocked
		}
	}
	if err != nil {
		return "", err
	}
	if status != 200 {
		return "", &StatusError{URL: url, Code: status}
	}
	return body, nil
}

func (b *BrowserFetcher) fetchInPage(url string) (int, string, error) {
	quoted, _ := json.Marshal(url)
	script := fmt.Sprintf(`fetch(%s, {credentials: "include"}).then(async r => ({status: r.status, body: await r.text()}))`, quoted)
	var res struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	awaitPromise := func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }
	if err := b.run(30*time.Second, chromedp.Evaluate(script, &res, awaitPromise)); err != nil {
		return 0, "", fmt.Errorf("fetching %s: %w", url, err)
	}
	return res.Status, res.Body, nil
}

// solveChallenge loads url in the tab and waits for Cloudflare's challenge
// page to hand over to the real page.
func (b *BrowserFetcher) solveChallenge(url string) error {
	// Navigate without waiting for a load event, which a challenge page may
	// never fire. The mark set on the current document disappears with it,
	// which tells the old page apart from the one being waited for.
	quoted, _ := json.Marshal(url)
	navigate := fmt.Sprintf(`window.movt4Stale = true; location.href = %s`, quoted)
	if err := b.run(10*time.Second, chromedp.Evaluate(navigate, nil)); err != nil {
		return fmt.Errorf("loading %s: %w", url, err)
	}

	cleared := fmt.Sprintf(`!window.movt4Stale && document.readyState !== "loading" && document.title !== %q`, challengeTitle)
	for deadline := time.Now().Add(45 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		var ok bool
		// Errors are expected while the page is mid-navigation; keep polling.
		if err := b.run(5*time.Second, chromedp.Evaluate(cleared, &ok)); err == nil && ok {
			return nil
		}
	}
	return ErrBlocked
}

func (b *BrowserFetcher) run(timeout time.Duration, actions ...chromedp.Action) error {
	ctx, cancel := context.WithTimeout(b.tab, timeout)
	defer cancel()
	return chromedp.Run(ctx, actions...)
}
