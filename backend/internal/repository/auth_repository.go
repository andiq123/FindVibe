package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"gorm.io/gorm"
)

type AuthRepository struct {
	DB *gorm.DB
}

func NewAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{
		DB: db,
	}
}

func (ar *AuthRepository) AuthenticateUser(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User

	err := ar.DB.WithContext(ctx).Where("name = ?", username).Take(&user).Error
	if err == nil {
		return &user, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("auth repository: database error: %w", err)
	}

	newUser := domain.NewUser(username)
	if err := ar.DB.WithContext(ctx).Create(newUser).Error; err != nil {
		return nil, fmt.Errorf("auth repository: failed to create user: %w", err)
	}

	return newUser, nil
}

func (ar *AuthRepository) GetUserById(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User

	err := ar.DB.WithContext(ctx).Take(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth repository: database error: %w", err)
	}

	return &user, nil
}
