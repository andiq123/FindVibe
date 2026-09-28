package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andiq123/FindVibeFiber/internal/core/constants"
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/core/ports"
	"github.com/andiq123/FindVibeFiber/internal/utils"
	"golang.org/x/sync/singleflight"
)

const (
	searchCacheTTL = 2 * time.Minute
	searchCacheCap = 256
	// Oversample Last.fm so provider mapping attrition still fills MaxResults.
	searchCatalogOversample = 2
	searchMapPeek           = 8
)

type searchCacheEntry struct {
	resp *domain.SearchResponse
	at   time.Time
}

// catalogSearcher is Last.fm discovery (tests inject a stub).
type catalogSearcher interface {
	Configured() bool
	Search(ctx context.Context, query string, page, limit int) (CatalogPage, error)
	TopAlbums(ctx context.Context, artist string, limit, page int) ([]domain.ArtistAlbum, *domain.PaginationInfo, error)
	AlbumSearch(ctx context.Context, query string, limit int) ([]domain.ArtistAlbum, error)
}

type SearchService struct {
	providers     []ports.IMusicProvider
	config        *domain.SearchConfig
	searchTimeout time.Duration
	catalog       catalogSearcher
	covers        *CoverService

	cacheMu sync.Mutex
	cache   map[string]searchCacheEntry
	sf      singleflight.Group
}

func NewSearchService(
	providers []ports.IMusicProvider,
	config *domain.SearchConfig,
	timeout time.Duration,
	catalog catalogSearcher,
) *SearchService {
	if config == nil {
		config = domain.DefaultSearchConfig()
	}
	if timeout <= 0 {
		timeout = time.Duration(constants.DefaultSearchTimeout) * time.Second
	}

	return &SearchService{
		providers:     providers,
		config:        config,
		searchTimeout: timeout,
		catalog:       catalog,
		cache:         make(map[string]searchCacheEntry),
	}
}

// SetCovers wires artwork lookup used while mapping (parallel per hit; does not stall the stream).
func (ss *SearchService) SetCovers(covers *CoverService) {
	if ss == nil {
		return
	}
	ss.covers = covers
}

// Search: Last.fm discovers songs/artists → providers map playable URLs → only successful maps.
func (ss *SearchService) Search(ctx context.Context, query string, page int) (*domain.SearchResponse, error) {
	return ss.SearchWithProgress(ctx, query, page, nil, nil)
}

