package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
)

const musifyOrigin = "https://musify.club"

var musifyPace = newPacer(120 * time.Millisecond)

// MusifyProvider scrapes https://musify.club/en/search
// Tracks: .tracklist__row.playlist__item with data-artist/data-name + [data-url]=/track/pl/….mp3
type MusifyProvider struct{ *BaseProvider }

func NewMusifyProvider(client *http.Client) *MusifyProvider {
	return &MusifyProvider{BaseProvider: NewBaseProvider("Musify", 6, client)}
}

func (p *MusifyProvider) UseRotator(r proxyRotator) *MusifyProvider {
	p.WithRotator(r)
	return p
}

func (p *MusifyProvider) SearchWithPage(ctx context.Context, query string, page int) ([]domain.ProviderResult, error) {
	if page < 1 {
		page = 1
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if err := musifyPace.wait(ctx); err != nil {
		return nil, fmt.Errorf("%s: %w", p.Name(), err)
	}

	doc, err := p.fetchDocument(ctx, musifySearchURL(query, page), musifyOrigin+"/en/")
	if err != nil {
		return nil, err
	}
	return p.parseResults(doc, page), nil
}

func musifySearchURL(query string, page int) string {
	q := url.Values{
		"searchText": {query},
		"type":       {"song"},
	}
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	return musifyOrigin + "/en/search?" + q.Encode()
}

func (p *MusifyProvider) parseResults(doc *goquery.Document, page int) []domain.ProviderResult {
	pagination := p.pagination(doc, page)
	results := make([]domain.ProviderResult, 0, 40)
	rank := 1

	doc.Find(".tracklist__row.playlist__item").Each(func(_ int, s *goquery.Selection) {
		artist := strings.TrimSpace(s.AttrOr("data-artist", ""))
		title := strings.TrimSpace(s.AttrOr("data-name", ""))
		if title == "" {
			title = text(s.Find(".tracklist__title a").First())
		}
		if artist == "" {
			artist = text(s.Find(".tracklist__artist a").First())
		}

		play := s.Find("[data-url]").First()
		link := musifyPlayURL(play.AttrOr("data-url", ""))
		if link == "" {
			link = musifyPlayURL(s.Find("a.dl-btn").AttrOr("href", ""))
		}
		image := playableLink(play.AttrOr("data-art", ""))
		if image == "" {
			image = playableLink(s.Find("img.tracklist__cover").AttrOr("src", ""))
		}

		if title == "" || artist == "" || link == "" {
			return
		}
		song := domain.NewSong(title, artist, image, link)
		results = append(results, domain.NewProviderResult(*song, p.Name(), rank, pagination))
		rank++
	})
	return results
}

func musifyPlayURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "/track/") {
		return playableLink(musifyOrigin + raw)
	}
	return playableLink(raw)
}

func (p *MusifyProvider) pagination(doc *goquery.Document, page int) *domain.PaginationInfo {
	info := &domain.PaginationInfo{
		CurrentPage: page,
		HasPrevPage: page > 1,
		TotalPages:  page,
	}
	maxPage := page
	doc.Find("a.pagination-item[href]").Each(func(_ int, s *goquery.Selection) {
		href := s.AttrOr("href", "")
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		n, err := strconv.Atoi(u.Query().Get("page"))
		if err == nil && n > maxPage {
			maxPage = n
		}
	})
	info.TotalPages = maxPage
	info.HasNextPage = doc.Find("a.pagination-next[href]").FilterFunction(func(_ int, s *goquery.Selection) bool {
		href := strings.TrimSpace(s.AttrOr("href", ""))
		return href != "" && href != "#"
	}).Length() > 0 || page < maxPage
	info.HasPrevPage = page > 1
	return info
}
