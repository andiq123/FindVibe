package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/gofiber/fiber/v3"
)

func (h *RecommendHandler) GetExploreCache(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	return c.JSON(h.exploreCacheStatus())
}

// GET /explore?refresh=1 → chart shelves (Romania / Worldwide / vibes), server-cached 24h.
// GET /explore?stream=1 → NDJSON: meta, then one section line as each shelf is ready, then done.
func (h *RecommendHandler) GetExplore(c fiber.Ctx) error {
	if h.apiKey == "" {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "LASTFM_API_KEY not set"})
	}
	refresh := c.Query("refresh") == "1" || strings.EqualFold(c.Query("refresh"), "true")
	stream := c.Query("stream") == "1" || strings.EqualFold(c.Query("stream"), "true")
	if stream {
		return h.streamExplore(c, refresh)
	}

	if !refresh {
		if sections, ok := h.exploreSnap(true); ok {
			c.Set("Cache-Control", "public, max-age="+exploreCacheAge)
			return c.JSON(fiber.Map{"country": exploreCountry, "sections": sections, "cached": true})
		}
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Context()), exploreStreamBudget)
	defer cancel()
	sections, err := h.coldExplore(ctx, refresh, nil)
	if err != nil || len(sections) == 0 {
		// Serve last good payload even past TTL — better than empty Explore.
		if stale, ok := h.exploreSnap(false); ok {
			c.Set("Cache-Control", "public, max-age=60")
			return c.JSON(fiber.Map{"country": exploreCountry, "sections": stale, "cached": true})
		}
		if err != nil {
			return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "Couldn't load charts"})
		}
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "No playable charts right now"})
	}
	c.Set("Cache-Control", "public, max-age="+exploreCacheAge)
	return c.JSON(fiber.Map{"country": exploreCountry, "sections": sections, "cached": false})
}