// SearchWithProgress streams discovery meta, then each mapped song as soon as it succeeds.
func (ss *SearchService) SearchWithProgress(
	ctx context.Context,
	query string,
	page int,
	onMeta func(domain.SearchProgress) error,
	onSong func(domain.Song) error,
) (*domain.SearchResponse, error) {
	if ss.catalog == nil || !ss.catalog.Configured() {
		return nil, fmt.Errorf("search: %w", domain.ErrUnavailable)
	}
	if len(ss.providers) == 0 {
		return domain.NewSearchResponse([]domain.Song{}, nil), nil
	}

	text := strings.TrimSpace(query)
	if text == "" {
		return domain.NewSearchResponse([]domain.Song{}, nil), nil
	}
	if page < 1 {
		page = 1
	}

	key := searchCacheKey(text, page)
	if hit := ss.cacheGet(key); hit != nil {
		return ss.emitCached(hit, onMeta, onSong)
	}

	// Live progress must not wait on singleflight followers — only cache the leader result.
	if onMeta != nil || onSong != nil {
		resp, err := ss.searchUncached(ctx, text, page, onMeta, onSong)
		if err != nil {
			return nil, err
		}
		if resp != nil && len(resp.Songs) > 0 {
			ss.cachePut(key, resp)
		}
		if resp == nil {
			return domain.NewSearchResponse([]domain.Song{}, nil), nil
		}
		return resp, nil
	}

	v, err, _ := ss.sf.Do(key, func() (any, error) {
		if hit := ss.cacheGet(key); hit != nil {
			return hit, nil
		}
		resp, err := ss.searchUncached(ctx, text, page, nil, nil)
		if err != nil {
			return nil, err
		}
		if resp != nil && len(resp.Songs) > 0 {
			ss.cachePut(key, resp)
		}
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	resp, _ := v.(*domain.SearchResponse)
	if resp == nil {
		return domain.NewSearchResponse([]domain.Song{}, nil), nil
	}
	return cloneSearchResponse(resp), nil
}

func (ss *SearchService) emitCached(
	hit *domain.SearchResponse,
	onMeta func(domain.SearchProgress) error,
	onSong func(domain.Song) error,
) (*domain.SearchResponse, error) {
	resp := cloneSearchResponse(hit)
	if onMeta != nil {
		if err := onMeta(domain.SearchProgress{
			Artists:    append([]domain.SearchArtist(nil), resp.Artists...),
			Albums:     append([]domain.ArtistAlbum(nil), resp.Albums...),
			Pagination: resp.Pagination,
		}); err != nil {
			return resp, err
		}
	}
	if onSong != nil {
		for i := range resp.Songs {
			if err := onSong(resp.Songs[i]); err != nil {
				return resp, err
			}
		}
	}
	return resp, nil
}

func (ss *SearchService) searchUncached(
	ctx context.Context,
	text string,
	page int,
	onMeta func(domain.SearchProgress) error,
	onSong func(domain.Song) error,
) (*domain.SearchResponse, error) {
	maxResults := ss.config.MaxResults
	if maxResults <= 0 {
		maxResults = constants.DefaultMaxSearchResults
	}
	limit := maxResults * searchCatalogOversample
	if limit < maxResults {
		limit = maxResults
	}

	pageData, err := ss.catalog.Search(ctx, text, page, limit)
	if err != nil {
		return nil, err
	}

	resp := domain.NewSearchResponse(nil, pageData.Pagination)
	if page == 1 && len(pageData.Artists) > 0 {
		resp.Artists = make([]domain.SearchArtist, 0, len(pageData.Artists))
		for _, a := range pageData.Artists {
			resp.Artists = append(resp.Artists, domain.SearchArtist{Name: a.Name, Image: a.Image})
		}
	}

	streaming := onMeta != nil || onSong != nil
	if streaming {
		// First paint immediately — do not wait on album Last.fm or covers.
		if onMeta != nil {
			if err := onMeta(domain.SearchProgress{
				Artists:    append([]domain.SearchArtist(nil), resp.Artists...),
				Pagination: resp.Pagination,
			}); err != nil {
				return resp, err
			}
		}

		var albums []domain.ArtistAlbum
		var albumWG sync.WaitGroup
		if page == 1 {
			albumWG.Add(1)
			go func() {
				defer albumWG.Done()
				albums = ss.gatherAlbums(ctx, text, pageData.Artists)
			}()
		}

		resp.Songs = ss.mapCatalogHits(ctx, pageData.Hits, maxResults, onSong)

		albumWG.Wait()
		resp.Albums = albums
		if onMeta != nil && len(albums) > 0 {
			_ = onMeta(domain.SearchProgress{Albums: append([]domain.ArtistAlbum(nil), albums...)})
		}
		return resp, nil
	}

	if page == 1 {
		resp.Albums = ss.gatherAlbums(ctx, text, pageData.Artists)
	}
	resp.Songs = ss.mapCatalogHits(ctx, pageData.Hits, maxResults, nil)
	return resp, nil
}

// gatherAlbums merges album.search (query) with top artist's top albums; prefers rows with art.
func (ss *SearchService) gatherAlbums(ctx context.Context, query string, artists []CatalogArtist) []domain.ArtistAlbum {
	var searched, tops []domain.ArtistAlbum
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		searched, _ = ss.catalog.AlbumSearch(ctx, query, catalogAlbumsForTopArtist)
	}()
	if len(artists) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tops, _, _ = ss.catalog.TopAlbums(ctx, artists[0].Name, catalogAlbumsForTopArtist, 1)
		}()
	}
	wg.Wait()
	return mergeDiscoveryAlbums(searched, tops, catalogAlbumsForTopArtist)
}

