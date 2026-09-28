package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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

	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Context()), 45*time.Second)
	defer cancel()

	want := lastfmPair{artist: artist, title: title}
	song, ok := h.resolveOne(ctx, want, lastfmPair{}, false)
	if !ok {
		song, ok = h.resolveOne(ctx, want, lastfmPair{}, true)
		if !ok {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "no match"})
		}
	}

	rng := strings.TrimSpace(c.Get("Range"))
	resp, err := h.openStreamUpstream(ctx, song.Link, rng)
	if err != nil || resp == nil || resp.StatusCode >= 400 {
		if resp != nil {
			resp.Body.Close()
		}
		fresh, freshed := h.resolveOne(ctx, want, lastfmPair{}, true)
		if !freshed || strings.TrimSpace(fresh.Link) == "" || fresh.Link == song.Link {
			return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "Couldn't fetch stream"})
		}
		resp, err = h.openStreamUpstream(ctx, fresh.Link, rng)
		if err != nil || resp == nil || resp.StatusCode >= 400 {
			if resp != nil {
				resp.Body.Close()
			}
			return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "Upstream stream failed"})
		}
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" || strings.HasPrefix(ct, "text/") {
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

	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer resp.Body.Close()
		_, _ = io.Copy(w, io.LimitReader(resp.Body, 80<<20)) // hard cap ~80MB
		_ = w.Flush()
	})
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
	return h.upstream.Do(req)
}

func streamProxyAllowed(u *url.URL) bool {
	if u == nil || !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	if host == "mp3.pm" || strings.HasSuffix(host, ".mp3.pm") {
		return true
	}
	if strings.Contains(host, "sunproxy") {
		return true
	}
	if host == "mp3mn.net" || strings.HasSuffix(host, ".mp3mn.net") {
		return true
	}
	if host == "musify.club" || strings.HasSuffix(host, ".musify.club") {
		return true
	}
	return false
}

func applyStreamUpstreamHeaders(req *http.Request, u *url.URL) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "*/*")
	host := strings.ToLower(u.Hostname())
	switch {
	case strings.Contains(host, "sunproxy"), strings.Contains(host, "mp3mn"):
		req.Header.Set("Referer", "https://mp3mn.net/")
	case strings.Contains(host, "mp3.pm"):
		req.Header.Set("Referer", "https://mp3.pm/")
	case strings.Contains(host, "musify.club"):
		req.Header.Set("Referer", "https://musify.club/")
	}
}
