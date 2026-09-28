package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
	"github.com/andiq123/FindVibeFiber/internal/core/ports"
	"github.com/andiq123/FindVibeFiber/internal/utils"
)

type FavoritesService struct {
	favoritesRepository ports.IFavoritesRepository
	authRepository      ports.IAuthRepository
}

func NewFavoritesService(favoritesRepository ports.IFavoritesRepository, authRepository ports.IAuthRepository) *FavoritesService {
	return &FavoritesService{
		favoritesRepository: favoritesRepository,
		authRepository:      authRepository,
	}
}

func (fs *FavoritesService) AddFavorite(ctx context.Context, userId string, song domain.FavoriteSong) error {
	user, err := fs.authRepository.GetUserById(ctx, userId)
	if err != nil {
		return fmt.Errorf("add favorite: %w", err)
	}
	song.UserID = user.ID
	song.Link = utils.UpgradeHTTPS(song.Link)
	song.Image = utils.UpgradeHTTPS(song.Image)
	if song.Link == "" || !strings.HasPrefix(song.Link, "https://") {
		return fmt.Errorf("add favorite: %w", domain.ErrInvalidInput)
	}
	if err := fs.favoritesRepository.AddFavorite(ctx, userId, song); err != nil {
		return fmt.Errorf("add favorite: %w", err)
	}
	return nil
}

func (fs *FavoritesService) DeleteFavorite(ctx context.Context, songId string) error {
	if err := fs.favoritesRepository.DeleteFavorite(ctx, songId); err != nil {
		return fmt.Errorf("delete favorite: %w", err)
	}
	return nil
}

func (fs *FavoritesService) GetFavorites(ctx context.Context, userId string) ([]domain.FavoriteSong, error) {
	songs, err := fs.favoritesRepository.GetFavorites(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("get favorites: %w", err)
	}
	return songs, nil
}

func (fs *FavoritesService) ReorderFavorites(ctx context.Context, songReorders []domain.ReorderRequest) error {
	cleaned := make([]domain.ReorderRequest, 0, len(songReorders))
	for _, r := range songReorders {
		if strings.TrimSpace(r.SongId) == "" {
			return fmt.Errorf("reorder favorites: %w", domain.ErrInvalidInput)
		}
		cleaned = append(cleaned, r)
	}
	if err := fs.favoritesRepository.ReorderFavorites(ctx, cleaned); err != nil {
		return fmt.Errorf("reorder favorites: %w", err)
	}
	return nil
}

func (fs *FavoritesService) UpdateFavoriteImage(ctx context.Context, songId, image string) error {
	image = strings.TrimSpace(image)
	// FavoriteSong.Image is varchar(1000); https-only keeps junk out of the vault.
	if image == "" || len(image) > 1000 || !strings.HasPrefix(image, "https://") {
		return domain.ErrInvalidInput
	}
	if err := fs.favoritesRepository.UpdateFavoriteImage(ctx, songId, image); err != nil {
		return fmt.Errorf("update favorite image: %w", err)
	}
	return nil
}

const maxFavoriteLyrics = 64_000

func (fs *FavoritesService) UpdateFavoriteLyrics(ctx context.Context, songId, lyrics string) error {
	lyrics = strings.TrimSpace(lyrics)
	if lyrics == "" || len(lyrics) > maxFavoriteLyrics {
		return domain.ErrInvalidInput
	}
	if err := fs.favoritesRepository.UpdateFavoriteLyrics(ctx, songId, lyrics); err != nil {
		return fmt.Errorf("update favorite lyrics: %w", err)
	}
	return nil
}

func (fs *FavoritesService) UpdateFavoriteLink(ctx context.Context, songId, link string) error {
	link = strings.TrimSpace(link)
	// FavoriteSong.Link is varchar(1000); https-only matches stream playback.
	if link == "" || len(link) > 1000 || !strings.HasPrefix(link, "https://") {
		return domain.ErrInvalidInput
	}
	if err := fs.favoritesRepository.UpdateFavoriteLink(ctx, songId, link); err != nil {
		return fmt.Errorf("update favorite link: %w", err)
	}
	return nil
}
