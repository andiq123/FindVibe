package ports

import (
	"context"
)

type ISuggestionsService interface {
	GetSuggestions(ctx context.Context, query, hl, gl string) ([]string, error)
}
