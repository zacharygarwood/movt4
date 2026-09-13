package letterboxd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// challengeTitle is the page title Cloudflare shows while its challenge runs.
const challengeTitle = "Just a moment..."

// ErrBlocked means Cloudflare still refused requests after its challenge,
// which happens for a while after many requests in a short time.
var ErrBlocked = errors.New("Letterboxd's Cloudflare protection is blocking this network for now; wait a few minutes and try again")

// errPageTimeout means a page didn't finish loading in time.
var errPageTimeout = errors.New("page didn't finish loading")

// headlessToken is how a headless browser names itself in its user agent.
var headlessToken = regexp.MustCompile(`HeadlessChrome/(\d+)[\d.]*`)

// BrowserFetcher fetches Cloudflare-guarded pages through a headless Chromium
// tab. Requests are made with fetch() from inside a Letterboxd page, which
// reuses the browser's cookies without rendering anything; a request that
// meets Cloudflare's challenge is loaded in the tab so the challenge can run.
//
// Cloudflare blocks browsers that look automated, so the tab reports the
// browser's real version without the headless marker, has a desktop-sized
// window, and keeps its profile, and with it Cloudflare's cookies, between
// runs.
type BrowserFetcher struct {
	tab    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex // chromedp tabs run one action at a time
}

// NewBrowserFetcher starts Chromium with its profile in profileDir and opens
// a tab on letterboxd.com. Call Close to shut the browser down.
func NewBrowserFetcher(ctx context.Context, profileDir string) (*BrowserFetcher, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserDataDir(profileDir),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	// chromedp logs browser events it doesn't handle to stderr, which would
	// draw over the terminal UI.
	quiet := func(string, ...any) {}
	tab, cancelTab := chromedp.NewContext(allocCtx, chromedp.WithLogf(quiet), chromedp.WithErrorf(quiet))
	b := &BrowserFetcher{tab: tab, cancel: func() { cancelTab(); cancelAlloc() }}

	// The first Run launches the browser and ties its lifetime to the context
	// it is given, so it must use the tab itself rather than a timeout.
	err := chromedp.Run(tab)
	if err == nil {
		err = b.run(10*time.Second, chromedp.ActionFunc(hideHeadless))
	}
	if err == nil {
		// Arrive on the home page like a visitor would. This also gives
		// fetch() a same-origin page to run from.
		err = b.navigate(baseURL+"/", 45*time.Second)
	}
	if err != nil {
		b.Close()
		return nil, fmt.Errorf("starting browser: %w", err)
	}
	return b, nil
}

// hideHeadless makes the tab report the browser's own user agent and client
// hints, minus the marker that gives away a headless browser. Chrome reports
// only its major version in the user agent, e.g. "Chrome/147.0.0.0".
func hideHeadless(ctx context.Context) error {
	_, _, _, userAgent, _, err := browser.GetVersion().Do(ctx)
	if err != nil {
		return err
	}
	// Client hints are only exposed to secure pages, and a blank tab isn't one.
	// chrome://version is, without a network request.
	if err := chromedp.Navigate("chrome://version").Do(ctx); err != nil {
		return err
	}
	var hints emulation.UserAgentMetadata
	highEntropy := `navigator.userAgentData.getHighEntropyValues(["architecture", "bitness", "fullVersionList", "model", "platformVersion", "wow64"])`
	if err := chromedp.Evaluate(highEntropy, &hints, awaitPromise).Do(ctx); err != nil {
		return fmt.Errorf("reading client hints: %w", err)
	}
	userAgent = headlessToken.ReplaceAllString(userAgent, "Chrome/${1}.0.0.0")
	return emulation.SetUserAgentOverride(userAgent).WithUserAgentMetadata(&hints).Do(ctx)
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
	if err := b.run(30*time.Second, chromedp.Evaluate(script, &res, awaitPromise)); err != nil {
		return 0, "", fmt.Errorf("fetching %s: %w", url, err)
	}
	return res.Status, res.Body, nil
}

// solveChallenge loads url in the tab so Cloudflare's challenge can run.
func (b *BrowserFetcher) solveChallenge(url string) error {
	err := b.navigate(url, 20*time.Second)
	if errors.Is(err, errPageTimeout) {
		return ErrBlocked
	}
	return err
}

// navigate loads url in the tab and waits until the new page is parsed and
// isn't Cloudflare's challenge. It doesn't wait for the load event, which
// ads or a challenge can hold up for a long time.
func (b *BrowserFetcher) navigate(url string, timeout time.Duration) error {
	// Mark the current document. The mark disappears with it, which tells the
	// old page apart from the new one.
	start := chromedp.ActionFunc(func(ctx context.Context) error {
		if err := chromedp.Evaluate(`window.movt4Stale = true`, nil).Do(ctx); err != nil {
			return err
		}
		_, _, errorText, _, err := page.Navigate(url).Do(ctx)
		if err == nil && errorText != "" {
			err = errors.New(errorText)
		}
		return err
	})
	if err := b.run(10*time.Second, start); err != nil {
		return fmt.Errorf("loading %s: %w", url, err)
	}

	ready := fmt.Sprintf(`!window.movt4Stale && document.readyState !== "loading" && document.title !== %q`, challengeTitle)
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		var ok bool
		// Errors are expected while the page is mid-navigation; keep polling.
		if err := b.run(5*time.Second, chromedp.Evaluate(ready, &ok)); err == nil && ok {
			return nil
		}
	}
	var title string
	b.run(5*time.Second, chromedp.Title(&title))
	return fmt.Errorf("loading %s (stuck on %q): %w", url, title, errPageTimeout)
}

func (b *BrowserFetcher) run(timeout time.Duration, actions ...chromedp.Action) error {
	ctx, cancel := context.WithTimeout(b.tab, timeout)
	defer cancel()
	return chromedp.Run(ctx, actions...)
}

func awaitPromise(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithAwaitPromise(true)
}
