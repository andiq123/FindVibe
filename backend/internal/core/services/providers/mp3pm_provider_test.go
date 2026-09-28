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

func TestAbsoluteURL(t *testing.T) {
	if got := absoluteURL("//cs1.mp3.pm/listen/1/x.mp3"); got != "https://cs1.mp3.pm/listen/1/x.mp3" {
		t.Fatalf("protocol-relative: %q", got)
	}
	if got := absoluteURL("http://cs1.mp3.pm/a.mp3"); got != "https://cs1.mp3.pm/a.mp3" {
		t.Fatalf("http upgrade: %q", got)
	}
}

func TestPlayableLink(t *testing.T) {
	if got := playableLink("https://cs1.mp3.pm/listen/1/a.mp3"); got != "https://cs1.mp3.pm/listen/1/a.mp3" {
		t.Fatalf("got %q", got)
	}
	if playableLink("") != "" || playableLink("javascript:void(0)") != "" {
		t.Fatal("expected empty for bad hrefs")
	}
}

func TestMp3pmFallbackSlug(t *testing.T) {
	p := NewMp3pmProvider(nil)
	got := p.fallbackResultsURL("Adele Hello!")
	if got != "https://s-adele-hello.mp3.pm/" {
		t.Fatalf("got %q", got)
	}
}

func TestMp3pmParseResults(t *testing.T) {
	html := `
<ul class="mp3list">
<li class="cplayer-sound-item" data-sound-id="1"
	data-sound-url="https://cs1.mp3.pm/listen/1/token/Adele_-_Hello_(mp3.pm).mp3"
	data-download-url="https://cs1.mp3.pm/download/1/token/Adele_-_Hello_(mp3.pm).mp3">
	<h4>
		<a><i class="cplayer-data-sound-author">Adele</i></a>
		<a><b class="cplayer-data-sound-title">Hello</b></a>
	</h4>
</li>
<li class="cplayer-sound-item" data-sound-url="">
	<h4>
		<a><i class="cplayer-data-sound-author">Skip</i></a>
		<a><b class="cplayer-data-sound-title">Me</b></a>
	</h4>
</li>
</ul>
<div class="listalka"><span class="listalka-page"><b>1</b><i>3</i></span><a href="/page/2/" class="listalka-next">next</a></div>
`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	got := NewMp3pmProvider(nil).parseResults(doc, 1)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got[0].Song.Artist != "Adele" || got[0].Song.Title != "Hello" {
		t.Fatalf("meta: %#v", got[0].Song)
	}
	if !strings.Contains(got[0].Song.Link, "cs1.mp3.pm/listen/") {
		t.Fatalf("link: %q", got[0].Song.Link)
	}
	if got[0].Pagination == nil || got[0].Pagination.TotalPages != 3 || !got[0].Pagination.HasNextPage {
		t.Fatalf("pagination: %#v", got[0].Pagination)
	}
}

func TestMp3pmLiveSearch(t *testing.T) {
	if os.Getenv("LIVE") != "1" {
		t.Skip("set LIVE=1 to hit mp3.pm")
	}
	res, err := NewMp3pmProvider(&http.Client{Timeout: 15 * time.Second}).
		SearchWithPage(context.Background(), "adele", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("no results")
	}
	if !strings.Contains(res[0].Song.Link, "mp3.pm") {
		t.Fatalf("unexpected link host: %q", res[0].Song.Link)
	}
}
