package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestCatalogParsing(t *testing.T) {
	for _, p := range []*CatalogProvider{NewMusicBossProvider(nil), NewHitmosProvider(nil)} {
		markup := `<div class="content-block"><ul class="playlist-list"><li class="playlist-item"><a class="playlist-item-play" data-url="http://new.kachevo.org/get/music/a.mp3"></a><div class="playlist-item-title">Cariceps</div><div class="playlist-item-subtitle">Dani Mocanu, Alex Velea</div><a class="playlist-item-img" data-img="/static/no-cover.jpg"></a></li></ul></div>`
		if p.Name() == "Hitmos" {
			markup = `<ul class="tracks__list"><li data-musmeta='{"artist":"Dani Mocanu, Alex Velea","title":"Cariceps","url":"/get/music/a.mp3","img":"/no-cover.jpg"}'></li><li data-musmeta='broken'></li></ul>`
		}
		markup += `<div class="pagination"><a href="/search/start/96?q=cariceps">3</a></div>`
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(markup))
		got := p.parseResults(doc, 1)
		if len(got) != 1 || got[0].Song.Title != "Cariceps" || got[0].Song.Link != p.origin+"/get/music/a.mp3" || got[0].Song.Image != "" || got[0].Song.Provider != p.Name() {
			t.Fatalf("%s: %+v", p.Name(), got)
		}
		if got[0].Pagination.TotalPages != 3 || !got[0].Pagination.HasNextPage {
			t.Fatal("pagination missing")
		}
		for _, raw := range []string{"https://evil.test/get/music/a.mp3", "//new.kachevo.org.evil.test/get/music/a.mp3", "/song/1", "javascript:alert(1)", "https://user@new.kachevo.org/get/music/a.mp3"} {
			if p.catalogURL(raw) != "" {
				t.Errorf("accepted %s", raw)
			}
		}
	}
}

func TestCatalogSearchPaginationAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/start/48" || r.URL.Query().Get("q") != "A & B" {
			t.Errorf("wrong search: %s", r.URL)
		}
		w.Write([]byte(`<div class="content-block"></div>`))
	}))
	defer server.Close()
	p := NewMusicBossProvider(server.Client())
	p.origin = server.URL
	if _, err := p.SearchWithPage(context.Background(), "A & B", 2); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.SearchWithPage(ctx, "A", 1); err == nil {
		t.Fatal("ignored cancellation")
	}
}
