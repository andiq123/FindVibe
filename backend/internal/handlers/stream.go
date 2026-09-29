package handlers

import (
	"context"
	"fmt"
	"github.com/andiq123/FindVibeFiber/internal/core/services"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"

	"github.com/gofiber/fiber/v3"
)

// GET /stream?artist=&title= → proxy CDN bytes (fallback when the phone can't hotlink).
// Cache-first resolve; re-resolve once if the cached link is dead.
// Not the default play path — clients must try the direct song.link first.
func (h *RecommendHandler) GetStream(c fiber.Ctx) error {
	artist := strings.TrimSpace(c.Query("artist"))
	title := strings.TrimSpace(c.Query("title"))
	if artist == "" || title == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "artist and title required"})
	}
	if h.upstream == nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "stream proxy unavailable"})
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Context()), 5*time.Minute)
	streaming := false
	defer func() {
		if !streaming {
			cancel()
		}
	}()

	want := lastfmPair{artist: artist, title: title}
	song := domain.Song{Link: strings.TrimSpace(c.Query("link"))}
	ok := song.Link != ""
	if !ok {
		song, ok = h.resolveOne(ctx, want, lastfmPair{}, false)
	}
	if !ok {
		song, ok = h.resolveOne(ctx, want, lastfmPair{}, true)
		if !ok {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "no match"})
		}
	}

	rng := strings.TrimSpace(c.Get("Range"))
	resp, err := h.openStreamUpstream(ctx, song.Link, rng)
	if err != nil || resp == nil || !validAudioResponse(resp) {
		if resp != nil {
			resp.Body.Close()
		}
		resp = nil
		// Search all sources and never retry the exact failed URL. Prefer another provider.
		recoveryCtx, recoveryCancel := context.WithTimeout(ctx, 25*time.Second)
		defer recoveryCancel()
		candidates, searchErr := h.search.Search(recoveryCtx, artist+" "+title, 1)
		if searchErr == nil && candidates != nil {
			// Search cards are deduplicated, but every alternate audio source stays available.
			expanded := make([]domain.Song, 0, len(candidates.Songs)*2)
			for _, candidate := range candidates.Songs {
				expanded = append(expanded, candidate)
				for _, alternate := range candidate.Alternatives {
					copy := candidate
					copy.Link = alternate.Link
					copy.Provider = alternate.Provider
					copy.Alternatives = nil
					expanded = append(expanded, copy)
				}
			}
			candidates.Songs = expanded
			failedURL, _ := url.Parse(song.Link)
			providerHost := func(link string) string {
				u, _ := url.Parse(link)
				if u == nil {
					return ""
				}
				host := u.Hostname()
				for _, suffix := range []string{"kachevo.org", "hitmoz.com", "deliciousbananas.com", "deliciouspeaches.com"} {
					if host == suffix || strings.HasSuffix(host, "."+suffix) {
						return suffix
					}
				}
				return host
			}
			failedHost := ""
			if failedURL != nil {
				failedHost = providerHost(failedURL.String())
			}
			sort.SliceStable(candidates.Songs, func(i, j int) bool {
				return providerHost(candidates.Songs[i].Link) != failedHost && providerHost(candidates.Songs[j].Link) == failedHost
			})
			tried := map[string]bool{song.Link: true}
			attempts := 0
			for _, candidate := range candidates.Songs {
				if tried[candidate.Link] || !services.IsPlayableMatch(artist, title, candidate) {
					continue
				}
				tried[candidate.Link] = true
				attempts++
				// Use the full stream context so successful audio survives recovery returning.
				next, openErr := h.openStreamUpstream(ctx, candidate.Link, rng)
				if openErr == nil && next != nil && validAudioResponse(next) {
					resp = next
					h.resolveStore(songKey(artist, title), candidate)
					break
				}
				if next != nil {
					next.Body.Close()
				}
				if attempts >= 4 || recoveryCtx.Err() != nil {
					break
				}
			}
		}
		if resp == nil {
			return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "No working source found"})
		}
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "audio/mpeg"
	}
	c.Set("Content-Type", ct)
	c.Set("Cache-Control", "private, no-store")
	c.Set("Accept-Ranges", "bytes")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Set("Content-Length", cl)
	}
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		c.Set("Content-Range", cr)
	}
	c.Status(resp.StatusCode)

	streaming = true
	return c.SendStream(&audioStreamBody{Reader: io.LimitReader(resp.Body, 80<<20), body: resp.Body, cancel: cancel}, int(resp.ContentLength))
}

// Fiber closes this body after sending, not when the handler returns.
type audioStreamBody struct {
	io.Reader
	body   io.Closer
	cancel context.CancelFunc
}

func (b *audioStreamBody) Close() error {
	defer b.cancel()
	return b.body.Close()
}

func validAudioResponse(resp *http.Response) bool {
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	return (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent) &&
		resp.ContentLength <= 80<<20 &&
		(ct == "" || strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "application/octet-stream"))
}

func (h *RecommendHandler) openStreamUpstream(ctx context.Context, link, rangeHeader string) (*http.Response, error) {
	link = strings.TrimSpace(link)
	upstreamURL, err := url.Parse(link)
	if err != nil || !streamProxyAllowed(upstreamURL) {
		return nil, fmt.Errorf("stream host not allowed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	applyStreamUpstreamHeaders(req, upstreamURL)
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	client := *h.upstream
	client.Timeout = 0 // The stream context owns the full-body deadline.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || !streamProxyAllowed(req.URL) {
			return fmt.Errorf("stream redirect not allowed")
		}
		applyStreamUpstreamHeaders(req, req.URL)
		return nil
	}
	return client.Do(req)
}

func streamProxyAllowed(u *url.URL) bool {
	if u == nil || !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	for _, source := range []string{"mp3.pm", "mp3mn.net", "musify.club", "sunproxy.net"} {
		if host == source || strings.HasSuffix(host, "."+source) {
			return true
		}
	}
	allowedHost := host == "new.kachevo.org" || host == "eu.hitmoz.com"
	// These are the providers' audio redirect hosts, not additional search sources.
	for _, cdn := range []string{"deliciousbananas.com", "deliciouspeaches.com"} {
		allowedHost = allowedHost || host == cdn || strings.HasSuffix(host, "."+cdn)
	}
	return allowedHost && strings.HasPrefix(u.Path, "/get/music/")
}

func applyStreamUpstreamHeaders(req *http.Request, u *url.URL) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "*/*")
	host := u.Hostname()
	switch {
	case strings.HasSuffix(host, "mp3.pm"):
		host = "mp3.pm"
	case strings.HasSuffix(host, "sunproxy.net"), strings.HasSuffix(host, "mp3mn.net"):
		host = "mp3mn.net"
	case strings.HasSuffix(host, "musify.club"):
		host = "musify.club"
	}
	req.Header.Set("Referer", "https://"+host+"/")
}
