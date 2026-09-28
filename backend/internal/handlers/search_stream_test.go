package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/gofiber/fiber/v3"
)

type slowProgressSearch struct{}

func (slowProgressSearch) Search(ctx context.Context, query string, page int) (*domain.SearchResponse, error) {
	return domain.NewSearchResponse(nil, nil), nil
}

func (slowProgressSearch) SearchWithProgress(
	ctx context.Context,
	query string,
	page int,
	onMeta func(domain.SearchProgress) error,
	onSong func(domain.Song) error,
) (*domain.SearchResponse, error) {
	if onMeta != nil {
		if err := onMeta(domain.SearchProgress{
			Artists: []domain.SearchArtist{{Name: "Adele"}},
		}); err != nil {
			return nil, err
		}
	}
	songs := []domain.Song{
		{Id: "1", Title: "One", Artist: "Adele", Link: "https://x/1.mp3"},
		{Id: "2", Title: "Two", Artist: "Adele", Link: "https://x/2.mp3"},
		{Id: "3", Title: "Three", Artist: "Adele", Link: "https://x/3.mp3"},
	}
	for i := range songs {
		time.Sleep(80 * time.Millisecond)
		if onSong != nil {
			if err := onSong(songs[i]); err != nil {
				return domain.NewSearchResponse(songs[:i+1], nil), err
			}
		}
	}
	return domain.NewSearchResponse(songs, nil), nil
}

func (slowProgressSearch) SearchFirst(context.Context, string, int) ([]domain.Song, error) {
	return nil, nil
}

func TestSearchStreamFlushesSongsProgressively(t *testing.T) {
	app := fiber.New()
	h := NewSearchHandler(slowProgressSearch{}, nil)
	app.Get("/search", h.Search)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() { _ = app.Listener(ln) }()

	url := fmt.Sprintf("http://%s/search?q=adele&stream=1", ln.Addr().String())
	start := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "ndjson") {
		t.Fatalf("content-type %q", resp.Header.Get("Content-Type"))
	}

	type line struct {
		at   time.Duration
		kind string
	}
	var seen []line
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		var ev searchStreamEvent
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			t.Fatalf("decode %q: %v", raw, err)
		}
		seen = append(seen, line{at: time.Since(start), kind: ev.Type})
		t.Logf("%.0fms %s", seen[len(seen)-1].at.Seconds()*1000, ev.Type)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}

	if len(seen) < 5 {
		t.Fatalf("want meta+3 songs+done, got %#v", seen)
	}
	if seen[0].kind != "meta" {
		t.Fatalf("first=%s", seen[0].kind)
	}
	var songAts []time.Duration
	for _, l := range seen {
		if l.kind == "song" {
			songAts = append(songAts, l.at)
		}
	}
	if len(songAts) < 3 {
		t.Fatalf("want 3 song lines, got %#v", seen)
	}
	spread := songAts[2] - songAts[0]
	if spread < 100*time.Millisecond {
		t.Fatalf("songs arrived in a burst (%v) — stream not progressive: %#v", spread, seen)
	}
}
