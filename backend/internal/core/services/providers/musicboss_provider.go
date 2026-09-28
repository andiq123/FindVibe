package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
)

const MusicBossOrigin = "https://new.kachevo.org"
const HitmosOrigin = "https://eu.hitmoz.com"
const catalogPageSize = 48

// MusicBoss and Hitmos share pagination but expose different track markup.
type CatalogProvider struct {
	*BaseProvider
	origin string
	pace   *pacer
}

func NewMusicBossProvider(client *http.Client) *CatalogProvider {
	return &CatalogProvider{NewBaseProvider("MusicBoss", 8, client), MusicBossOrigin, newPacer(120 * time.Millisecond)}
}
func NewHitmosProvider(client *http.Client) *CatalogProvider {
	return &CatalogProvider{NewBaseProvider("Hitmos", 7, client), HitmosOrigin, newPacer(120 * time.Millisecond)}
}

func (p *CatalogProvider) SearchWithPage(ctx context.Context, query string, page int) ([]domain.ProviderResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	page = max(1, page)
	if err := p.pace.wait(ctx); err != nil {
		return nil, err
	}
	path := "/search"
	if page > 1 {
		path += "/start/" + strconv.Itoa((page-1)*catalogPageSize)
	}
	doc, err := p.fetchDocument(ctx, p.origin+path+"?"+url.Values{"q": {query}}.Encode(), p.origin+"/")
	if err != nil {
		return nil, err
	}
	// Do not cache a successful empty response when upstream markup changed.
	if doc.Find(".content-block, .p-info-title, .tracks__list").Length() == 0 {
		return nil, fmt.Errorf("%s: search markup missing", p.Name())
	}
	return p.parseResults(doc, page), nil
}

func (p *CatalogProvider) parseResults(doc *goquery.Document, page int) []domain.ProviderResult {
	pagination := &domain.PaginationInfo{CurrentPage: page, TotalPages: page, HasPrevPage: page > 1}
	doc.Find(".paggination a[href], .pagination a[href]").Each(func(_ int, s *goquery.Selection) {
		u, err := url.Parse(s.AttrOr("href", ""))
		if err != nil {
			return
		}
		offset, err := strconv.Atoi(strings.TrimPrefix(u.Path, "/search/start/"))
		if err == nil && offset >= 0 {
			pagination.TotalPages = max(pagination.TotalPages, offset/catalogPageSize+1)
		}
	})
	pagination.HasNextPage = pagination.TotalPages > page
	var results []domain.ProviderResult
	doc.Find(".playlist-list .playlist-item, .tracks__list [data-musmeta]").Each(func(_ int, s *goquery.Selection) {
		var track struct {
			Artist string `json:"artist"`
			Title  string `json:"title"`
			URL    string `json:"url"`
			Image  string `json:"img"`
		}
		if raw, ok := s.Attr("data-musmeta"); ok {
			if json.Unmarshal([]byte(raw), &track) != nil {
				return
			}
		} else {
			track.Artist = text(s.Find(".playlist-item-subtitle"))
			track.Title = text(s.Find(".playlist-item-title"))
			track.URL = s.Find(".playlist-item-play").AttrOr("data-url", "")
			track.Image = s.Find(".playlist-item-img").AttrOr("data-img", "")
		}
		link := p.catalogURL(track.URL)
		if track.Artist == "" || track.Title == "" || link == "" {
			return
		}
		image := ""
		if track.Image != "" && !strings.Contains(track.Image, "no-cover") {
			base, _ := url.Parse(p.origin)
			if u, err := url.Parse(track.Image); err == nil {
				image = playableLink(base.ResolveReference(u).String())
			}
		}
		song := domain.NewSong(strings.TrimSpace(track.Title), strings.TrimSpace(track.Artist), image, link)
		results = append(results, domain.NewProviderResult(*song, p.Name(), len(results)+1, pagination))
	})
	return results
}

func (p *CatalogProvider) catalogURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	base, _ := url.Parse(p.origin)
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	u = base.ResolveReference(u)
	if u.Host != base.Host || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || !strings.HasPrefix(u.Path, "/get/music/") {
		return ""
	}
	u.Scheme = "https"
	return u.String()
}
