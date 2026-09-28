package services

import (
	"errors"
	"testing"

	"github.com/andiq123/FindVibeFiber/internal/core/domain"
)

func TestUpdateFavoriteImageRejectsBadURL(t *testing.T) {
	fs := &FavoritesService{}
	for _, img := range []string{"", "http://x", "ftp://x", string(make([]byte, 1001))} {
		if err := fs.UpdateFavoriteImage(t.Context(), "id", img); err != domain.ErrInvalidInput {
			t.Fatalf("image %q: got %v want ErrInvalidInput", truncate(img), err)
		}
	}
}

func TestUpdateFavoriteLyricsRejectsEmptyOrHuge(t *testing.T) {
	fs := &FavoritesService{}
	for _, ly := range []string{"", "   ", string(make([]byte, maxFavoriteLyrics+1))} {
		if err := fs.UpdateFavoriteLyrics(t.Context(), "id", ly); err != domain.ErrInvalidInput {
			t.Fatalf("lyrics %q: got %v want ErrInvalidInput", truncate(ly), err)
		}
	}
}

func TestUpdateFavoriteLinkRejectsBadURL(t *testing.T) {
	fs := &FavoritesService{}
	for _, link := range []string{"", "http://x/a.mp3", "ftp://x", string(make([]byte, 1001))} {
		if err := fs.UpdateFavoriteLink(t.Context(), "id", link); err != domain.ErrInvalidInput {
			t.Fatalf("link %q: got %v want ErrInvalidInput", truncate(link), err)
		}
	}
}

func TestReorderFavoritesRejectsEmptySongID(t *testing.T) {
	fs := &FavoritesService{}
	err := fs.ReorderFavorites(t.Context(), []domain.ReorderRequest{{SongId: "", Order: 0}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func truncate(s string) string {
	if len(s) > 32 {
		return s[:32] + "…"
	}
	return s
}
