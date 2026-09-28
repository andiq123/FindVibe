package providers

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func TestMusifyParseResults(t *testing.T) {
	html := `
<div class="tracklist__row playlist__item"
     itemprop="track"
     data-artist="Skrillex"
     data-name="Bangarang."
     data-song-id="3273451">
  <div class="tracklist__cover-wrap"
       data-player-row-play
       data-url="/track/pl/3273451/skrillex-bangarang.mp3"
       data-art="https://38s.musify.club/img/70/6765815/19533443.jpg">
  </div>
  <div class="tracklist__meta">
    <div class="tracklist__title"><a itemprop="url" href="/en/track/x">Bangarang.</a></div>
    <div class="tracklist__artist"><a href="/en/artist/skrillex">Skrillex</a></div>
  </div>
</div>
<div class="tracklist__row playlist__item" data-artist="Skip" data-name="">
  <div data-url="/track/pl/1/bad.mp3"></div>
</div>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	got := NewMusifyProvider(nil).parseResults(doc, 1)
	if len(got) != 1 {
		t.Fatalf("want 1 track, got %d", len(got))
	}
	s := got[0].Song
	if s.Artist != "Skrillex" || s.Title != "Bangarang." {
		t.Fatalf("got %q — %q", s.Artist, s.Title)
	}
	if s.Link != "https://musify.club/track/pl/3273451/skrillex-bangarang.mp3" {
		t.Fatalf("link: %q", s.Link)
	}
	if !strings.Contains(s.Image, "musify.club") {
		t.Fatalf("image: %q", s.Image)
	}
}

func TestMusifySearchURL(t *testing.T) {
	u := musifySearchURL("skrillex", 2)
	if !strings.Contains(u, "searchText=skrillex") || !strings.Contains(u, "type=song") {
		t.Fatalf("base query missing: %s", u)
	}
	if !strings.Contains(u, "page=2") {
		t.Fatalf("page missing: %s", u)
	}
	if strings.Contains(u, "yearFrom") || strings.Contains(u, "yearTo") {
		t.Fatalf("year filters should not be present: %s", u)
	}
}

func TestMusifyLiveSearch(t *testing.T) {
	if os.Getenv("LIVE") != "1" {
		t.Skip("set LIVE=1 to hit musify.club")
	}
	res, err := NewMusifyProvider(&http.Client{Timeout: 20 * time.Second}).
		SearchWithPage(context.Background(), "skrillex", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("expected tracks")
	}
	if !strings.Contains(res[0].Song.Link, "musify.club/track/") {
		t.Fatalf("link: %q", res[0].Song.Link)
	}
	t.Logf("first: %s — %s (%s)", res[0].Song.Artist, res[0].Song.Title, res[0].Song.Link)
}