type exploreStreamEvent struct {
	Type    string          `json:"type"`
	Country string          `json:"country,omitempty"`
	Cached  *bool           `json:"cached,omitempty"`
	Section *ExploreSection `json:"section,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func (h *RecommendHandler) streamExplore(c fiber.Ctx, refresh bool) error {
	c.Set("Content-Type", "application/x-ndjson")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	return c.SendStreamWriter(func(w *bufio.Writer) {
		enc := json.NewEncoder(w)
		write := func(ev exploreStreamEvent) bool {
			if err := enc.Encode(ev); err != nil {
				return false
			}
			return w.Flush() == nil
		}
		boolPtr := func(v bool) *bool { return &v }

		if !refresh {
			if sections, ok := h.exploreSnap(true); ok {
				if !write(exploreStreamEvent{Type: "meta", Country: exploreCountry, Cached: boolPtr(true)}) {
					return
				}
				for i := range sections {
					sec := sections[i]
					if !write(exploreStreamEvent{Type: "section", Section: &sec}) {
						return
					}
				}
				_ = write(exploreStreamEvent{Type: "done", Cached: boolPtr(true)})
				return
			}
		}

		if !write(exploreStreamEvent{Type: "meta", Country: exploreCountry, Cached: boolPtr(false)}) {
			return
		}

		// Detach cancel so a brief client blip doesn't abort in-flight resolve;
		// Flush errors still stop the stream when the client is gone.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Context()), exploreStreamBudget)
		defer cancel()

		// Leader streams shelves as they resolve; followers wait on singleflight then flush.
		var streamed int
		sections, err, shared := h.coldExploreShared(ctx, refresh, func(sec ExploreSection) error {
			streamed++
			if !write(exploreStreamEvent{Type: "section", Section: &sec}) {
				return context.Canceled
			}
			return nil
		})
		if shared {
			for i := range sections {
				sec := sections[i]
				if !write(exploreStreamEvent{Type: "section", Section: &sec}) {
					return
				}
			}
		}
		if err != nil && len(sections) == 0 && streamed == 0 {
			if stale, ok := h.exploreSnap(false); ok {
				for i := range stale {
					sec := stale[i]
					if !write(exploreStreamEvent{Type: "section", Section: &sec}) {
						return
					}
				}
				_ = write(exploreStreamEvent{Type: "done", Cached: boolPtr(true)})
				return
			}
			_ = write(exploreStreamEvent{Type: "error", Error: "Couldn't load charts"})
			return
		}
		if len(sections) == 0 && streamed == 0 {
			if stale, ok := h.exploreSnap(false); ok {
				for i := range stale {
					sec := stale[i]
					if !write(exploreStreamEvent{Type: "section", Section: &sec}) {
						return
					}
				}
				_ = write(exploreStreamEvent{Type: "done", Cached: boolPtr(true)})
				return
			}
			_ = write(exploreStreamEvent{Type: "error", Error: "No playable charts right now"})
			return
		}
		_ = write(exploreStreamEvent{Type: "done", Cached: boolPtr(false)})
	})
}

// coldExplore builds chart shelves once per cold window (singleflight).
func (h *RecommendHandler) coldExplore(ctx context.Context, refresh bool, onSection func(ExploreSection) error) ([]ExploreSection, error) {
	sections, err, _ := h.coldExploreShared(ctx, refresh, onSection)
	return sections, err
}

func (h *RecommendHandler) coldExploreShared(
	ctx context.Context,
	refresh bool,
	onSection func(ExploreSection) error,
) ([]ExploreSection, error, bool) {
	key := "explore"
	if refresh {
		key = "explore-refresh"
	}
	v, err, shared := h.exploreSF.Do(key, func() (any, error) {
		sections, err := h.buildExplore(ctx, onSection)
		// Keep a useful day cache even if one shelf fails — empty builds never overwrite.
		if len(sections) >= 1 {
			h.exploreStore(sections)
		}
		return sections, err
	})
	sections, _ := v.([]ExploreSection)
	return sections, err, shared
}

type exploreJob struct {
	id, title, subtitle string
	q                   url.Values
}

// buildExplore resolves shelves one at a time. onSection is called as each shelf is ready (stream path).
func (h *RecommendHandler) buildExplore(ctx context.Context, onSection func(ExploreSection) error) ([]ExploreSection, error) {
	limit := strconv.Itoa(exploreFetch)
	jobs := []exploreJob{
		{"romania", "Romania", "Charts at home", url.Values{
			"method": {"geo.getTopTracks"}, "country": {exploreCountry}, "limit": {limit},
		}},
		// Closest free viral proxy for RO — YouTube Music trending via kworb.
		{"viral-ro", "Viral Romania", "Rising right now", nil},
		{"worldwide", "Worldwide", "Global chart", url.Values{
			"method": {"chart.getTopTracks"}, "limit": {limit},
		}},
		{"pop", "Pop", "Everywhere right now", url.Values{
			"method": {"tag.getTopTracks"}, "tag": {"pop"}, "limit": {limit},
		}},
		{"hip-hop", "Hip-Hop", "Worldwide vibe", url.Values{
			"method": {"tag.getTopTracks"}, "tag": {"hip-hop"}, "limit": {limit},
		}},
		{"rock", "Rock", "Worldwide vibe", url.Values{
			"method": {"tag.getTopTracks"}, "tag": {"rock"}, "limit": {limit},
		}},
		{"electronic", "Electronic", "Worldwide vibe", url.Values{
			"method": {"tag.getTopTracks"}, "tag": {"electronic"}, "limit": {limit},
		}},
	}

	// ponytail: one shelf at a time — parallel resolve floods providers and returns 1 song.
	out := make([]ExploreSection, 0, len(jobs))
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		sectionCtx, cancel := context.WithTimeout(ctx, exploreSectionBudget)
		var pairs []lastfmPair
		var err error
		if job.id == "viral-ro" {
			pairs, err = h.kworbROViral(sectionCtx)
		} else {
			pairs, err = h.lastfmTracks(sectionCtx, job.q, "tracks")
		}
		if err != nil || len(pairs) == 0 {
			cancel()
			continue
		}
		songs := h.resolveN(sectionCtx, pairs, lastfmPair{}, exploreCap, false, false)
		cancel()
		if len(songs) == 0 {
			continue
		}
		// Short cover budget after resolve — don't let iTunes eat the section timeout.
		coverCtx, coverCancel := context.WithTimeout(context.WithoutCancel(ctx), exploreCoverBudget)
		h.covers.FillSongs(coverCtx, songs)
		coverCancel()

		sec := ExploreSection{
			ID: job.id, Title: job.title, Subtitle: job.subtitle, Songs: songs,
		}
		out = append(out, sec)
		if onSection != nil {
			// Best-effort stream — a dropped client must not abort the shared cold build.
			_ = onSection(sec)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// YouTube Music trending Romania via kworb (public HTML). Closest free stand-in for TikTok RO.
const kworbROViralURL = "https://kworb.net/youtube/trending/ro.html"

func (h *RecommendHandler) kworbROViral(ctx context.Context) ([]lastfmPair, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kworbROViralURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FindVibe/1.0")
	req.Header.Set("Accept", "text/html")
	res, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kworb viral: HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return parseKworbMusicPairs(string(body)), nil
}

var kworbAnchorRe = regexp.MustCompile(`(?i)<a[^>]*>([^<]+)</a>`)

// parseKworbMusicPairs reads the music-only table (not overall trending with gaming/vlogs).
func parseKworbMusicPairs(html string) []lastfmPair {
	start := strings.Index(html, `class="music"`)
	if start < 0 {
		return nil
	}
	chunk := html[start:]
	if i := strings.Index(chunk, "</table>"); i >= 0 {
		chunk = chunk[:i]
	}
	seen := map[string]bool{}
	out := make([]lastfmPair, 0, exploreFetch)
	for _, m := range kworbAnchorRe.FindAllStringSubmatch(chunk, -1) {
		if len(out) >= exploreFetch {
			break
		}
		artist, title, ok := parseChartTitle(m[1])
		if !ok {
			continue
		}
		k := songKey(artist, title)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, lastfmPair{artist: artist, title: title})
	}
	return out
}

// "VANILLA x ALEX VELEA - 7 din 7 | Official Video" → ("VANILLA", "7 din 7")
func parseChartTitle(raw string) (artist, title string, ok bool) {
	raw = strings.TrimSpace(htmlUnescape(raw))
	if raw == "" {
		return "", "", false
	}
	if i := strings.Index(raw, "|"); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
	}
	sep := " - "
	i := strings.Index(raw, sep)
	if i < 0 {
		sep = " – "
		i = strings.Index(raw, sep)
	}
	if i <= 0 {
		return "", "", false
	}
	artist = chartPrimaryArtist(strings.TrimSpace(raw[:i]))
	title = strings.TrimSpace(raw[i+len(sep):])
	title = parenRe.ReplaceAllString(title, " ")
	title = spaceRe.ReplaceAllString(strings.TrimSpace(title), " ")
	if artist == "" || title == "" {
		return "", "", false
	}
	return artist, title, true
}

func chartPrimaryArtist(a string) string {
	for _, sep := range []string{" x ", " ❌", " ✘", " × ", " & ", " feat.", " ft.", " featuring "} {
		if i := strings.Index(strings.ToLower(a), strings.ToLower(sep)); i > 0 {
			return strings.TrimSpace(a[:i])
		}
	}
	return strings.TrimSpace(a)
}

func htmlUnescape(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	return s
}

func (h *RecommendHandler) exploreSnap(freshOnly bool) ([]ExploreSection, bool) {
	h.exploreMu.Lock()
	defer h.exploreMu.Unlock()
	if len(h.exploreSections) == 0 {
		return nil, false
	}
	if freshOnly && time.Since(h.exploreAt) > exploreTTL {
		return nil, false
	}
	out := make([]ExploreSection, len(h.exploreSections))
	copy(out, h.exploreSections)
	return out, true
}

func (h *RecommendHandler) exploreStore(sections []ExploreSection) {
	h.exploreMu.Lock()
	defer h.exploreMu.Unlock()
	h.exploreSections = append([]ExploreSection(nil), sections...)
	h.exploreAt = time.Now()
}

// exploreCacheStatus is the authoritative Explore chart cache view (in-memory on the API).
func (h *RecommendHandler) exploreCacheStatus() fiber.Map {
	h.exploreMu.Lock()
	defer h.exploreMu.Unlock()

	ttlSec := int64(exploreTTL / time.Second)
	shelves := make([]string, 0, len(h.exploreSections))
	songs := 0
	for _, sec := range h.exploreSections {
		if t := strings.TrimSpace(sec.Title); t != "" {
			shelves = append(shelves, t)
		} else if id := strings.TrimSpace(sec.ID); id != "" {
			shelves = append(shelves, id)
		}
		songs += len(sec.Songs)
	}

	if len(h.exploreSections) == 0 || h.exploreAt.IsZero() {
		return fiber.Map{
			"country":          exploreCountry,
			"cached":           false,
			"fresh":            false,
			"createdAt":        nil,
			"expiresAt":        nil,
			"ttlSeconds":       ttlSec,
			"remainingSeconds": int64(0),
			"ageSeconds":       int64(0),
			"sections":         0,
			"songs":            0,
			"shelves":          shelves,
		}
	}

	age := time.Since(h.exploreAt)
	remaining := exploreTTL - age
	if remaining < 0 {
		remaining = 0
	}
	created := h.exploreAt.UTC().Format(time.RFC3339)
	expires := h.exploreAt.Add(exploreTTL).UTC().Format(time.RFC3339)
	return fiber.Map{
		"country":          exploreCountry,
		"cached":           true,
		"fresh":            remaining > 0,
		"createdAt":        created,
		"expiresAt":        expires,
		"ttlSeconds":       ttlSec,
		"remainingSeconds": int64(remaining / time.Second),
		"ageSeconds":       int64(age / time.Second),
		"sections":         len(h.exploreSections),
		"songs":            songs,
		"shelves":          shelves,
	}
}

func (h *RecommendHandler) recommendSnap(key string) ([]domain.Song, bool) {
	if key == "" {
		return nil, false
	}
	h.recommendMu.Lock()
	defer h.recommendMu.Unlock()
	e, ok := h.recommendCache[key]
	if !ok || len(e.songs) == 0 || time.Since(e.at) > recommendTTL {
		return nil, false
	}
	out := make([]domain.Song, len(e.songs))
	copy(out, e.songs)
	return out, true
}

func (h *RecommendHandler) recommendStale(key string) ([]domain.Song, bool) {
	if key == "" {
		return nil, false
	}
	h.recommendMu.Lock()
	defer h.recommendMu.Unlock()
	e, ok := h.recommendCache[key]
	if !ok || len(e.songs) == 0 {
		return nil, false
	}
	out := make([]domain.Song, len(e.songs))
	copy(out, e.songs)
	return out, true
}

func (h *RecommendHandler) recommendStore(key string, songs []domain.Song) {
	if key == "" || len(songs) == 0 {
		return
	}
	h.recommendMu.Lock()
	defer h.recommendMu.Unlock()
	if h.recommendCache == nil {
		h.recommendCache = make(map[string]recommendEntry)
	}
	// ponytail: bounded map — drop expired, else wipe when full
	if len(h.recommendCache) >= recommendCacheCap {
		for k, e := range h.recommendCache {
			if time.Since(e.at) > recommendTTL {
				delete(h.recommendCache, k)
			}
		}
		if len(h.recommendCache) >= recommendCacheCap {
			h.recommendCache = make(map[string]recommendEntry, recommendCacheCap/2)
		}
	}
	cp := make([]domain.Song, len(songs))
	copy(cp, songs)
	h.recommendCache[key] = recommendEntry{songs: cp, at: time.Now()}
}

// GET /similar-artists?artist= → {artists: string[]} from Last.fm artist.getSimilar.
