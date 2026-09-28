package ports

import (
	"context"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
)

type IMusicProvider interface {
	Name() string
	SearchWithPage(ctx context.Context, query string, page int) ([]domain.ProviderResult, error)
	Priority() int
}
