package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/utils"
)

const (
	catalogArtistLimit       = 6 // artist.search rows for discovery UI
	catalogArtistTopN        = 3 // artists that expand into top tracks
	catalogTracksPerArtist   = 2
	catalogAlbumsForTopArtist = 8
)

// CatalogHit is a Last.fm discovery row before provider mapping.
type CatalogHit struct {
	Artist string
	Title  string
	Image  string
}

// CatalogArtist is a Last.fm artist.search match for search UI.
type CatalogArtist struct {
	Name  string
	Image string
}

// CatalogPage is one discovery page: tracks to map + artists/albums for UI.
type CatalogPage struct {
	Hits       []CatalogHit
	Artists    []CatalogArtist
	Pagination *domain.PaginationInfo
}

// LastFMCatalog discovers tracks/artists via Last.fm only (no scrapers).
type LastFMCatalog struct {
	client *http.Client
	apiKey string
}

func NewLastFMCatalog(client *http.Client, apiKey string) *LastFMCatalog {
	return &LastFMCatalog{client: client, apiKey: strings.TrimSpace(apiKey)}
}

func (l *LastFMCatalog) Configured() bool {
	return l != nil && l.apiKey != "" && l.client != nil
}

// Search returns Last.fm track matches (+ page-1 artist top-tracks) and artist matches.
func (l *LastFMCatalog) Search(ctx context.Context, query string, page, limit int) (CatalogPage, error) {
	if !l.Configured() {
		return CatalogPage{}, fmt.Errorf("lastfm catalog: %w", domain.ErrUnavailable)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return CatalogPage{}, nil
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	tracks, pag, err := l.trackSearch(ctx, query, page, limit)
	if err != nil {
		return CatalogPage{}, err
	}

	out := append([]CatalogHit(nil), tracks...)
	seen := make(map[string]struct{}, len(out)+8)
	for _, h := range out {
		if k := SongKey(h.Artist, h.Title); k != "" {
			seen[k] = struct{}{}
		}
	}

	var artists []CatalogArtist
	// Artist matches only on page 1 — keeps pagination honest to track.search pages.
	if page == 1 {
		artists = l.artistMatches(ctx, query, catalogArtistLimit)
		for _, h := range l.artistTopHitsFrom(ctx, artists) {
			k := SongKey(h.Artist, h.Title)
			if k == "" {
				continue
			}
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, h)
		}
	}
	return CatalogPage{Hits: out, Artists: artists, Pagination: pag}, nil
}

