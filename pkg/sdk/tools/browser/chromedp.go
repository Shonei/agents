package browser

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const (
	WaitDOMContentLoaded = "domcontentloaded"
	WaitLoad             = "load"
	WaitNetworkIdle      = "networkidle"
)

type FetchOptions struct {
	URL             string
	Wait            string
	Timeout         time.Duration
	WaitForSelector string
	Headless        *bool
	Settle          time.Duration
}

type FetchResult struct {
	RequestedURL string
	FinalURL     string
	Title        string
	HTML         string
	Wait         string
}

type chromeSession struct {
	ctx         context.Context
	cancel      context.CancelFunc
	allocCancel context.CancelFunc
	headless    bool
}

var (
	sessionMu sync.Mutex
	session   *chromeSession
)

// FetchURL renders a page in Chromium and returns its final HTML.
//
// The browser session is intentionally reused for the life of the process so
// users can see the current page and repeated tool calls do not relaunch Chrome.
func FetchURL(opts FetchOptions) (*FetchResult, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 45 * time.Second
	}
	if opts.Wait == "" {
		opts.Wait = WaitLoad
	}
	wait := strings.ToLower(opts.Wait)
	headless := false
	if opts.Headless != nil {
		headless = *opts.Headless
	}

	sessionMu.Lock()
	defer sessionMu.Unlock()

	current, err := getSession(headless)
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(current.ctx, opts.Timeout)
	defer cancel()

	var (
		finalURL string
		title    string
		html     string
	)
	actions := []chromedp.Action{
		chromedp.EmulateViewport(1365, 900),
	}

	switch wait {
	case WaitDOMContentLoaded:
		actions = append(actions,
			navigateWithoutLoadWait(opts.URL),
			chromedp.WaitReady("body", chromedp.ByQuery),
		)
	case WaitLoad:
		actions = append(actions, chromedp.Navigate(opts.URL))
	case WaitNetworkIdle:
		actions = append(actions,
			chromedp.Navigate(opts.URL),
			chromedp.Sleep(750*time.Millisecond),
		)
	default:
		return nil, fmt.Errorf("invalid wait mode %q (expected domcontentloaded, load, or networkidle)", opts.Wait)
	}

	if opts.WaitForSelector != "" {
		actions = append(actions, chromedp.WaitReady(opts.WaitForSelector, chromedp.ByQuery))
	}
	if opts.Settle > 0 {
		actions = append(actions, chromedp.Sleep(opts.Settle))
	}
	actions = append(actions,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)

	if err := chromedp.Run(runCtx, actions...); err != nil {
		return nil, fmt.Errorf("failed to render page: %w", err)
	}
	if finalURL == "" {
		finalURL = opts.URL
	}

	return &FetchResult{
		RequestedURL: opts.URL,
		FinalURL:     finalURL,
		Title:        title,
		HTML:         html,
		Wait:         opts.Wait,
	}, nil
}

func getSession(headless bool) (*chromeSession, error) {
	if session != nil && session.headless == headless {
		return session, nil
	}
	if session != nil {
		session.cancel()
		session.allocCancel()
		session = nil
	}

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("hide-scrollbars", headless),
		chromedp.Flag("mute-audio", headless),
		chromedp.WindowSize(1365, 900),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	ctx, cancel := chromedp.NewContext(allocCtx)

	// Force browser startup now so launch errors are reported by this call.
	if err := chromedp.Run(ctx); err != nil {
		cancel()
		allocCancel()

		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	session = &chromeSession{
		ctx:         ctx,
		cancel:      cancel,
		allocCancel: allocCancel,
		headless:    headless,
	}

	return session, nil
}

func navigateWithoutLoadWait(url string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, errorText, _, err := page.Navigate(url).Do(ctx)
		if err != nil {
			return err
		}
		if errorText != "" {
			return fmt.Errorf("page load error %s", errorText)
		}

		return nil
	})
}
