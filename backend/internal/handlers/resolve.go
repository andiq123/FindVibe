package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/andiq123/FindVibeFiber/internal/core/constants"
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/core/services"
	"github.com/andiq123/FindVibeFiber/internal/utils"
)

func (h *RecommendHandler) resolve(ctx context.Context, pairs []lastfmPair, seed lastfmPair) []domain.Song {
	return h.resolveN(ctx, pairs, seed, recommendResolveCap, false, false)
}

func (h *RecommendHandler) resolveN(
	ctx context.Context,
	pairs []lastfmPair,
	seed lastfmPair,
	cap int,
	bypassCache bool,
	sameArtistOK bool,
) []domain.Song {
	if cap < 1 {
		cap = recommendResolveCap
	}
	if len(pairs) > cap*2 {
		pairs = pairs[:cap*2]
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type slot struct {
		i    int
		song domain.Song
		ok   bool
	}
	ch := make(chan slot, len(pairs))
	var wg sync.WaitGroup
	// Bound fan-out — unbounded parallel /search was flooding Mp3pm/Mp3mn.
	sem := make(chan struct{}, constants.DefaultResolveConcurrency)
	for i, p := range pairs {
		wg.Add(1)
		go func(i int, p lastfmPair) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				ch <- slot{i: i}
				return
			}
			song, ok := h.resolveOne(ctx, p, seed, bypassCache)
			ch <- slot{i: i, song: song, ok: ok}
		}(i, p)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	byIdx := make(map[int]domain.Song, len(pairs))
	decided := make([]bool, len(pairs))
	for s := range ch {
		decided[s.i] = true
		if s.ok {
			byIdx[s.i] = s.song
		}
		// ponytail: only cut once a prefix is fully decided — keeps Last.fm order.
		prefix := 0
		for prefix < len(pairs) && decided[prefix] {
			prefix++
		}
		if out := pickResolved(byIdx, prefix, seed, cap, sameArtistOK); len(out) >= cap {
			cancel()
			return out
		}
		// Album: return what we have before the gateway kills the request.
		if sameArtistOK {
			if out := pickResolved(byIdx, prefix, seed, cap, true); len(out) > 0 && albumBudgetNearlyGone(ctx) {
				cancel()
				return out
			}
		}
	}
	return pickResolved(byIdx, len(pairs), seed, cap, sameArtistOK)
}

func albumBudgetNearlyGone(ctx context.Context) bool {
	deadline, ok := ctx.Deadline()
	if !ok {
		return false
	}
	return time.Until(deadline) < 3*time.Second
}

func pickResolved(byIdx map[int]domain.Song, n int, seed lastfmPair, cap int, sameArtistOK bool) []domain.Song {
	seen := map[string]bool{songKey(seed.artist, seed.title): true}
	seenArtists := map[string]bool{}
	out := make([]domain.Song, 0, cap)
	for i := 0; i < n && len(out) < cap; i++ {
		song, ok := byIdx[i]
		if !ok {
			continue
		}
		k := songKey(song.Artist, song.Title)
		if k == "" || seen[k] {
			continue
		}
		art := utils.NormalizeString(song.Artist)
		// Soft diversity for radio/because — albums must keep same-artist tracks.
		if !sameArtistOK && seenArtists[art] && len(out) < cap-1 && i < n-1 {
			continue
		}
		seen[k] = true
		seenArtists[art] = true
		out = append(out, song)
	}
	return out
}

func (h *RecommendHandler) resolveOne(ctx context.Context, want, seed lastfmPair, bypassCache bool) (domain.Song, bool) {
	cacheKey := songKey(want.artist, want.title)
	if !bypassCache {
		if song, ok := h.resolveSnap(cacheKey); ok {
			if seedKey := songKey(seed.artist, seed.title); seedKey == "" || songKey(song.Artist, song.Title) != seedKey {
				// Re-validate cached rows — older caches may predate the title≠audio fix.
				if services.IsPlayableMatch(want.artist, want.title, song) {
					return song, true
				}
			}
		}
	}

	// First-wins by provider priority — explore/radio don't need a full merge.
	songs, err := h.search.SearchFirst(ctx, want.artist+" "+want.title, recommendSearchPeek)
	if err != nil || len(songs) == 0 {
		return domain.Song{}, false
	}

	seedKey := songKey(seed.artist, seed.title)
	song, ok := services.PickPlayableSong(want.artist, want.title, songs, seedKey, recommendSearchPeek)
	if !ok {
		// Never return an unrelated playable hit under this want key — that stamped the wrong
		// stream URL onto radio/explore/vault rows (UI title ≠ audio).
		return domain.Song{}, false
	}
	h.resolveStore(cacheKey, song)
	return song, true
}

func (h *RecommendHandler) resolveSnap(key string) (domain.Song, bool) {
	if key == "" {
		return domain.Song{}, false
	}
	h.resolveMu.Lock()
	defer h.resolveMu.Unlock()
	e, ok := h.resolveCache[key]
	if !ok || e.song.Link == "" || time.Since(e.at) > resolveTTL {
		return domain.Song{}, false
	}
	return e.song, true
}

func (h *RecommendHandler) resolveStore(key string, song domain.Song) {
	if key == "" || song.Link == "" {
		return
	}
	h.resolveMu.Lock()
	defer h.resolveMu.Unlock()
	if h.resolveCache == nil {
		h.resolveCache = make(map[string]resolveEntry)
	}
	if len(h.resolveCache) >= resolveCacheCap {
		for k, e := range h.resolveCache {
			if time.Since(e.at) > resolveTTL {
				delete(h.resolveCache, k)
			}
		}
		if len(h.resolveCache) >= resolveCacheCap {
			h.resolveCache = make(map[string]resolveEntry, resolveCacheCap/2)
		}
	}
	h.resolveCache[key] = resolveEntry{song: song, at: time.Now()}
}