func mergeDiscoveryAlbums(searched, tops []domain.ArtistAlbum, limit int) []domain.ArtistAlbum {
	if limit < 1 {
		limit = catalogAlbumsForTopArtist
	}
	seen := map[string]int{} // key → index in out
	out := make([]domain.ArtistAlbum, 0, limit)

	add := func(a domain.ArtistAlbum) {
		name := strings.TrimSpace(a.Name)
		if name == "" || strings.EqualFold(name, "(null)") || strings.EqualFold(name, "null") {
			return
		}
		artist := strings.TrimSpace(a.Artist)
		key := utils.NormalizeString(artist) + "|" + utils.NormalizeString(name)
		if key == "|" {
			return
		}
		if i, ok := seen[key]; ok {
			// Upgrade stub/empty art when a later source has a real image.
			if strings.TrimSpace(out[i].Image) == "" && strings.TrimSpace(a.Image) != "" {
				out[i].Image = a.Image
			}
			if a.Playcount > out[i].Playcount {
				out[i].Playcount = a.Playcount
			}
			return
		}
		if len(out) >= limit {
			return
		}
		seen[key] = len(out)
		out = append(out, a)
	}

	for _, a := range searched {
		add(a)
	}
	for _, a := range tops {
		add(a)
	}

	// Stable: albums with cover art first so the rail looks filled.
	sort.SliceStable(out, func(i, j int) bool {
		hi := strings.TrimSpace(out[i].Image) != ""
		hj := strings.TrimSpace(out[j].Image) != ""
		if hi != hj {
			return hi
		}
		return false
	})
	return out
}

// mapCatalogHits resolves Last.fm rows through providers; emits each success immediately.
func (ss *SearchService) mapCatalogHits(
	ctx context.Context,
	hits []CatalogHit,
	capN int,
	onSong func(domain.Song) error,
) []domain.Song {
	if capN < 1 || len(hits) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type slot struct {
		song domain.Song
		ok   bool
	}
	ch := make(chan slot, len(hits))
	var wg sync.WaitGroup
	sem := make(chan struct{}, constants.DefaultResolveConcurrency)

	for _, hit := range hits {
		wg.Add(1)
		go func(hit CatalogHit) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				ch <- slot{}
				return
			}
			song, ok := ss.mapOne(ctx, hit)
			ch <- slot{song: song, ok: ok}
		}(hit)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	seen := map[string]struct{}{}
	out := make([]domain.Song, 0, capN)
	for s := range ch {
		if !s.ok {
			continue
		}
		k := SongKey(s.song.Artist, s.song.Title)
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, s.song)
		if onSong != nil {
			if err := onSong(s.song); err != nil {
				cancel()
				return out
			}
		}
		if len(out) >= capN {
			cancel()
			for range ch {
			}
			return out
		}
	}
	return out
}

func (ss *SearchService) mapOne(ctx context.Context, hit CatalogHit) (domain.Song, bool) {
	songs, err := ss.SearchFirst(ctx, hit.Artist+" "+hit.Title, searchMapPeek)
	if err != nil || len(songs) == 0 {
		return domain.Song{}, false
	}
	song, ok := PickPlayableSong(hit.Artist, hit.Title, songs, "", searchMapPeek)
	if !ok {
		return domain.Song{}, false
	}
	// Prefer catalog art; never wait on Lookup — stream emits now, client fills via /cover.
	if img := hit.Image; hasRealCover(img) {
		song.Image = img
	} else if !hasRealCover(song.Image) {
		song.Image = ""
	}
	return song, true
}

