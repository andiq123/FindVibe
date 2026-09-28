package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/core/ports"
	"github.com/andiq123/FindVibeFiber/internal/core/services"
	"github.com/andiq123/FindVibeFiber/internal/utils"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/sync/singleflight"
)

const (
	recommendResolveCap = 10 // Explore "Because" rails — longer browse before radio
	/** Radio wants a longer same-vibe batch so the queue doesn't pivot every extend. */
	radioResolveCap = 12
	/** Album play only needs a short queue to start — full resolve burns the gateway budget. */
	albumResolveCap = 4
	recommendSearchPeek = 8
	recommendTTL        = 6 * time.Hour
	recommendCacheCap   = 64
	pairsTTL            = 6 * time.Hour
	pairsCacheCap       = 64

	// ponytail: region hard-coded until settings grow a country picker
	exploreCountry  = "Romania"
	exploreTTL      = 24 * time.Hour
	exploreCacheAge = "86400" // Cache-Control max-age for fresh chart payloads
	// Smaller shelves → faster first paint when streaming section-by-section.
	exploreCap   = 8
	exploreFetch = 12
	// One stuck chart must not block the rest of the stream.
	exploreSectionBudget = 12 * time.Second
	exploreCoverBudget   = 2500 * time.Millisecond
	exploreStreamBudget  = 75 * time.Second

	resolveTTL      = 15 * time.Minute
	resolveCacheCap = 256

	albumTracksBudget = 45 * time.Second
)

type recommendEntry struct {
	songs []domain.Song
	at    time.Time
}

type pairsEntry struct {
	pairs []lastfmPair
	at    time.Time
}

type resolveEntry struct {
	song domain.Song
	at   time.Time
}

type ExploreSection struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Subtitle string        `json:"subtitle"`
	Songs    []domain.Song `json:"songs"`
}

var (
	parenRe = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)
	spaceRe = regexp.MustCompile(`\s+`)
)

type RecommendHandler struct {
	client   *http.Client
	upstream *http.Client // media proxy upstream (scrape client when available)
	apiKey   string
	search   ports.ISearchService
	covers   *services.CoverService

	exploreMu       sync.Mutex
	exploreSections []ExploreSection
	exploreAt       time.Time
	exploreSF       singleflight.Group

	recommendMu    sync.Mutex
	recommendCache map[string]recommendEntry

	pairsMu     sync.Mutex
	pairsCache  map[string]pairsEntry
	pairsSF     singleflight.Group
	recommendSF singleflight.Group

	resolveMu    sync.Mutex
	resolveCache map[string]resolveEntry
}

func NewRecommendHandler(client *http.Client, apiKey string, search ports.ISearchService, covers *services.CoverService) *RecommendHandler {
	return NewRecommendHandlerUpstream(client, client, apiKey, search, covers)
}

// NewRecommendHandlerUpstream uses `upstream` for media proxy fetches (prefer scrape client w/ cookies).
func NewRecommendHandlerUpstream(
	client *http.Client,
	upstream *http.Client,
	apiKey string,
	search ports.ISearchService,
	covers *services.CoverService,
) *RecommendHandler {
	if upstream == nil {
		upstream = client
	}
	return &RecommendHandler{
		client: client, upstream: upstream, apiKey: apiKey, search: search, covers: covers,
		recommendCache: make(map[string]recommendEntry),
		pairsCache:     make(map[string]pairsEntry),
		resolveCache:   make(map[string]resolveEntry),
	}
}

// GET /explore/cache → server chart-cache status (TTL remaining, shelf counts). No client chart store.
func (h *RecommendHandler) GetSimilarArtists(c fiber.Ctx) error {
	if h.apiKey == "" {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "LASTFM_API_KEY not set"})
	}
	artist := strings.TrimSpace(c.Query("artist"))
	if artist == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "artist required"})
	}
	names, err := h.lastfmSimilarArtists(c.Context(), artist)
	if err != nil {
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "Couldn't load similar artists"})
	}
	if len(names) > 12 {
		names = names[:12]
	}
	return c.JSON(fiber.Map{"artists": names})
}

// GET /artist-albums?artist=&page= → {artist, albums, pagination} via Last.fm artist.getTopAlbums.
func (h *RecommendHandler) GetArtistAlbums(c fiber.Ctx) error {
	if h.apiKey == "" {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "LASTFM_API_KEY not set"})
	}
	artist := strings.TrimSpace(c.Query("artist"))
	if artist == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "artist required"})
	}
	page := 1
	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil {
			page = p
		}
	}
	if err := utils.ValidatePage(page); err != nil {
		return HandleError(c, err)
	}
	albums, pag, err := h.lastfmTopAlbums(c.Context(), artist, page, 20)
	if err != nil {
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "Couldn't load albums"})
	}
	return c.JSON(fiber.Map{
		"artist":     artist,
		"albums":     albums,
		"pagination": pag,
	})
}

