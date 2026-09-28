package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/core/ports"
)

type AuthService struct {
	authRepository ports.IAuthRepository
}

func NewAuthService(repository ports.IAuthRepository) *AuthService {
	return &AuthService{
		authRepository: repository,
	}
}

func (as *AuthService) AuthenticateUser(ctx context.Context, username string) (*domain.User, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	user, err := as.authRepository.AuthenticateUser(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("authenticate user: %w", err)
	}
	return user, nil
}