// SearchFirst fans out by priority and returns as soon as the best available
// playable hit cannot be beaten by any still-in-flight higher-priority provider.
func (ss *SearchService) SearchFirst(ctx context.Context, query string, limit int) ([]domain.Song, error) {
	if len(ss.providers) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = searchMapPeek
	}

	text := strings.TrimSpace(query)
	if text == "" {
		return nil, nil
	}

	providers := append([]ports.IMusicProvider(nil), ss.providers...)
	sort.SliceStable(providers, func(i, j int) bool {
		return providers[i].Priority() > providers[j].Priority()
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type outcome struct {
		idx   int
		songs []domain.Song
	}
	ch := make(chan outcome, len(providers))
	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Add(1)
		go func(i int, p ports.IMusicProvider) {
			defer wg.Done()
			pctx, pcancel := context.WithTimeout(ctx, ss.searchTimeout)
			defer pcancel()
			got, err := p.SearchWithPage(pctx, text, 1)
			if err != nil {
				if !isBenignSearchErr(err) {
					utils.GetLogger().Warn("provider search-first failed", "provider", p.Name(), "query", text, "error", err)
				}
				ch <- outcome{idx: i}
				return
			}
			ch <- outcome{idx: i, songs: playableSongs(got, limit)}
		}(i, p)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	unfinished := make([]bool, len(providers))
	for i := range unfinished {
		unfinished[i] = true
	}

	var best []domain.Song
	bestPri := -1
	for o := range ch {
		unfinished[o.idx] = false
		if len(o.songs) > 0 {
			pri := providers[o.idx].Priority()
			if pri > bestPri {
				bestPri = pri
				best = o.songs
			}
		}
		if len(best) == 0 {
			continue
		}
		higherPending := false
		for j, p := range providers {
			if unfinished[j] && p.Priority() > bestPri {
				higherPending = true
				break
			}
		}
		if !higherPending {
			cancel()
			for range ch {
			}
			return best, nil
		}
	}
	return best, nil
}

func playableSongs(results []domain.ProviderResult, limit int) []domain.Song {
	songs := make([]domain.Song, 0, limit)
	for i := range results {
		if len(songs) >= limit {
			break
		}
		s := results[i].Song
		s.Link = utils.UpgradeHTTPS(s.Link)
		s.Image = utils.UpgradeHTTPS(s.Image)
		if !strings.HasPrefix(s.Link, "https://") {
			continue
		}
		songs = append(songs, s)
	}
	return songs
}

func searchCacheKey(text string, page int) string {
	return utils.NormalizeString(text) + "|" + strconv.Itoa(page)
}

func (ss *SearchService) cacheGet(key string) *domain.SearchResponse {
	ss.cacheMu.Lock()
	defer ss.cacheMu.Unlock()
	e, ok := ss.cache[key]
	if !ok || time.Since(e.at) > searchCacheTTL {
		return nil
	}
	return e.resp
}

func (ss *SearchService) cachePut(key string, resp *domain.SearchResponse) {
	if resp == nil || len(resp.Songs) == 0 {
		return
	}
	ss.cacheMu.Lock()
	defer ss.cacheMu.Unlock()
	if len(ss.cache) >= searchCacheCap {
		ss.cache = make(map[string]searchCacheEntry, searchCacheCap/2)
	}
	ss.cache[key] = searchCacheEntry{resp: cloneSearchResponse(resp), at: time.Now()}
}

func cloneSearchResponse(r *domain.SearchResponse) *domain.SearchResponse {
	if r == nil {
		return nil
	}
	songs := append([]domain.Song(nil), r.Songs...)
	artists := append([]domain.SearchArtist(nil), r.Artists...)
	albums := append([]domain.ArtistAlbum(nil), r.Albums...)
	var pag *domain.PaginationInfo
	if r.Pagination != nil {
		p := *r.Pagination
		pag = &p
	}
	return &domain.SearchResponse{
		Songs:      songs,
		Artists:    artists,
		Albums:     albums,
		Pagination: pag,
	}
}

func isBenignSearchErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