// TopAlbums returns Last.fm artist.getTopAlbums (browse-only), most popular order.
// page matches Last.fm discography paging (/music/ARTIST/+albums?page=N).
func (l *LastFMCatalog) TopAlbums(ctx context.Context, artist string, limit, page int) ([]domain.ArtistAlbum, *domain.PaginationInfo, error) {
	if !l.Configured() {
		return nil, nil, fmt.Errorf("lastfm catalog: %w", domain.ErrUnavailable)
	}
	artist = strings.TrimSpace(artist)
	if artist == "" {
		return nil, nil, nil
	}
	if limit < 1 {
		limit = catalogAlbumsForTopArtist
	}
	if page < 1 {
		page = 1
	}
	q := url.Values{
		"method":      {"artist.getTopAlbums"},
		"artist":      {artist},
		"api_key":     {l.apiKey},
		"format":      {"json"},
		"autocorrect": {"1"},
		"limit":       {strconv.Itoa(limit)},
		"page":        {strconv.Itoa(page)},
	}
	raw, err := l.getJSON(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	var payload struct {
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
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, nil, err
	}
	rows, err := decodeCatalogAlbums(payload.TopAlbums.Album)
	if err != nil {
		return nil, nil, err
	}
	pag := paginationFromAttr(
		page,
		limit,
		payload.TopAlbums.Attr.Page,
		payload.TopAlbums.Attr.PerPage,
		payload.TopAlbums.Attr.TotalPages,
		payload.TopAlbums.Attr.Total,
	)
	return mapCatalogAlbums(artist, rows, limit), pag, nil
}

// AlbumSearch returns Last.fm album.search matches for the query.
func (l *LastFMCatalog) AlbumSearch(ctx context.Context, query string, limit int) ([]domain.ArtistAlbum, error) {
	if !l.Configured() {
		return nil, fmt.Errorf("lastfm catalog: %w", domain.ErrUnavailable)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit < 1 {
		limit = catalogAlbumsForTopArtist
	}
	q := url.Values{
		"method":  {"album.search"},
		"album":   {query},
		"api_key": {l.apiKey},
		"format":  {"json"},
		"limit":   {strconv.Itoa(limit)},
	}
	raw, err := l.getJSON(ctx, q)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Results struct {
			AlbumMatches struct {
				Album json.RawMessage `json:"album"`
			} `json:"albummatches"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	rows, err := decodeCatalogAlbums(payload.Results.AlbumMatches.Album)
	if err != nil {
		return nil, err
	}
	return mapCatalogAlbums("", rows, limit), nil
}

func (l *LastFMCatalog) artistMatches(ctx context.Context, query string, limit int) []CatalogArtist {
	rows, err := l.artistSearchRows(ctx, query, limit)
	if err != nil || len(rows) == 0 {
		return nil
	}
	out := make([]CatalogArtist, 0, len(rows))
	seen := map[string]struct{}{}
	for _, a := range rows {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			continue
		}
		key := utils.NormalizeString(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, CatalogArtist{
			Name:  name,
			Image: utils.UpgradeHTTPS(bestSearchImage(a.Image)),
		})
	}
	return out
}

func (l *LastFMCatalog) artistTopHitsFrom(ctx context.Context, artists []CatalogArtist) []CatalogHit {
	names := artists
	if len(names) > catalogArtistTopN {
		names = names[:catalogArtistTopN]
	}
	if len(names) == 0 {
		return nil
	}

	type slot struct {
		i    int
		hits []CatalogHit
	}
	ch := make(chan slot, len(names))
	var wg sync.WaitGroup
	for i, a := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			tops, err := l.artistTopTracks(ctx, name, catalogTracksPerArtist)
			if err != nil || len(tops) == 0 {
				ch <- slot{i: i}
				return
			}
			ch <- slot{i: i, hits: tops}
		}(i, a.Name)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	byIdx := make(map[int][]CatalogHit, len(names))
	for s := range ch {
		if len(s.hits) > 0 {
			byIdx[s.i] = s.hits
		}
	}
	out := make([]CatalogHit, 0, len(names)*catalogTracksPerArtist)
	for i := 0; i < len(names); i++ {
		out = append(out, byIdx[i]...)
	}
	return out
}

func (l *LastFMCatalog) trackSearch(ctx context.Context, query string, page, limit int) ([]CatalogHit, *domain.PaginationInfo, error) {
	q := url.Values{
		"method":  {"track.search"},
		"track":   {query},
		"page":    {strconv.Itoa(page)},
		"limit":   {strconv.Itoa(limit)},
		"api_key": {l.apiKey},
		"format":  {"json"},
	}
	raw, err := l.getJSON(ctx, q)
	if err != nil {
		return nil, nil, err
	}

	var payload struct {
		Results struct {
			TotalResults string `json:"opensearch:totalResults"`
			ItemsPerPage string `json:"opensearch:itemsPerPage"`
			TrackMatches struct {
				Track json.RawMessage `json:"track"`
			} `json:"trackmatches"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, nil, err
	}

	rows, err := decodeSearchTracks(payload.Results.TrackMatches.Track)
	if err != nil {
		return nil, nil, err
	}
	hits := make([]CatalogHit, 0, len(rows))
	for _, r := range rows {
		artist, title := strings.TrimSpace(r.Artist), strings.TrimSpace(r.Name)
		if artist == "" || title == "" {
			continue
		}
		hits = append(hits, CatalogHit{
			Artist: artist,
			Title:  title,
			Image:  utils.UpgradeHTTPS(bestSearchImage(r.Image)),
		})
	}

	total, _ := strconv.Atoi(payload.Results.TotalResults)
	perPage, _ := strconv.Atoi(payload.Results.ItemsPerPage)
	if perPage <= 0 {
		perPage = limit
	}
	totalPages := 0
	if total > 0 && perPage > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	pag := &domain.PaginationInfo{
		CurrentPage:  page,
		TotalResults: total,
		HasNextPage:  totalPages > 0 && page < totalPages,
		HasPrevPage:  page > 1,
		TotalPages:   totalPages,
	}
	return hits, pag, nil
}

// paginationFromAttr builds PaginationInfo from Last.fm @attr fields (albums / charts).
func paginationFromAttr(reqPage, fallbackPerPage int, pageS, perPageS, totalPagesS, totalS string) *domain.PaginationInfo {
	page := reqPage
	if p, err := strconv.Atoi(strings.TrimSpace(pageS)); err == nil && p > 0 {
		page = p
	}
	perPage := fallbackPerPage
	if p, err := strconv.Atoi(strings.TrimSpace(perPageS)); err == nil && p > 0 {
		perPage = p
	}
	total, _ := strconv.Atoi(strings.TrimSpace(totalS))
	totalPages, _ := strconv.Atoi(strings.TrimSpace(totalPagesS))
	if totalPages <= 0 && total > 0 && perPage > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	return &domain.PaginationInfo{
		CurrentPage:  page,
		TotalResults: total,
		HasNextPage:  totalPages > 0 && page < totalPages,
		HasPrevPage:  page > 1,
		TotalPages:   totalPages,
	}
}

func (l *LastFMCatalog) artistSearchRows(ctx context.Context, query string, limit int) ([]searchArtistRow, error) {
	q := url.Values{
		"method":  {"artist.search"},
		"artist":  {query},
		"limit":   {strconv.Itoa(limit)},
		"api_key": {l.apiKey},
		"format":  {"json"},
	}
	raw, err := l.getJSON(ctx, q)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Results struct {
			ArtistMatches struct {
				Artist json.RawMessage `json:"artist"`
			} `json:"artistmatches"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return decodeSearchArtists(payload.Results.ArtistMatches.Artist)
}

func (l *LastFMCatalog) artistTopTracks(ctx context.Context, artist string, limit int) ([]CatalogHit, error) {
	q := url.Values{
		"method":  {"artist.getTopTracks"},
		"artist":  {artist},
		"limit":   {strconv.Itoa(limit)},
		"api_key": {l.apiKey},
		"format":  {"json"},
	}
	raw, err := l.getJSON(ctx, q)
	if err != nil {
		return nil, err
	}
	var payload struct {
		TopTracks struct {
			Track json.RawMessage `json:"track"`
		} `json:"toptracks"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	tracks, err := decodeLastFMTrackList(payload.TopTracks.Track)
	if err != nil {
		return nil, err
	}
	out := make([]CatalogHit, 0, len(tracks))
	for _, t := range tracks {
		a := strings.TrimSpace(t.Artist.Name)
		if a == "" {
			a = artist
		}
		title := strings.TrimSpace(t.Name)
		if a == "" || title == "" {
			continue
		}
		out = append(out, CatalogHit{
			Artist: a,
			Title:  title,
			Image:  utils.UpgradeHTTPS(bestSearchImage(t.Image)),
		})
	}
	return out, nil
}

func (l *LastFMCatalog) getJSON(ctx context.Context, q url.Values) (json.RawMessage, error) {
	u := "https://ws.audioscrobbler.com/2.0/?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lastfm: status %d", resp.StatusCode)
	}
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

type searchTrackRow struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
	Image  []struct {
		URL  string `json:"#text"`
		Size string `json:"size"`
	} `json:"image"`
}

type searchArtistRow struct {
	Name  string `json:"name"`
	Image []struct {
		URL  string `json:"#text"`
		Size string `json:"size"`
	} `json:"image"`
}

type catalogAlbumRow struct {
	Name      string `json:"name"`
	Playcount any    `json:"playcount"`
	Image     []struct {
		URL  string `json:"#text"`
		Size string `json:"size"`
	} `json:"image"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
}

type lastfmTrackRow struct {
	Name   string `json:"name"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
	Image []struct {
		URL  string `json:"#text"`
		Size string `json:"size"`
	} `json:"image"`
}

func decodeSearchTracks(raw json.RawMessage) ([]searchTrackRow, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var many []searchTrackRow
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var one searchTrackRow
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	if one.Name == "" {
		return nil, nil
	}
	return []searchTrackRow{one}, nil
}

func decodeSearchArtists(raw json.RawMessage) ([]searchArtistRow, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var many []searchArtistRow
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var one searchArtistRow
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	if one.Name == "" {
		return nil, nil
	}
	return []searchArtistRow{one}, nil
}

func decodeLastFMTrackList(raw json.RawMessage) ([]lastfmTrackRow, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var many []lastfmTrackRow
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var one lastfmTrackRow
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	if one.Name == "" {
		return nil, nil
	}
	return []lastfmTrackRow{one}, nil
}

func decodeCatalogAlbums(raw json.RawMessage) ([]catalogAlbumRow, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var many []catalogAlbumRow
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var one catalogAlbumRow
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	if strings.TrimSpace(one.Name) == "" {
		return nil, nil
	}
	return []catalogAlbumRow{one}, nil
}

func mapCatalogAlbums(fallbackArtist string, rows []catalogAlbumRow, limit int) []domain.ArtistAlbum {
	if limit < 1 {
		limit = catalogAlbumsForTopArtist
	}
	seen := map[string]bool{}
	out := make([]domain.ArtistAlbum, 0, limit)
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
			Image:     utils.UpgradeHTTPS(bestSearchImage(row.Image)),
			Playcount: catalogPlaycount(row.Playcount),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func catalogPlaycount(v any) int64 {
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

func bestSearchImage(images []struct {
	URL  string `json:"#text"`
	Size string `json:"size"`
}) string {
	prefer := []string{"extralarge", "large", "medium", "small"}
	bySize := make(map[string]string, len(images))
	var first string
	for _, im := range images {
		u := strings.TrimSpace(im.URL)
		if u == "" || strings.Contains(u, "2a96cbd8b46e442fc41c2b86b821562f") {
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
