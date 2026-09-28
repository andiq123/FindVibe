package services

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type catalogTransport func(*http.Request) (*http.Response, error)

func (f catalogTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCatalogSearchOverlapsArtistAndTrackRequests(t *testing.T) {
	artistStarted := make(chan struct{})
	client := &http.Client{Transport: catalogTransport(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Query().Get("method") {
		case "track.search":
			select {
			case <-artistStarted:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
			body = `{"results":{"trackmatches":{"track":[]}}}`
		case "artist.search":
			close(artistStarted)
			body = `{"results":{"artistmatches":{"artist":[]}}}`
		default:
			t.Errorf("unexpected method: %s", r.URL.Query().Get("method"))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := NewLastFMCatalog(client, "test-key").Search(ctx, "test", 1, 20)
	if err != nil {
		t.Fatalf("independent catalog calls did not overlap: %v", err)
	}
}
