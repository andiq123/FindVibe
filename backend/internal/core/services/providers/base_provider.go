package providers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andiq123/FindVibeFiber/internal/utils"
)

const maxHTMLBytes = 1 << 20

var browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

type proxyRotator interface {
	Rotate()
	AttemptBudget(fallback int) int
}

type BaseProvider struct {
	name     string
	priority int
	client   *http.Client
	rotator  proxyRotator
}

func NewBaseProvider(name string, priority int, client *http.Client) *BaseProvider {
	return &BaseProvider{name: name, priority: priority, client: client}
}

// WithRotator attaches a scrape proxy pool (optional).
func (bp *BaseProvider) WithRotator(r proxyRotator) *BaseProvider {
	bp.rotator = r
	return bp
}

func (bp *BaseProvider) Name() string  { return bp.name }
func (bp *BaseProvider) Priority() int { return bp.priority }

func (bp *BaseProvider) fetchDocument(ctx context.Context, rawURL, referer string) (*goquery.Document, error) {
	attempts := 2
	if bp.rotator != nil {
		attempts = bp.rotator.AttemptBudget(2)
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			// Brief pause before retry / proxy hop.
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("%s: %w", bp.name, ctx.Err())
			case <-time.After(time.Duration(150*i) * time.Millisecond):
			}
		}

		doc, blocked, err := bp.doFetch(ctx, rawURL, referer)
		if err == nil && !blocked {
			return doc, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("%s: blocked or challenge page", bp.name)
		}
		if bp.rotator != nil {
			bp.rotator.Rotate()
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%s: fetch failed", bp.name)
	}
	return nil, lastErr
}

func (bp *BaseProvider) doFetch(ctx context.Context, rawURL, referer string) (*goquery.Document, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("%s: request: %w", bp.name, err)
	}
	setBrowserHeaders(req, referer)

	resp, err := bp.client.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("%s: fetch: %w", bp.name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTMLBytes))
	if err != nil {
		return nil, true, fmt.Errorf("%s: read: %w", bp.name, err)
	}

	if isBlockedStatus(resp.StatusCode) || isBlockedBody(body) {
		return nil, true, fmt.Errorf("%s: status %d (blocked)", bp.name, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode >= 500, fmt.Errorf("%s: status %d", bp.name, resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, false, fmt.Errorf("%s: parse: %w", bp.name, err)
	}
	return doc, false, nil
}

func setBrowserHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,ru;q=0.8")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")
	if referer != "" {
		req.Header.Set("Referer", referer)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
	} else {
		req.Header.Set("Sec-Fetch-Site", "none")
	}
}

func isBlockedStatus(code int) bool {
	switch code {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusBadGateway:
		return true
	default:
		return false
	}
}

func isNotFoundStatus(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "status 404")
}

func isBlockedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "(blocked)") || strings.Contains(msg, "blocked or challenge")
}

func isBlockedBody(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	lower := strings.ToLower(string(body))
	markers := []string{
		"cf-browser-verification",
		"cf-challenge",
		"challenge-platform",
		"just a moment",
		"attention required",
		"access denied",
		"sorry, you have been blocked",
		"verify you are human",
		"captcha",
		"ddos-guard",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func absoluteURL(raw string) string {
	return utils.UpgradeHTTPS(raw)
}

// playableLink — stream URLs we hand to iOS AVPlayer (https only).
func playableLink(raw string) string {
	u := absoluteURL(raw)
	if !strings.HasPrefix(u, "https://") {
		return ""
	}
	return u
}

func text(s *goquery.Selection) string {
	return strings.TrimSpace(s.Text())
}
