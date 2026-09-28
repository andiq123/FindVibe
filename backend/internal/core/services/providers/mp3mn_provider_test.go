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

func TestMp3mnParseResults(t *testing.T) {
	html := `
<ul class="playlist" data-urlnext="false">
  <li class="first">
    <a href="javascript:void(0);" class="playlist-play" data-url="https://mn1.sunproxy.net/file/abc/Drake_-_Gods_Plan.mp3">Play</a>
    <span class="playlist-name-artist"><a href="/a/1-drake/">Drake</a></span>
    <span class="playlist-name-title"><a href="/t/1/"><em>God&#039;s Plan</em></a></span>
  </li>
  <li>
    <a href="javascript:void(0);" class="playlist-play" data-url="https://mn1.sunproxy.net/file/abc/Skip.mp3">Play</a>
    <span class="playlist-name-artist"><a>Drake</a></span>
    <span class="playlist-name-title"><a><em></em></a></span>
  </li>
</ul>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	got := NewMp3mnProvider(nil).parseResults(doc)
	if len(got) != 1 {
		t.Fatalf("want 1 track, got %d", len(got))
	}
	if got[0].Song.Artist != "Drake" || got[0].Song.Title != "God's Plan" {
		t.Fatalf("got %q — %q", got[0].Song.Artist, got[0].Song.Title)
	}
	if !strings.HasPrefix(got[0].Song.Link, "https://mn1.sunproxy.net/") {
		t.Fatalf("link: %q", got[0].Song.Link)
	}
}

func TestMp3mnLiveSearch(t *testing.T) {
	if os.Getenv("LIVE") != "1" {
		t.Skip("set LIVE=1 to hit mp3mn.net")
	}
	res, err := NewMp3mnProvider(&http.Client{Timeout: 15 * time.Second}).
		SearchWithPage(context.Background(), "drake", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("no results")
	}
}