// GET /album-tracks?artist=&album= → playable Songs from Last.fm album.getInfo + resolve.
func (h *RecommendHandler) GetAlbumTracks(c fiber.Ctx) error {
	if h.apiKey == "" {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "LASTFM_API_KEY not set"})
	}
	artist := strings.TrimSpace(c.Query("artist"))
	album := strings.TrimSpace(c.Query("album"))
	if artist == "" || album == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "artist and album required"})
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Context()), albumTracksBudget)
	defer cancel()

	pairs, err := h.lastfmAlbumTracks(ctx, artist, album)
	if err != nil {
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "Couldn't load album tracks"})
	}
	// Last.fm often omits tracklists for singles — fall back to provider search.
	if len(pairs) == 0 {
		songs := h.albumSearchFallback(ctx, artist, album)
		if len(songs) == 0 {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "No tracks for this album"})
		}
		return c.JSON(songs)
	}
	// Short queue is enough to start playback; client CoverResolver fills art.
	songs := h.resolveN(ctx, pairs, lastfmPair{}, albumResolveCap, false, true)
	if len(songs) == 0 {
		songs = h.albumSearchFallback(ctx, artist, album)
	}
	if len(songs) == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "Couldn't resolve album tracks"})
	}
	return c.JSON(songs)
}

// GET /resolve?artist=&title=&refresh=1 → one playable Song whose artist+title match the request.
// `refresh=1` bypasses the short resolve cache (dead CDN recovery).
// `strict` is accepted for client compat; resolve always requires a real title+artist match
// (never a first-hit / artist-only fallback — that stamped wrong audio onto vault rows).
func (h *RecommendHandler) GetResolve(c fiber.Ctx) error {
	artist := strings.TrimSpace(c.Query("artist"))
	title := strings.TrimSpace(c.Query("title"))
	if artist == "" || title == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "artist and title required"})
	}
	refresh := c.Query("refresh") == "1" || strings.EqualFold(c.Query("refresh"), "true")
	song, ok := h.resolveOne(c.Context(), lastfmPair{artist: artist, title: title}, lastfmPair{}, refresh)
	if !ok {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "no match"})
	}
	// Spotify import + play-from-resolve — fill art before the client saves to vault.
	songs := []domain.Song{song}
	h.covers.FillSongs(c.Context(), songs)
	return c.JSON(songs[0])
}

// GET /recommend?artist=&title=&mode=radio&offset=N
// Cached 6h per normalized seed (+ mode/offset). Explore "because" stays diverse;
// mode=radio keeps same-artist + similar so the station doesn't pivot genres.
func (h *RecommendHandler) GetRecommend(c fiber.Ctx) error {
	artist := strings.TrimSpace(c.Query("artist"))
	title := strings.TrimSpace(c.Query("title"))
	if artist == "" || title == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "artist and title are required"})
	}
	if h.apiKey == "" {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "LASTFM_API_KEY not set"})
	}

	radio := strings.EqualFold(c.Query("mode"), "radio")
	offset, _ := strconv.Atoi(c.Query("offset"))
	if offset < 0 {
		offset = 0
	}
	capN := recommendResolveCap
	if radio {
		capN = radioResolveCap
	}

	refresh := c.Query("refresh") == "1" || strings.EqualFold(c.Query("refresh"), "true")
	key := songKey(artist, title)
	if radio {
		key = key + "|radio|" + strconv.Itoa(offset)
	}
	if !refresh {
		if songs, ok := h.recommendSnap(key); ok {
			c.Set("Cache-Control", "private, max-age=21600")
			return c.JSON(songs)
		}
	}

	// Refresh uses a distinct SF key so it never joins a non-refresh flight serving cache.
	sfKey := key
	if refresh {
		sfKey = key + "|refresh"
	}
	v, err, _ := h.recommendSF.Do(sfKey, func() (any, error) {
		if !refresh {
			if songs, ok := h.recommendSnap(key); ok {
				return songs, nil
			}
		}
		// Don't cancel shared work if the first client disconnects mid-build.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Context()), 30*time.Second)
		defer cancel()

		seed := lastfmPair{artist: artist, title: title}
		artists := artistCandidates(artist)
		pairs, err := h.pairsFor(ctx, seed, artists, radio, refresh)
		if err != nil {
			return nil, err
		}
		if radio && offset > 0 && len(pairs) > 0 {
			off := offset % len(pairs)
			pairs = append(pairs[off:], pairs[:off]...)
		}

		songs := h.resolveN(ctx, pairs, seed, capN, refresh, false)
		// ponytail: Last.fm often misses collab credits — fall back to our search by artist name.
		if len(songs) == 0 {
			songs = h.searchFallback(ctx, artists, seed)
			if radio && len(songs) > capN {
				songs = songs[:capN]
			}
		}
		if len(songs) == 0 {
			return nil, errRecommendEmpty
		}
		h.covers.FillSongs(ctx, songs)
		h.recommendStore(key, songs)
		return songs, nil
	})
	if err != nil {
		if stale, ok := h.recommendStale(key); ok {
			c.Set("Cache-Control", "private, max-age=60")
			return c.JSON(stale)
		}
		if err == errRecommendEmpty {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "Couldn't build a radio for this track"})
		}
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "Radio lookup failed. Try again."})
	}
	songs, _ := v.([]domain.Song)
	c.Set("Cache-Control", "private, max-age=21600")
	return c.JSON(songs)
}

