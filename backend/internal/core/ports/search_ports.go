package ports

import (
	"context"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
)

type ISearchService interface {
	// Search: Last.fm discovers songs/artists; providers map playable URLs; only mapped hits return.
	Search(ctx context.Context, query string, page int) (*domain.SearchResponse, error)
	// SearchWithProgress is Search; onMeta fires after catalog discovery, onSong as each hit maps.
	SearchWithProgress(
		ctx context.Context,
		query string,
		page int,
		onMeta func(domain.SearchProgress) error,
		onSong func(domain.Song) error,
	) (*domain.SearchResponse, error)
	// SearchFirst fans out providers and returns the highest-priority playable hit
	// as soon as no higher-priority provider is still in flight (explore/radio resolve).
	SearchFirst(ctx context.Context, query string, limit int) ([]domain.Song, error)
}
