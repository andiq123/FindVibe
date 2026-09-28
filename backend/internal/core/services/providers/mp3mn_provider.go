package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
)

const mp3mnOrigin = "https://mp3mn.net"

var mp3mnPace = newPacer(100 * time.Millisecond)

type Mp3mnProvider struct{ *BaseProvider }

func NewMp3mnProvider(client *http.Client) *Mp3mnProvider {
	return &Mp3mnProvider{BaseProvider: NewBaseProvider("Mp3mn", 7, client)}
}

func (p *Mp3mnProvider) SearchWithPage(ctx context.Context, query string, page int) ([]domain.ProviderResult, error) {
	// ponytail: site has no stable page>1
	if page > 1 {
		return nil, nil
	}
	if err := mp3mnPace.wait(ctx); err != nil {
		return nil, fmt.Errorf("%s: %w", p.Name(), err)
	}

	apiURL := mp3mnOrigin + "/?" + url.Values{"song": {query}}.Encode()
	doc, err := p.fetchDocument(ctx, apiURL, mp3mnOrigin+"/")
	if err != nil {
		return nil, err
	}
	return p.parseResults(doc), nil
}

func (p *Mp3mnProvider) parseResults(doc *goquery.Document) []domain.ProviderResult {
	results := make([]domain.ProviderResult, 0, 24)
	rank := 1

	doc.Find("ul.playlist li").Each(func(_ int, s *goquery.Selection) {
		title := text(s.Find(".playlist-name-title em").First())
		if title == "" {
			title = text(s.Find(".playlist-name-title").First())
		}
		artist := text(s.Find(".playlist-name-artist").First())
		link := strings.TrimSpace(s.Find("a.playlist-play").AttrOr("data-url", ""))
		if title == "" || artist == "" || link == "" {
			return
		}
		results = append(results, domain.NewProviderResult(*domain.NewSong(title, artist, "", link), p.Name(), rank, nil))
		rank++
	})
	return results
}
