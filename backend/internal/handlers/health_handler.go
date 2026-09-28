package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Musify search HTML puts track rows after a large <head> (~75KB+). Read past that
// so marker checks match the real scrape path (providers use up to 1MiB).
const healthBodyLimit = 512 << 10

// Per-attempt budget. Cold TLS / first-hit challenge pages need more than a tight 8s,
// and we retry once — keep each try under the scrape client's ~12s Timeout.
const healthAttemptTimeout = 10 * time.Second

type sourceSpec struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	URL     string `json:"-"`
	Referer string `json:"-"`
	// Markers — OK if the body contains any one (search scrape path).
	Markers [][]byte `json:"-"`
	// Retries after a failed marker/blocked response (Musify often needs a warm cookie).
	Retries int `json:"-"`
}

type sourceStatus struct {
	Name string `json:"name"`
	Host string `json:"host"`
	OK   bool   `json:"ok"`
	Ms   int64  `json:"ms"`
}

var musicSources = []sourceSpec{
	{
		Name: "Mp3pm",
		Host: "mp3.pm",
		URL:  "https://mp3.pm/",
		Markers: [][]byte{
			[]byte(`cplayer-sound-item`),
			[]byte(`data-sound-url`),
		},
		Retries: 1,
	},
	{
		Name:    "Mp3mn",
		Host:    "mp3mn.net",
		URL:     "https://mp3mn.net/",
		Markers: nil, // homepage reachability only
	},
	{
		Name:    "Musify",
		Host:    "musify.club",
		URL:     "https://musify.club/en/search?searchText=nero&type=song",
		Referer: "https://musify.club/en/",
		Markers: [][]byte{
			[]byte(`tracklist__row`),
			[]byte(`/track/pl/`),
		},
		Retries: 1,
	},
}

type HealthHandler struct {
	client *http.Client
}

func NewHealthHandler(client *http.Client) *HealthHandler {
	return &HealthHandler{client: client}
}

func (hh *HealthHandler) GetSources(c fiber.Ctx) error {
	out := make([]sourceStatus, len(musicSources))
	var wg sync.WaitGroup
	for i, s := range musicSources {
		wg.Add(1)
		go func(i int, s sourceSpec) {
			defer wg.Done()
			out[i] = hh.probe(s)
		}(i, s)
	}
	wg.Wait()
	return c.JSON(fiber.Map{"sources": out})
}

func (hh *HealthHandler) probe(s sourceSpec) sourceStatus {
	st := sourceStatus{Name: s.Name, Host: s.Host}
	start := time.Now()
	attempts := 1 + s.Retries
	if attempts < 1 {
		attempts = 1
	}

	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(time.Duration(200*i) * time.Millisecond)
		}
		if hh.probeOnce(s) {
			st.OK = true
			st.Ms = time.Since(start).Milliseconds()
			return st
		}
	}
	st.Ms = time.Since(start).Milliseconds()
	return st
}

func (hh *HealthHandler) probeOnce(s sourceSpec) bool {
	ctx, cancel := context.WithTimeout(context.Background(), healthAttemptTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return false
	}
	setHealthBrowserHeaders(req, s.Referer)

	resp, err := hh.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, healthBodyLimit))
	if isHealthBlockedStatus(resp.StatusCode) || isHealthBlockedBody(body) {
		return false
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	if len(s.Markers) == 0 {
		return true
	}
	for _, m := range s.Markers {
		if bytes.Contains(body, m) {
			return true
		}
	}
	return false
}

func setHealthBrowserHeaders(req *http.Request, referer string) {
	// Match providers.BaseProvider headers so health and search see the same edge behavior.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
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

func isHealthBlockedStatus(code int) bool {
	switch code {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusBadGateway:
		return true
	default:
		return false
	}
}

func isHealthBlockedBody(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	lower := strings.ToLower(string(body))
	for _, m := range []string{
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
	} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
