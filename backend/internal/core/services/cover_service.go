package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/utils"
	"golang.org/x/sync/singleflight"
)

const (
	coverCacheTTL = 24 * time.Hour
	// ponytail: empty misses must not lock out retries for a full day
	coverMissTTL  = 15 * time.Minute
	coverFillConc = 8
	coverCacheMax = 10_000
	coverFetchCap = 8 * time.Second
	// Last.fm grey star placeholder — treat as missing.
	lastfmStubMarker = "2a96cbd8b46e442fc41c2b86b821562f"
)

type coverEntry struct {
	url string
	exp time.Time
}

// CoverService resolves missing artwork: Last.fm (same stack as search) raced with Apple.
// Search hits already carry Last.fm art — Fill* only runs for empties/stubs.
type CoverService struct {
	client    *http.Client
	lastfmKey string
	mu        sync.Mutex
	cache     map[string]coverEntry
	sf        singleflight.Group
}

func NewCoverService(client *http.Client, lastfmKey string) *CoverService {
	return &CoverService{
		client:    client,
		lastfmKey: strings.TrimSpace(lastfmKey),
		cache:     make(map[string]coverEntry),
	}
}

// HasRealCover is true for a usable http(s) image (not empty, not Last.fm stub).
func HasRealCover(image string) bool {
	u := strings.TrimSpace(image)
	if u == "" || strings.Contains(u, lastfmStubMarker) {
		return false
	}
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

func hasRealCover(image string) bool { return HasRealCover(image) }

// Lookup returns artwork for a free-text query: cache → race Last.fm + Apple → first hit.
func (cs *CoverService) Lookup(ctx context.Context, q string) string {
	if cs == nil {
		return ""
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	key := utils.NormalizeString(q)
	if img, ok := cs.get(key); ok {
		return img
	}

	v, _, _ := cs.sf.Do(key, func() (any, error) {
		if img, ok := cs.get(key); ok {
			return img, nil
		}
		fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), coverFetchCap)
		defer cancel()
		img := cs.fetchFirst(fctx, q)
		cs.put(key, img)
		return img, nil
	})
	img, _ := v.(string)
	return img
}

// fetchFirst races Last.fm track.search and iTunes; returns the first real URL.
func (cs *CoverService) fetchFirst(ctx context.Context, q string) string {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan string, 2)
	go func() { ch <- fetchLastfmCover(ctx, cs.client, cs.lastfmKey, q) }()
	go func() { ch <- fetchItunesCover(ctx, cs.client, q) }()

	var miss int
	for miss < 2 {
		select {
		case <-ctx.Done():
			return ""
		case img := <-ch:
			if hasRealCover(img) {
				cancel()
				return utils.UpgradeHTTPS(img)
			}
			miss++
		}
	}
	return ""
}

// FillSongs sets Image on songs that lack real art. Honors ctx deadline.
func (cs *CoverService) FillSongs(ctx context.Context, songs []domain.Song) {
	if cs == nil || len(songs) == 0 {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, coverFillConc)
	for i := range songs {
		if hasRealCover(songs[i].Image) {
			continue
		}
		songs[i].Image = ""
		q := strings.TrimSpace(songs[i].Artist + " " + songs[i].Title)
		if q == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return
		}
		wg.Add(1)
		go func(i int, q string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if img := cs.Lookup(ctx, q); img != "" {
				songs[i].Image = img
			}
		}(i, q)
	}
	wg.Wait()
}

// FillAlbums sets Image on albums that lack real Last.fm art.
func (cs *CoverService) FillAlbums(ctx context.Context, albums []domain.ArtistAlbum) {
	if cs == nil || len(albums) == 0 {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, coverFillConc)
	for i := range albums {
		if hasRealCover(albums[i].Image) {
			continue
		}
		albums[i].Image = ""
		q := strings.TrimSpace(albums[i].Artist + " " + albums[i].Name)
		if q == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return
		}
		wg.Add(1)
		go func(i int, q string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if img := cs.Lookup(ctx, q); img != "" {
				albums[i].Image = img
			}
		}(i, q)
	}
	wg.Wait()
}

// FillArtists sets Image on search artists that lack real Last.fm art.
func (cs *CoverService) FillArtists(ctx context.Context, artists []domain.SearchArtist) {
	if cs == nil || len(artists) == 0 {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, coverFillConc)
	for i := range artists {
		if hasRealCover(artists[i].Image) {
			continue
		}
		artists[i].Image = ""
		q := strings.TrimSpace(artists[i].Name)
		if q == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return
		}
		wg.Add(1)
		go func(i int, q string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if img := cs.Lookup(ctx, q); img != "" {
				artists[i].Image = img
			}
		}(i, q)
	}
	wg.Wait()
}

func (cs *CoverService) get(key string) (string, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	e, ok := cs.cache[key]
	if !ok {
		return "", false
	}
	if time.Now().After(e.exp) {
		delete(cs.cache, key)
		return "", false
	}
	return e.url, true
}

func (cs *CoverService) put(key, img string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if len(cs.cache) >= coverCacheMax {
		cs.cache = make(map[string]coverEntry)
	}
	ttl := coverCacheTTL
	if img == "" {
		ttl = coverMissTTL
	}
	cs.cache[key] = coverEntry{url: img, exp: time.Now().Add(ttl)}
}

func fetchLastfmCover(ctx context.Context, client *http.Client, apiKey, q string) string {
	if client == nil || strings.TrimSpace(apiKey) == "" {
		return ""
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	vals := url.Values{
		"method":  {"track.search"},
		"track":   {q},
		"api_key": {apiKey},
		"format":  {"json"},
		"limit":   {"1"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ws.audioscrobbler.com/2.0/?"+vals.Encode(), nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var payload struct {
		Results struct {
			TrackMatches struct {
				Track json.RawMessage `json:"track"`
			} `json:"trackmatches"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	rows, err := decodeSearchTracks(payload.Results.TrackMatches.Track)
	if err != nil || len(rows) == 0 {
		return ""
	}
	return bestSearchImage(rows[0].Image)
}

func fetchItunesCover(ctx context.Context, client *http.Client, q string) string {
	if client == nil {
		return ""
	}
	apiURL := "https://itunes.apple.com/search?media=music&entity=song&limit=1&term=" + url.QueryEscape(q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var data struct {
		Results []struct {
			ArtworkURL100 string `json:"artworkUrl100"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}
	if len(data.Results) == 0 || data.Results[0].ArtworkURL100 == "" {
		return ""
	}
	return strings.Replace(data.Results[0].ArtworkURL100, "100x100", "600x600", 1)
}
