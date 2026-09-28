package services

import (
	"context"
	"errors"
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
	searchMapPeek  = 8
)

type searchCacheEntry struct {
	resp *domain.SearchResponse
	at   time.Time
}

type SearchService struct {
	providers     []ports.IMusicProvider
	config        *domain.SearchConfig
	searchTimeout time.Duration

	cacheMu sync.Mutex
	cache   map[string]searchCacheEntry
	sf      singleflight.Group
}

func NewSearchService(
	providers []ports.IMusicProvider,
	config *domain.SearchConfig,
	timeout time.Duration,
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
		cache:         make(map[string]searchCacheEntry),
	}
}

// Search queries the music providers directly.
func (ss *SearchService) Search(ctx context.Context, query string, page int) (*domain.SearchResponse, error) {
	return ss.SearchWithProgress(ctx, query, page, nil, nil)
}

// SearchWithProgress emits each provider batch as soon as it arrives.
func (ss *SearchService) SearchWithProgress(
	ctx context.Context,
	query string,
	page int,
	onMeta func(domain.SearchProgress) error,
	onSong func(domain.Song) error,
) (*domain.SearchResponse, error) {
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
	ctx context.Context, text string, page int,
	onMeta func(domain.SearchProgress) error, onSong func(domain.Song) error,
) (*domain.SearchResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, ss.searchTimeout)
	defer cancel()
	type batch struct {
		rows []domain.ProviderResult
		err  error
	}
	ch := make(chan batch, len(ss.providers))
	for _, provider := range ss.providers {
		go func(p ports.IMusicProvider) {
			rows, err := p.SearchWithPage(ctx, text, page)
			ch <- batch{rows, err}
		}(provider)
	}
	pagination := &domain.PaginationInfo{CurrentPage: page, HasPrevPage: page > 1}
	resp := domain.NewSearchResponse([]domain.Song{}, pagination)
	seen := map[string]bool{}
	successes := 0
	for range ss.providers {
		var result batch
		select {
		case result = <-ch:
		case <-ctx.Done():
			if len(resp.Songs) > 0 {
				return resp, nil
			}
			return resp, ctx.Err()
		}
		if result.err != nil {
			continue
		}
		successes++
		for _, row := range result.rows {
			if p := row.Pagination; p != nil {
				pagination.HasNextPage = pagination.HasNextPage || p.HasNextPage
				pagination.TotalPages = max(pagination.TotalPages, p.TotalPages)
				pagination.TotalResults = max(pagination.TotalResults, p.TotalResults)
			}
		}
		limit := ss.config.MaxResults
		if limit <= 0 {
			limit = constants.DefaultMaxSearchResults
		}
		for _, song := range playableSongs(result.rows, limit) {
			// Keep alternate providers/versions available, but never duplicate a URL.
			if seen[song.Link] {
				continue
			}
			seen[song.Link] = true
			resp.Songs = append(resp.Songs, song)
			if onSong != nil {
				if err := onSong(song); err != nil {
					return resp, err
				}
			}
		}
		if onMeta != nil {
			if err := onMeta(domain.SearchProgress{Pagination: pagination}); err != nil {
				return resp, err
			}
		}
	}
	if successes == 0 {
		return resp, domain.ErrUnavailable
	}
	return resp, nil
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
