package services

import (
	"context"
	"errors"
	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/core/ports"
	"testing"
	"time"
)

func TestSearchFirstReturnsHighestPriorityProvider(t *testing.T) {
	high := stubProvider{
		name:     "Mp3pm",
		priority: 8,
		results: []domain.ProviderResult{
			{Song: domain.Song{Title: "Hello", Artist: "Adele", Link: "https://pm.mp3"}, Provider: "Mp3pm", ProviderRank: 1},
		},
	}
	low := stubProvider{
		name:     "Mp3mn",
		priority: 7,
		delay:    50 * time.Millisecond,
		results: []domain.ProviderResult{
			{Song: domain.Song{Title: "Hello", Artist: "Adele", Link: "https://mn.mp3"}, Provider: "Mp3mn", ProviderRank: 1},
		},
	}
	svc := NewSearchService([]ports.IMusicProvider{low, high}, domain.DefaultSearchConfig(), time.Second)
	got, err := svc.SearchFirst(context.Background(), "adele hello", 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Link != "https://pm.mp3" {
		t.Fatalf("want Mp3pm first-wins, got %+v", got)
	}
}

func TestSearchFirstWaitsForHigherPriority(t *testing.T) {
	high := stubProvider{
		name:     "Mp3pm",
		priority: 8,
		delay:    40 * time.Millisecond,
		results: []domain.ProviderResult{
			{Song: domain.Song{Title: "Hello", Artist: "Adele", Link: "https://pm.mp3"}, Provider: "Mp3pm", ProviderRank: 1},
		},
	}
	low := stubProvider{
		name:     "Mp3mn",
		priority: 7,
		results: []domain.ProviderResult{
			{Song: domain.Song{Title: "Hello", Artist: "Adele", Link: "https://mn.mp3"}, Provider: "Mp3mn", ProviderRank: 1},
		},
	}
	svc := NewSearchService([]ports.IMusicProvider{low, high}, domain.DefaultSearchConfig(), time.Second)
	start := time.Now()
	got, err := svc.SearchFirst(context.Background(), "adele hello", 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Link != "https://pm.mp3" {
		t.Fatalf("want higher-priority link, got %+v", got)
	}
	if time.Since(start) < 35*time.Millisecond {
		t.Fatalf("returned before higher-priority provider could finish")
	}
}

type stubProvider struct {
	name     string
	priority int
	delay    time.Duration
	results  []domain.ProviderResult
	err      error
}

func (s stubProvider) Name() string  { return s.name }
func (s stubProvider) Priority() int { return s.priority }
func (s stubProvider) SearchWithPage(ctx context.Context, query string, page int) ([]domain.ProviderResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.results, nil
}

var _ ports.IMusicProvider = stubProvider{}

func TestDirectSearchStreamsBeforeSlowProviderAndKeepsAlternates(t *testing.T) {
	fast := stubProvider{name: "Musify", results: []domain.ProviderResult{{Song: domain.Song{Artist: "A", Title: "T", Link: "https://musify.club/a"}, Pagination: &domain.PaginationInfo{HasNextPage: true}}}}
	slow := stubProvider{name: "Mp3pm", delay: 100 * time.Millisecond, results: []domain.ProviderResult{{Song: domain.Song{Artist: "A", Title: "T", Link: "https://mp3.pm/a"}}}}
	svc := NewSearchService([]ports.IMusicProvider{slow, fast}, nil, time.Second)
	start := time.Now()
	var first time.Duration
	count := 0
	resp, err := svc.SearchWithProgress(context.Background(), "A T", 1, nil, func(s domain.Song) error {
		if count == 0 {
			first = time.Since(start)
		}
		count++
		return nil
	})
	if err != nil || count != 2 || len(resp.Songs) != 1 || len(resp.Songs[0].Alternatives) != 1 || !resp.Pagination.HasNextPage {
		t.Fatalf("bad response: %+v %v", resp, err)
	}
	if first >= 90*time.Millisecond {
		t.Fatalf("stream waited for slow provider: %v", first)
	}
	resp.Songs[0].Title = "mutated"
	cached, err := svc.Search(context.Background(), "a  t", 1)
	if err != nil || cached.Songs[0].Title == "mutated" {
		t.Fatal("cache mutated")
	}
}
func TestDirectSearchFailureAndCancellation(t *testing.T) {
	svc := NewSearchService([]ports.IMusicProvider{stubProvider{err: errors.New("offline")}}, nil, time.Second)
	if _, err := svc.Search(context.Background(), "test", 1); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("missing failure: %v", err)
	}
	svc = NewSearchService([]ports.IMusicProvider{stubProvider{delay: time.Second}}, nil, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Search(ctx, "test", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("missing cancellation: %v", err)
	}
}
func TestDirectSearchStopsOnConsumerError(t *testing.T) {
	svc := NewSearchService([]ports.IMusicProvider{stubProvider{results: []domain.ProviderResult{{Song: domain.Song{Link: "https://mp3.pm/a"}}}}}, nil, time.Second)
	stop := errors.New("disconnected")
	_, err := svc.SearchWithProgress(context.Background(), "test", 1, nil, func(domain.Song) error { return stop })
	if !errors.Is(err, stop) {
		t.Fatalf("lost callback error: %v", err)
	}
}
