package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
)

const (
	mp3pmOrigin = "https://mp3.pm"
	mp3pmSearch = mp3pmOrigin + "/public/api.search.php"
)

var (
	mp3pmPace      = newPacer(120 * time.Millisecond)
	mp3pmSlugClean = regexp.MustCompile(`[^a-z0-9]+`)
)

// Mp3pmProvider scrapes https://mp3.pm/
// Flow: POST /public/api.search.php → https://s-<slug>.mp3.pm[/page/N/]
// Tracks: li.cplayer-sound-item[data-sound-url].
type Mp3pmProvider struct{ *BaseProvider }

func NewMp3pmProvider(client *http.Client) *Mp3pmProvider {
	return &Mp3pmProvider{BaseProvider: NewBaseProvider("Mp3pm", 8, client)}
}

func (p *Mp3pmProvider) UseRotator(r proxyRotator) *Mp3pmProvider {
	p.WithRotator(r)
	return p
}

func (p *Mp3pmProvider) SearchWithPage(ctx context.Context, query string, page int) ([]domain.ProviderResult, error) {
	if page < 1 {
		page = 1
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if err := mp3pmPace.wait(ctx); err != nil {
		return nil, fmt.Errorf("%s: %w", p.Name(), err)
	}

	listURL, err := p.resolveResultsURL(ctx, query)
	if err != nil {
		return nil, err
	}
	if page > 1 {
		listURL = strings.TrimRight(listURL, "/") + fmt.Sprintf("/page/%d/", page)
	}

	doc, err := p.fetchDocument(ctx, listURL, mp3pmOrigin+"/")
	if err != nil {
		// Missing slug pages are normal misses (search API often 404s → fallback host 404).
		if isNotFoundStatus(err) {
			return nil, nil
		}
		return nil, err
	}
	return p.parseResults(doc, page), nil
}

func (p *Mp3pmProvider) resolveResultsURL(ctx context.Context, query string) (string, error) {
	form := url.Values{"q": {query}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mp3pmSearch, strings.NewReader(form))
	if err != nil {
		return "", fmt.Errorf("%s: request: %w", p.Name(), err)
	}
	setBrowserHeaders(req, mp3pmOrigin+"/")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", mp3pmOrigin)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := p.client.Do(req)
	if err != nil {
		return p.fallbackResultsURL(query), nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if err != nil || resp.StatusCode != http.StatusOK {
		return p.fallbackResultsURL(query), nil
	}

	loc := strings.TrimSpace(string(body))
	loc = strings.Trim(loc, "\"'")
	if !strings.HasPrefix(loc, "https://s-") || !strings.Contains(loc, ".mp3.pm") {
		return p.fallbackResultsURL(query), nil
	}
	if !strings.HasSuffix(loc, "/") {
		loc += "/"
	}
	return loc, nil
}

func (p *Mp3pmProvider) fallbackResultsURL(query string) string {
	slug := strings.ToLower(strings.TrimSpace(query))
	slug = mp3pmSlugClean.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "music"
	}
	return fmt.Sprintf("https://s-%s.mp3.pm/", slug)
}

func (p *Mp3pmProvider) parseResults(doc *goquery.Document, page int) []domain.ProviderResult {
	pagination := p.pagination(doc, page)
	results := make([]domain.ProviderResult, 0, 50)
	rank := 1

	doc.Find("li.cplayer-sound-item").Each(func(_ int, s *goquery.Selection) {
		link := playableLink(s.AttrOr("data-sound-url", ""))
		if link == "" {
			link = playableLink(s.AttrOr("data-download-url", ""))
		}
		artist := text(s.Find(".cplayer-data-sound-author").First())
		title := text(s.Find(".cplayer-data-sound-title").First())
		if title == "" || artist == "" || link == "" {
			return
		}
		song := domain.NewSong(title, artist, "", link)
		results = append(results, domain.NewProviderResult(*song, p.Name(), rank, pagination))
		rank++
	})
	return results
}

func (p *Mp3pmProvider) pagination(doc *goquery.Document, page int) *domain.PaginationInfo {
	info := &domain.PaginationInfo{
		CurrentPage: page,
		HasPrevPage: page > 1,
		TotalPages:  page,
	}
	// <span class="listalka-page"><b>1</b><i>5</i></span>
	if n, err := strconv.Atoi(text(doc.Find(".listalka-page b").First())); err == nil && n > 0 {
		info.CurrentPage = n
	}
	if n, err := strconv.Atoi(text(doc.Find(".listalka-page i").First())); err == nil && n > 0 {
		info.TotalPages = n
	}
	if info.TotalPages < info.CurrentPage {
		info.TotalPages = info.CurrentPage
	}
	info.HasNextPage = info.CurrentPage < info.TotalPages || doc.Find("a.listalka-next").Length() > 0
	info.HasPrevPage = info.CurrentPage > 1
	return info
}
