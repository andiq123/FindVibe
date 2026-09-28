package handlers

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/core/services"
)

type stubSearch struct {
	hits map[string][]domain.Song
}

func (s stubSearch) Search(_ context.Context, query string, _ int) (*domain.SearchResponse, error) {
	if songs, ok := s.hits[strings.ToLower(query)]; ok {
		return domain.NewSearchResponse(songs, nil), nil
	}
	return domain.NewSearchResponse(nil, nil), nil
}

func (s stubSearch) SearchWithProgress(
	ctx context.Context,
	query string,
	page int,
	onMeta func(domain.SearchProgress) error,
	onSong func(domain.Song) error,
) (*domain.SearchResponse, error) {
	resp, err := s.Search(ctx, query, page)
	if err != nil || resp == nil {
		return resp, err
	}
	if onMeta != nil {
		if err := onMeta(domain.SearchProgress{
			Artists:    resp.Artists,
			Albums:     resp.Albums,
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

func (s stubSearch) SearchFirst(ctx context.Context, query string, limit int) ([]domain.Song, error) {
	resp, err := s.Search(ctx, query, 1)
	if err != nil || resp == nil {
		return nil, err
	}
	songs := resp.Songs
	if limit > 0 && len(songs) > limit {
		songs = songs[:limit]
	}
	return songs, nil
}

func TestCoreTitleStripsRemixVariants(t *testing.T) {
	a := coreTitle("Collide (Extended Mix)")
	b := coreTitle("Collide feat Rosi Golan [Extended Mix]")
	c := coreTitle("Collide (Original Mix)")
	if a != "collide" || b != "collide" || c != "collide" {
		t.Fatalf("got %q %q %q", a, b, c)
	}
}

func TestParseChartTitle(t *testing.T) {
	cases := []struct {
		raw, artist, title string
	}{
		{"VANILLA x ALEX VELEA - 7 din 7 (VIDEOCLIP OFICIAL)", "VANILLA", "7 din 7"},
		{"Denis Ramniceanu x BABASHA - Vara nu e vara | Official video", "Denis Ramniceanu", "Vara nu e vara"},
		{"IAN x AZTECA - FOCU'", "IAN", "FOCU'"},
		{"MIRA - Caramela | Official Video", "MIRA", "Caramela"},
	}
	for _, tc := range cases {
		a, title, ok := parseChartTitle(tc.raw)
		if !ok || a != tc.artist || title != tc.title {
			t.Fatalf("%q → (%q, %q, %v) want (%q, %q)", tc.raw, a, title, ok, tc.artist, tc.title)
		}
	}
}

func TestParseKworbMusicPairsUsesMusicTable(t *testing.T) {
	html := `
<div class="overall"><table><tbody>
<tr><td class="text"><div><a href="#">Gaming video not music</a></div></td></tr>
</tbody></table></div>
<div class="music" style="display: none;"><table id="trendingcountry"><tbody>
<tr><td class="text"><div><a href="#">VANILLA x ALEX VELEA - 7 din 7 | Official</a></div></td></tr>
<tr><td class="text"><div><a href="#">MIRA - Caramela | Official Video</a></div></td></tr>
</tbody></table></div>`
	got := parseKworbMusicPairs(html)
	if len(got) != 2 || got[0].artist != "VANILLA" || got[1].title != "Caramela" {
		t.Fatalf("got %+v", got)
	}
}

func TestUniquePairsDropsRemixDupes(t *testing.T) {
	seed := lastfmPair{"Vicetone", "Collide"}
	got := uniquePairs([]lastfmPair{
		{"Vicetone", "Collide (Original Mix)"},
		{"Vicetone", "Collide (Extended Mix)"},
		{"Nero", "Promises"},
		{"Nero", "Promises (Original Mix)"},
	}, seed)
	if len(got) != 1 || got[0].artist != "Nero" {
		t.Fatalf("got %+v", got)
	}
}

func TestArtistCandidatesSplitsCollabs(t *testing.T) {
	got := artistCandidates("Satoshi, magnat, feoctist")
	if len(got) < 3 || got[0] != "Satoshi" {
		t.Fatalf("want Satoshi first, got %v", got)
	}
}

func TestSearchFallbackByArtist(t *testing.T) {
	h := &RecommendHandler{
		search: stubSearch{hits: map[string][]domain.Song{
			"satoshi": {
				{Title: "Pațanii", Artist: "Satoshi, magnat, feoctist", Link: "https://seed.mp3"},
				{Title: "Viva, Moldova!", Artist: "Satoshi", Link: "https://a.mp3"},
				{Title: "Bate, vântule", Artist: "Satoshi, Irina Rimes", Link: "https://b.mp3"},
			},
		}},
	}
	seed := lastfmPair{"Satoshi, magnat, feoctist", "Pațanii"}
	got := h.searchFallback(context.Background(), []string{"Satoshi"}, seed)
	if len(got) != 2 || got[0].Link != "https://a.mp3" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveSkipsSeedAndRemixDupes(t *testing.T) {
	h := &RecommendHandler{
		search: stubSearch{hits: map[string][]domain.Song{
			"vicetone collide": {
				{Title: "Collide (Extended Mix)", Artist: "Vicetone", Link: "https://a.mp3"},
			},
			"nero promises": {
				{Title: "Promises", Artist: "Nero", Link: "https://b.mp3"},
				{Title: "Promises (Remix)", Artist: "Nero", Link: "https://c.mp3"},
			},
			"flux pavilion bass cannon": {
				{Title: "Bass Cannon", Artist: "Flux Pavilion", Link: "https://d.mp3"},
			},
		}},
	}
	seed := lastfmPair{"Vicetone", "Collide"}
	got := h.resolve(context.Background(), []lastfmPair{
		{"Vicetone", "Collide"},
		{"Nero", "Promises"},
		{"Flux Pavilion", "Bass Cannon"},
	}, seed)
	if len(got) != 2 {
		t.Fatalf("want 2 unique other tracks, got %+v", got)
	}
	if got[0].Link != "https://b.mp3" || got[1].Link != "https://d.mp3" {
		t.Fatalf("got %+v", got)
	}
}

func TestPickResolvedAlbumKeepsSameArtist(t *testing.T) {
	byIdx := map[int]domain.Song{
		0: {Title: "One", Artist: "Nero", Link: "https://a.mp3"},
		1: {Title: "Two", Artist: "Nero", Link: "https://b.mp3"},
		2: {Title: "Three", Artist: "Nero", Link: "https://c.mp3"},
	}
	// Diversity keeps the first hit and may admit the last slot — not a full album.
	diverse := pickResolved(byIdx, 3, lastfmPair{}, 3, false)
	if len(diverse) >= 3 {
		t.Fatalf("diversity must not keep a full same-artist album, got %+v", diverse)
	}
	album := pickResolved(byIdx, 3, lastfmPair{}, 3, true)
	if len(album) != 3 {
		t.Fatalf("album mode must keep same-artist tracks, got %+v", album)
	}
}

func TestExploreCacheRoundTrip(t *testing.T) {
	h := &RecommendHandler{}
	if _, ok := h.exploreSnap(true); ok {
		t.Fatal("empty cache should miss")
	}
	h.exploreStore([]ExploreSection{{
		ID: "romania", Title: "Romania",
		Songs: []domain.Song{{Title: "A", Artist: "B", Link: "https://x.mp3"}},
	}})
	got, ok := h.exploreSnap(true)
	if !ok || len(got) != 1 || got[0].ID != "romania" || len(got[0].Songs) != 1 {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	status := h.exploreCacheStatus()
	if status["cached"] != true || status["fresh"] != true || status["sections"] != 1 || status["songs"] != 1 {
		t.Fatalf("fresh status %+v", status)
	}
	if rem, _ := status["remainingSeconds"].(int64); rem <= 0 || rem > int64(exploreTTL/time.Second) {
		t.Fatalf("remainingSeconds %+v", status["remainingSeconds"])
	}
	// Stale-beyond-TTL still serves last good payload.
	h.exploreAt = h.exploreAt.Add(-exploreTTL - time.Minute)
	if _, ok := h.exploreSnap(true); ok {
		t.Fatal("fresh snap should miss after TTL")
	}
	stale, ok := h.exploreSnap(false)
	if !ok || len(stale) != 1 {
		t.Fatalf("stale snap got %+v ok=%v", stale, ok)
	}
	staleStatus := h.exploreCacheStatus()
	if staleStatus["cached"] != true || staleStatus["fresh"] != false || staleStatus["remainingSeconds"] != int64(0) {
		t.Fatalf("stale status %+v", staleStatus)
	}
}

func TestResolveStrictRejectsWeakHits(t *testing.T) {
	h := &RecommendHandler{
		search: stubSearch{hits: map[string][]domain.Song{
			"nero collide": {
				{Title: "Promises", Artist: "Nero", Link: "https://wrong.mp3"},
			},
		}},
	}
	want := lastfmPair{"Nero", "Collide"}
	if _, ok := h.resolveOne(context.Background(), want, lastfmPair{}, false); ok {
		t.Fatal("must reject artist-only / wrong-title hit")
	}
}

func TestResolveAcceptsTitleArtistOverlap(t *testing.T) {
	h := &RecommendHandler{
		search: stubSearch{hits: map[string][]domain.Song{
			"nero promises": {
				{Title: "Promises (Original Mix)", Artist: "Nero", Link: "https://ok.mp3"},
			},
		}},
	}
	want := lastfmPair{"Nero", "Promises"}
	got, ok := h.resolveOne(context.Background(), want, lastfmPair{}, false)
	if !ok || got.Link != "https://ok.mp3" {
		t.Fatalf("expected overlap match, got %+v ok=%v", got, ok)
	}
}

func TestResolvePicksBestAmongPeek(t *testing.T) {
	h := &RecommendHandler{
		search: stubSearch{hits: map[string][]domain.Song{
			"magnat, feoctist auzeam, gheorghe": {
				{Title: "Miagy", Artist: "Someone", Link: "https://wrong.mp3"},
				{Title: "Auzeam, Gheorghe (Remix)", Artist: "magnat, feoctist", Link: "https://remix.mp3"},
				{Title: "Auzeam, Gheorghe", Artist: "magnat, feoctist", Link: "https://right.mp3"},
			},
		}},
	}
	want := lastfmPair{"magnat, feoctist", "Auzeam, Gheorghe"}
	got, ok := h.resolveOne(context.Background(), want, lastfmPair{}, false)
	if !ok || got.Link != "https://right.mp3" {
		t.Fatalf("want clean title hit, got %+v ok=%v", got, ok)
	}
}

func TestIsPlayableMatchRejectsShortAmbiguousTitle(t *testing.T) {
	got := domain.Song{Title: "Someone Like You", Artist: "Adele", Link: "https://a.mp3"}
	if services.IsPlayableMatch("Adele", "You", got) {
		t.Fatal("short core must not Contains-match a longer title")
	}
	if !services.IsPlayableMatch("Adele", "Someone Like You", got) {
		t.Fatal("exact core should match")
	}
}

func TestResolveIgnoresStaleWrongCache(t *testing.T) {
	h := NewRecommendHandler(nil, "", stubSearch{hits: map[string][]domain.Song{
		"nero promises": {
			{Title: "Promises", Artist: "Nero", Link: "https://ok.mp3"},
		},
	}}, nil)
	want := lastfmPair{"Nero", "Promises"}
	key := songKey(want.artist, want.title)
	h.resolveStore(key, domain.Song{Title: "Miagy", Artist: "Wrong", Link: "https://stale.mp3"})
	got, ok := h.resolveOne(context.Background(), want, lastfmPair{}, false)
	if !ok || got.Link != "https://ok.mp3" {
		t.Fatalf("stale wrong cache must be revalidated, got %+v ok=%v", got, ok)
	}
}

func TestResolveRefreshBypassesCache(t *testing.T) {
	h := NewRecommendHandler(nil, "", stubSearch{hits: map[string][]domain.Song{
		"nero promises": {
			{Title: "Promises", Artist: "Nero", Link: "https://fresh.mp3"},
		},
	}}, nil)
	want := lastfmPair{"Nero", "Promises"}
	key := songKey(want.artist, want.title)
	h.resolveStore(key, domain.Song{Title: "Promises", Artist: "Nero", Link: "https://cached.mp3"})
	cached, ok := h.resolveOne(context.Background(), want, lastfmPair{}, false)
	if !ok || cached.Link != "https://cached.mp3" {
		t.Fatalf("warm cache should hit, got %+v ok=%v", cached, ok)
	}
	fresh, ok := h.resolveOne(context.Background(), want, lastfmPair{}, true)
	if !ok || fresh.Link != "https://fresh.mp3" {
		t.Fatalf("refresh must bypass cache, got %+v ok=%v", fresh, ok)
	}
}

func TestStreamProxyAllowedHosts(t *testing.T) {
	allow := []string{
		"https://cs1.mp3.pm/listen/a.mp3",
		"https://mn1.sunproxy.net/file/abc/x.mp3",
		"https://musify.club/track/pl/1/x.mp3",
	}
	for _, raw := range allow {
		u, err := url.Parse(raw)
		if err != nil || !streamProxyAllowed(u) {
			t.Fatalf("expected allow %s", raw)
		}
	}
	deny := []string{
		"http://cs1.mp3.pm/listen/a.mp3",
		"https://evil.example/a.mp3",
		"https://mp3.pm.evil.com/a.mp3",
	}
	for _, raw := range deny {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", raw, err)
		}
		if streamProxyAllowed(u) {
			t.Fatalf("expected deny %s", raw)
		}
	}
}

func TestMapLastfmAlbumsCleansAndDedupes(t *testing.T) {
	got := mapLastfmAlbums("Cher", []lastfmAlbumRow{
		{Name: "Believe", Playcount: "100", Artist: struct {
			Name string `json:"name"`
		}{Name: "Cher"}, Image: []lastfmImage{{URL: "http://img/a.jpg", Size: "large"}}},
		{Name: "(null)"},
		{Name: "  "},
		{Name: "Believe"}, // dupe
		{Name: "The Very Best of Cher", Playcount: float64(50), Artist: struct {
			Name string `json:"name"`
		}{Name: ""}},
	}, 20)
	if len(got) != 2 {
		t.Fatalf("want 2 albums, got %+v", got)
	}
	if got[0].Name != "Believe" || got[0].Image != "https://img/a.jpg" || got[0].Playcount != 100 {
		t.Fatalf("first album %+v", got[0])
	}
	if got[1].Name != "The Very Best of Cher" || got[1].Artist != "Cher" {
		t.Fatalf("second album %+v", got[1])
	}
}

func TestRecommendCacheRoundTrip(t *testing.T) {
	h := NewRecommendHandler(nil, "", stubSearch{}, nil)
	key := songKey("Nero", "Promises")
	if _, ok := h.recommendSnap(key); ok {
		t.Fatal("empty recommend cache should miss")
	}
	songs := []domain.Song{{Title: "Bass Cannon", Artist: "Flux Pavilion", Link: "https://d.mp3"}}
	h.recommendStore(key, songs)
	got, ok := h.recommendSnap(key)
	if !ok || len(got) != 1 || got[0].Link != "https://d.mp3" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	// Mutating returned slice must not poison the cache.
	got[0].Link = "mutated"
	again, _ := h.recommendSnap(key)
	if again[0].Link != "https://d.mp3" {
		t.Fatalf("cache poisoned: %q", again[0].Link)
	}
}