var errRecommendEmpty = fmt.Errorf("recommend empty")

// pairsFor returns Last.fm neighborhood for a seed; cached across radio offsets.
// refresh skips the pairs cache so Explore pull-to-refresh can recompute.
func (h *RecommendHandler) pairsFor(ctx context.Context, seed lastfmPair, artists []string, radio, refresh bool) ([]lastfmPair, error) {
	pk := songKey(seed.artist, seed.title)
	if pk == "" {
		return h.collectPairs(ctx, seed, artists, radio)
	}
	if radio {
		pk += "|radio"
	} else {
		pk += "|rec"
	}
	if refresh {
		pairs, err := h.collectPairs(ctx, seed, artists, radio)
		if err != nil {
			return nil, err
		}
		h.pairsStore(pk, pairs)
		return pairs, nil
	}
	v, err, _ := h.pairsSF.Do(pk, func() (any, error) {
		if pairs, ok := h.pairsSnap(pk); ok {
			return pairs, nil
		}
		pairs, err := h.collectPairs(ctx, seed, artists, radio)
		if err != nil {
			return nil, err
		}
		h.pairsStore(pk, pairs)
		return pairs, nil
	})
	if err != nil {
		return nil, err
	}
	src, _ := v.([]lastfmPair)
	out := make([]lastfmPair, len(src))
	copy(out, src)
	return out, nil
}

func (h *RecommendHandler) pairsSnap(key string) ([]lastfmPair, bool) {
	if key == "" {
		return nil, false
	}
	h.pairsMu.Lock()
	defer h.pairsMu.Unlock()
	e, ok := h.pairsCache[key]
	if !ok || len(e.pairs) == 0 || time.Since(e.at) > pairsTTL {
		return nil, false
	}
	out := make([]lastfmPair, len(e.pairs))
	copy(out, e.pairs)
	return out, true
}

func (h *RecommendHandler) pairsStore(key string, pairs []lastfmPair) {
	if key == "" || len(pairs) == 0 {
		return
	}
	h.pairsMu.Lock()
	defer h.pairsMu.Unlock()
	if h.pairsCache == nil {
		h.pairsCache = make(map[string]pairsEntry)
	}
	if len(h.pairsCache) >= pairsCacheCap {
		for k, e := range h.pairsCache {
			if time.Since(e.at) > pairsTTL {
				delete(h.pairsCache, k)
			}
		}
		if len(h.pairsCache) >= pairsCacheCap {
			h.pairsCache = make(map[string]pairsEntry, pairsCacheCap/2)
		}
	}
	cp := make([]lastfmPair, len(pairs))
	copy(cp, pairs)
	h.pairsCache[key] = pairsEntry{pairs: cp, at: time.Now()}
}

type lastfmPair struct{ artist, title string }

