package ports

import (
	"context"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
)

type IAuthService interface {
	AuthenticateUser(ctx context.Context, username string) (*domain.User, error)
}

type IAuthRepository interface {
	AuthenticateUser(ctx context.Context, username string) (*domain.User, error)
	GetUserById(ctx context.Context, id string) (*domain.User, error)
}
