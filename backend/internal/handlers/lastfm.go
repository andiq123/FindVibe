package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/utils"
)

func (h *RecommendHandler) lastfmSimilar(ctx context.Context, artist, title string) ([]lastfmPair, error) {
	return h.lastfmTracks(ctx, url.Values{
		"method": {"track.getSimilar"},
		"artist": {artist},
		"track":  {title},
		"limit":  {"25"},
	}, "similartracks")
}

func (h *RecommendHandler) lastfmArtistTop(ctx context.Context, artist, skipTitle string) ([]lastfmPair, error) {
	pairs, err := h.lastfmTracks(ctx, url.Values{
		"method": {"artist.getTopTracks"},
		"artist": {artist},
		"limit":  {"12"},
	}, "toptracks")
	if err != nil {
		return nil, err
	}
	out := make([]lastfmPair, 0, len(pairs))
	skipCore := coreTitle(skipTitle)
	for _, p := range pairs {
		if coreTitle(p.title) == skipCore {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// pairsFromSimilarArtists: artist.getSimilar → one top track each (different artists by design).
func (h *RecommendHandler) pairsFromSimilarArtists(ctx context.Context, artist, skipTitle string) ([]lastfmPair, error) {
	names, err := h.lastfmSimilarArtists(ctx, artist)
	if err != nil || len(names) == 0 {
		return nil, err
	}
	if len(names) > recommendResolveCap+2 {
		names = names[:recommendResolveCap+2]
	}

	type slot struct {
		i    int
		pair lastfmPair
		ok   bool
	}
	ch := make(chan slot, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			tops, err := h.lastfmArtistTop(ctx, name, skipTitle)
			if err != nil || len(tops) == 0 {
				ch <- slot{i: i}
				return
			}
			ch <- slot{i: i, pair: tops[0], ok: true}
		}(i, name)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	byIdx := make(map[int]lastfmPair, len(names))
	for s := range ch {
		if s.ok {
			byIdx[s.i] = s.pair
		}
	}
	out := make([]lastfmPair, 0, len(byIdx))
	for i := 0; i < len(names); i++ {
		if p, ok := byIdx[i]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (h *RecommendHandler) lastfmSimilarArtists(ctx context.Context, artist string) ([]string, error) {
	q := url.Values{
		"method":  {"artist.getSimilar"},
		"artist":  {artist},
		"api_key": {h.apiKey},
		"format":  {"json"},
		"limit":   {"10"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ws.audioscrobbler.com/2.0/?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var data struct {
		SimilarArtists struct {
			Artist json.RawMessage `json:"artist"`
		} `json:"similarartists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	artists, err := decodeLastfmArtistList(data.SimilarArtists.Artist)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(artists))
	seed := utils.NormalizeString(artist)
	for _, a := range artists {
		n := strings.TrimSpace(a.Name)
		if n == "" || utils.NormalizeString(n) == seed {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func (h *RecommendHandler) lastfmTopAlbums(ctx context.Context, artist string, page, limit int) ([]domain.ArtistAlbum, *domain.PaginationInfo, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	q := url.Values{
		"method":      {"artist.getTopAlbums"},
		"artist":      {artist},
		"api_key":     {h.apiKey},
		"format":      {"json"},
		"autocorrect": {"1"},
		"limit":       {strconv.Itoa(limit)},
		"page":        {strconv.Itoa(page)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ws.audioscrobbler.com/2.0/?"+q.Encode(), nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil
	}

	var data struct {
		TopAlbums struct {
			Album json.RawMessage `json:"album"`
			Attr  struct {
				Page       string `json:"page"`
				PerPage    string `json:"perPage"`
				TotalPages string `json:"totalPages"`
				Total      string `json:"total"`
			} `json:"@attr"`
		} `json:"topalbums"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, nil, err
	}
	raw, err := decodeLastfmAlbumList(data.TopAlbums.Album)
	if err != nil {
		return nil, nil, err
	}
	pag := &domain.PaginationInfo{
		CurrentPage: page,
		HasPrevPage: page > 1,
	}
	if p, err := strconv.Atoi(strings.TrimSpace(data.TopAlbums.Attr.Page)); err == nil && p > 0 {
		pag.CurrentPage = p
	}
	perPage := limit
	if p, err := strconv.Atoi(strings.TrimSpace(data.TopAlbums.Attr.PerPage)); err == nil && p > 0 {
		perPage = p
	}
	if t, err := strconv.Atoi(strings.TrimSpace(data.TopAlbums.Attr.Total)); err == nil {
		pag.TotalResults = t
	}
	if tp, err := strconv.Atoi(strings.TrimSpace(data.TopAlbums.Attr.TotalPages)); err == nil {
		pag.TotalPages = tp
	} else if pag.TotalResults > 0 && perPage > 0 {
		pag.TotalPages = (pag.TotalResults + perPage - 1) / perPage
	}
	pag.HasNextPage = pag.TotalPages > 0 && pag.CurrentPage < pag.TotalPages
	pag.HasPrevPage = pag.CurrentPage > 1

	return mapLastfmAlbums(artist, raw, limit), pag, nil
}

func (h *RecommendHandler) lastfmAlbumTracks(ctx context.Context, artist, album string) ([]lastfmPair, error) {
	q := url.Values{
		"method":      {"album.getInfo"},
		"artist":      {artist},
		"album":       {album},
		"api_key":     {h.apiKey},
		"format":      {"json"},
		"autocorrect": {"1"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ws.audioscrobbler.com/2.0/?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var data struct {
		Album struct {
			Artist string `json:"artist"`
			Tracks struct {
				Track json.RawMessage `json:"track"`
			} `json:"tracks"`
		} `json:"album"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	tracks, err := decodeLastfmTrackList(data.Album.Tracks.Track)
	if err != nil {
		return nil, err
	}
	fallbackArtist := strings.TrimSpace(data.Album.Artist)
	if fallbackArtist == "" {
		fallbackArtist = artist
	}
	out := make([]lastfmPair, 0, len(tracks))
	seen := map[string]bool{}
	for _, t := range tracks {
		title := strings.TrimSpace(t.Name)
		a := strings.TrimSpace(t.Artist.Name)
		if a == "" {
			a = fallbackArtist
		}
		if title == "" || a == "" {
			continue
		}
		k := songKey(a, title)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, lastfmPair{artist: a, title: title})
	}
	return out, nil
}

func (h *RecommendHandler) lastfmTracks(ctx context.Context, q url.Values, root string) ([]lastfmPair, error) {
	q.Set("api_key", h.apiKey)
	q.Set("format", "json")
	u := "https://ws.audioscrobbler.com/2.0/?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	block, ok := raw[root]
	if !ok {
		return nil, nil
	}
	var wrap struct {
		Track json.RawMessage `json:"track"`
	}
	if err := json.Unmarshal(block, &wrap); err != nil || len(wrap.Track) == 0 {
		return nil, nil
	}

	tracks, err := decodeLastfmTrackList(wrap.Track)
	if err != nil {
		return nil, err
	}
	out := make([]lastfmPair, 0, len(tracks))
	for _, t := range tracks {
		a, n := strings.TrimSpace(t.Artist.Name), strings.TrimSpace(t.Name)
		if a == "" || n == "" {
			continue
		}
		out = append(out, lastfmPair{a, n})
	}
	return out, nil
}

type lastfmTrack struct {
	Name   string `json:"name"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
}

type lastfmArtist struct {
	Name string `json:"name"`
}

type lastfmImage struct {
	URL  string `json:"#text"`
	Size string `json:"size"`
}

type lastfmAlbumRow struct {
	Name      string        `json:"name"`
	Playcount any           `json:"playcount"`
	Image     []lastfmImage `json:"image"`
	Artist    struct {
		Name string `json:"name"`
	} `json:"artist"`
}

func decodeLastfmAlbumList(raw json.RawMessage) ([]lastfmAlbumRow, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var many []lastfmAlbumRow
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var one lastfmAlbumRow
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	if strings.TrimSpace(one.Name) == "" {
		return nil, nil
	}
	return []lastfmAlbumRow{one}, nil
}

// mapLastfmAlbums drops null/empty/"(null)" rows and dedupes by normalized name.
func mapLastfmAlbums(fallbackArtist string, rows []lastfmAlbumRow, limit int) []domain.ArtistAlbum {
	if limit < 1 {
		limit = 20
	}
	seen := map[string]bool{}
	out := make([]domain.ArtistAlbum, 0, min(len(rows), limit))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" || strings.EqualFold(name, "(null)") || strings.EqualFold(name, "null") {
			continue
		}
		key := utils.NormalizeString(name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		artist := strings.TrimSpace(row.Artist.Name)
		if artist == "" {
			artist = strings.TrimSpace(fallbackArtist)
		}
		out = append(out, domain.ArtistAlbum{
			Name:      name,
			Artist:    artist,
			Image:     lastfmBestImage(row.Image),
			Playcount: lastfmPlaycount(row.Playcount),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func lastfmBestImage(images []lastfmImage) string {
	prefer := []string{"extralarge", "large", "medium", "small"}
	bySize := make(map[string]string, len(images))
	var first string
	for _, im := range images {
		u := utils.UpgradeHTTPS(im.URL)
		if u == "" || isLastfmStubImage(u) {
			continue
		}
		if first == "" {
			first = u
		}
		bySize[im.Size] = u
	}
	for _, size := range prefer {
		if u := bySize[size]; u != "" {
			return u
		}
	}
	return first
}

// Last.fm's grey star placeholder — treat as missing art.
func isLastfmStubImage(u string) bool {
	return strings.Contains(u, "2a96cbd8b46e442fc41c2b86b821562f")
}

func lastfmPlaycount(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case string:
		n = strings.TrimSpace(n)
		if n == "" {
			return 0
		}
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	default:
		return 0
	}
}

func decodeLastfmTrackList(raw json.RawMessage) ([]lastfmTrack, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var many []lastfmTrack
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var one lastfmTrack
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	if one.Name == "" {
		return nil, nil
	}
	return []lastfmTrack{one}, nil
}

func decodeLastfmArtistList(raw json.RawMessage) ([]lastfmArtist, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var many []lastfmArtist
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var one lastfmArtist
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	if one.Name == "" {
		return nil, nil
	}
	return []lastfmArtist{one}, nil
}