func (h *RecommendHandler) collectPairs(ctx context.Context, seed lastfmPair, artists []string, radio bool) ([]lastfmPair, error) {
	pairCap := recommendResolveCap * 2
	if radio {
		pairCap = radioResolveCap * 3 // more candidates so offset rotates through a neighborhood
	}
	var pairs []lastfmPair

	for _, a := range artists {
		similar, err := h.lastfmSimilar(ctx, a, seed.title)
		if err != nil {
			return nil, err
		}
		pairs = uniquePairs(append(pairs, similar...), seed)
		if len(pairs) >= pairCap {
			break
		}
	}

	for _, a := range artists {
		if len(pairs) >= pairCap {
			break
		}
		extra, err := h.pairsFromSimilarArtists(ctx, a, seed.title)
		if err != nil {
			return nil, err
		}
		pairs = uniquePairs(append(pairs, extra...), seed)
	}

	for _, a := range artists {
		if len(pairs) >= pairCap {
			break
		}
		tops, err := h.lastfmArtistTop(ctx, a, seed.title)
		if err != nil {
			return nil, err
		}
		pairs = uniquePairs(append(pairs, tops...), seed)
	}

	// Explore "because": prefer other artists. Radio: keep same-artist tops in the mix.
	if !radio {
		others := filterNotSeedArtists(pairs, artists)
		if len(others) >= 3 {
			return others, nil
		}
	}
	return pairs, nil
}

// artistCandidates: "Satoshi, magnat, feoctist" → Satoshi first (Last.fm / search), then others.
func artistCandidates(artist string) []string {
	full := strings.TrimSpace(artist)
	if full == "" {
		return nil
	}
	parts := splitArtistParts(full)
	out := make([]string, 0, len(parts)+1)
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		k := utils.NormalizeString(s)
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, s)
	}
	for _, p := range parts {
		add(p)
	}
	add(full)
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func splitArtistParts(artist string) []string {
	s := artist
	for _, sep := range []string{" feat. ", " feat ", " ft. ", " ft ", " featuring ", " x ", " X ", " / ", ";", "&"} {
		s = strings.ReplaceAll(s, sep, ",")
	}
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func filterNotSeedArtists(pairs []lastfmPair, seedArtists []string) []lastfmPair {
	block := map[string]bool{}
	for _, a := range seedArtists {
		block[utils.NormalizeString(a)] = true
	}
	out := make([]lastfmPair, 0, len(pairs))
	for _, p := range pairs {
		if block[utils.NormalizeString(p.artist)] {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (h *RecommendHandler) searchFallback(ctx context.Context, artists []string, seed lastfmPair) []domain.Song {
	seedKey := songKey(seed.artist, seed.title)
	seen := map[string]bool{seedKey: true}
	out := make([]domain.Song, 0, recommendResolveCap)

	for _, a := range artists {
		if len(out) >= recommendResolveCap {
			break
		}
		resp, err := h.search.Search(ctx, a, 1)
		if err != nil || resp == nil {
			continue
		}
		for _, s := range resp.Songs {
			k := songKey(s.Artist, s.Title)
			if k == "" || seen[k] || s.Link == "" {
				continue
			}
			seen[k] = true
			out = append(out, s)
			if len(out) >= recommendResolveCap {
				break
			}
		}
	}
	return out
}

// albumSearchFallback plays singles / albums Last.fm lists without a tracklist.
func (h *RecommendHandler) albumSearchFallback(ctx context.Context, artist, album string) []domain.Song {
	if h.search == nil {
		return nil
	}
	q := strings.TrimSpace(artist + " " + album)
	if q == "" {
		return nil
	}
	songs, err := h.search.SearchFirst(ctx, q, albumResolveCap)
	if err != nil || len(songs) == 0 {
		return nil
	}
	out := make([]domain.Song, 0, albumResolveCap)
	seen := map[string]bool{}
	wantArt := utils.NormalizeString(artist)
	for _, s := range songs {
		if s.Link == "" {
			continue
		}
		k := songKey(s.Artist, s.Title)
		if k == "" || seen[k] {
			continue
		}
		// Singles: album title is often the track title.
		if album != "" && services.IsPlayableMatch(artist, album, s) {
			seen[k] = true
			out = append(out, s)
		} else if wantArt != "" {
			gotArt := utils.NormalizeString(s.Artist)
			if gotArt == wantArt || strings.Contains(gotArt, wantArt) || strings.Contains(wantArt, gotArt) {
				seen[k] = true
				out = append(out, s)
			}
		} else {
			seen[k] = true
			out = append(out, s)
		}
		if len(out) >= albumResolveCap {
			break
		}
	}
	return out
}

func uniquePairs(pairs []lastfmPair, seed lastfmPair) []lastfmPair {
	seen := map[string]bool{songKey(seed.artist, seed.title): true}
	out := make([]lastfmPair, 0, len(pairs))
	for _, p := range pairs {
		k := songKey(p.artist, p.title)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	return out
}

// songKey collapses remix/feat variants: "Collide (Extended Mix)" ≈ "Collide".
func songKey(artist, title string) string {
	return services.SongKey(artist, title)
}

func coreTitle(title string) string {
	return services.CoreTitle(title)
}
